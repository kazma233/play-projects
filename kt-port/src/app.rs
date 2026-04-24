use std::cell::RefCell;
use std::collections::HashSet;
use std::rc::Rc;

use chrono::{DateTime, Local};
use slint::{Model, ModelRc, SharedString, VecModel};
use sysinfo::{Pid, ProcessesToUpdate, System};

use crate::ports::{self, KillTarget, PortInfo};

slint::include_modules!();

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum SortField {
    Port,
    Protocol,
    Status,
    Process,
    Pid,
    User,
}

impl SortField {
    fn from_key(key: &str) -> Self {
        match key {
            "protocol" => Self::Protocol,
            "status" => Self::Status,
            "process" => Self::Process,
            "pid" => Self::Pid,
            "user" => Self::User,
            _ => Self::Port,
        }
    }

    fn as_key(self) -> &'static str {
        match self {
            Self::Port => "port",
            Self::Protocol => "protocol",
            Self::Status => "status",
            Self::Process => "process",
            Self::Pid => "pid",
            Self::User => "user",
        }
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum SortDirection {
    Asc,
    Desc,
}

impl SortDirection {
    fn toggle(self) -> Self {
        match self {
            Self::Asc => Self::Desc,
            Self::Desc => Self::Asc,
        }
    }

    fn from_key(key: &str) -> Self {
        if key.eq_ignore_ascii_case("desc") {
            Self::Desc
        } else {
            Self::Asc
        }
    }

    fn as_key(self) -> &'static str {
        match self {
            Self::Asc => "asc",
            Self::Desc => "desc",
        }
    }
}

#[derive(Clone, Debug, Default)]
struct DetailInfo {
    process_path: String,
    command_line: String,
    current_dir: String,
    parent_pid: String,
    start_time: String,
    run_time: String,
    memory: String,
}

pub fn run() -> Result<(), slint::PlatformError> {
    let ui = AppWindow::new()?;
    let all_rows = Rc::new(RefCell::new(Vec::<PortInfo>::new()));
    let filtered_rows = Rc::new(RefCell::new(Vec::<PortInfo>::new()));
    let selected_keys = Rc::new(RefCell::new(HashSet::<String>::new()));
    let pending_kill_targets = Rc::new(RefCell::new(Vec::<KillTarget>::new()));
    let current_detail_key = Rc::new(RefCell::new(String::new()));
    let rows_model = Rc::new(VecModel::from(Vec::<PortRow>::new()));

    ui.set_rows(ModelRc::from(rows_model.clone()));
    ui.set_status_options(ModelRc::from(Rc::new(VecModel::from(vec![
        "全部状态".into(),
        "监听中".into(),
        "已连接".into(),
        "等待中".into(),
        "关闭中".into(),
        "关闭阶段1".into(),
        "关闭阶段2".into(),
        "最后确认".into(),
        "已关闭".into(),
        "新连接中".into(),
        "发起中".into(),
        "连接中".into(),
        "UDP".into(),
        "未知".into(),
    ]))));
    ui.set_status_filter_index(0);
    ui.set_protocol_options(ModelRc::from(Rc::new(VecModel::from(vec![
        "全部协议".into(),
        "TCP".into(),
        "UDP".into(),
    ]))));
    ui.set_protocol_filter_index(0);
    ui.set_sort_field("port".into());
    ui.set_sort_direction("asc".into());
    reset_detail_panel(&ui);

    let sync_rows = {
        let ui = ui.as_weak();
        let filtered_rows = Rc::clone(&filtered_rows);
        let selected_keys = Rc::clone(&selected_keys);
        let current_detail_key = Rc::clone(&current_detail_key);
        let rows_model = rows_model.clone();

        move |status: String| {
            if let Some(ui) = ui.upgrade() {
                let selected_keys = selected_keys.borrow();
                let detail_key = current_detail_key.borrow().clone();
                let rows = filtered_rows
                    .borrow()
                    .iter()
                    .map(|row| {
                        let key = selection_key(row);
                        PortRow::from_state(
                            row,
                            selected_keys.contains(&key),
                            detail_key == key,
                        )
                    })
                    .collect::<Vec<_>>();
                rows_model.set_vec(rows);
                ui.set_has_selection(!selected_keys.is_empty());
                ui.set_status_text(status.into());
            }
        }
    };

    let sync_filtered_rows = {
        let ui_weak = ui.as_weak();
        let all_rows = Rc::clone(&all_rows);
        let filtered_rows = Rc::clone(&filtered_rows);
        let selected_keys = Rc::clone(&selected_keys);
        let current_detail_key = Rc::clone(&current_detail_key);
        let sync_rows = sync_rows.clone();

        move |status: String| {
            let Some(ui) = ui_weak.upgrade() else {
                return;
            };

            let filter_text = ui.get_filter_text().to_string();
            let status_filter = current_status_filter(&ui);
            let protocol_filter = current_protocol_filter(&ui);
            let sort_field = SortField::from_key(&ui.get_sort_field());
            let sort_direction = SortDirection::from_key(&ui.get_sort_direction());

            *filtered_rows.borrow_mut() = build_visible_rows(
                &all_rows.borrow(),
                &filter_text,
                &status_filter,
                &protocol_filter,
                sort_field,
                sort_direction,
            );

            retain_visible_selection(&filtered_rows.borrow(), &mut selected_keys.borrow_mut());
            sync_current_detail(
                &ui,
                &filtered_rows.borrow(),
                &mut current_detail_key.borrow_mut(),
            );
            sync_rows(status);
        }
    };

    let refresh = {
        let all_rows = Rc::clone(&all_rows);
        let selected_keys = Rc::clone(&selected_keys);
        let sync_filtered_rows = sync_filtered_rows.clone();

        move || match ports::get_port_list() {
            Ok(entries) => {
                selected_keys.borrow_mut().clear();
                *all_rows.borrow_mut() = entries;
                sync_filtered_rows(format!("Loaded {} rows", all_rows.borrow().len()));
            }
            Err(error) => {
                sync_filtered_rows(format!("Refresh failed: {error}"));
            }
        }
    };

    {
        let refresh = refresh.clone();
        ui.on_refresh(move || refresh());
    }

    {
        let sync_filtered_rows = sync_filtered_rows.clone();
        ui.on_filter_changed(move |_| {
            sync_filtered_rows("Filter updated".to_string());
        });
    }

    {
        let sync_filtered_rows = sync_filtered_rows.clone();
        ui.on_status_filter_changed(move |_, _| {
            sync_filtered_rows("Status filter updated".to_string());
        });
    }

    {
        let sync_filtered_rows = sync_filtered_rows.clone();
        ui.on_protocol_filter_changed(move |_, _| {
            sync_filtered_rows("Protocol filter updated".to_string());
        });
    }

    {
        let ui_weak = ui.as_weak();
        let sync_filtered_rows = sync_filtered_rows.clone();
        ui.on_sort_changed(move |column_key| {
            let Some(ui) = ui_weak.upgrade() else {
                return;
            };

            let next_field = SortField::from_key(&column_key);
            let current_field = SortField::from_key(&ui.get_sort_field());
            let current_direction = SortDirection::from_key(&ui.get_sort_direction());
            let next_direction = if next_field == current_field {
                current_direction.toggle()
            } else {
                SortDirection::Asc
            };

            ui.set_sort_field(next_field.as_key().into());
            ui.set_sort_direction(next_direction.as_key().into());
            sync_filtered_rows(format!("Sorted by {} {}", next_field.as_key(), next_direction.as_key()));
        });
    }

    {
        let filtered_rows = Rc::clone(&filtered_rows);
        let selected_keys = Rc::clone(&selected_keys);
        let sync_rows = sync_rows.clone();
        ui.on_toggle_row_selection(move |index| {
            let Some(entry) = filtered_rows.borrow().get(index as usize).cloned() else {
                return;
            };

            let key = selection_key(&entry);
            let count = {
                let mut selected = selected_keys.borrow_mut();
                if !selected.insert(key) {
                    selected.remove(&selection_key(&entry));
                }
                selected.len()
            };

            sync_rows(if count == 0 {
                "No rows selected".to_string()
            } else {
                format!("Selected {} rows", count)
            });
        });
    }

    {
        let filtered_rows = Rc::clone(&filtered_rows);
        let current_detail_key = Rc::clone(&current_detail_key);
        let sync_rows = sync_rows.clone();
        let ui_weak = ui.as_weak();
        ui.on_show_row_details(move |index| {
            let Some(ui) = ui_weak.upgrade() else {
                return;
            };

            let Some(entry) = filtered_rows.borrow().get(index as usize).cloned() else {
                return;
            };

            *current_detail_key.borrow_mut() = selection_key(&entry);
            set_detail_panel(&ui, &entry);
            sync_rows(format!("Viewing {} {}", entry.protocol, entry.local_addr));
        });
    }

    {
        let filtered_rows = Rc::clone(&filtered_rows);
        let selected_keys = Rc::clone(&selected_keys);
        let pending_kill_targets = Rc::clone(&pending_kill_targets);
        let ui_weak = ui.as_weak();
        ui.on_kill_selected(move || {
            let selected = selected_keys.borrow().clone();
            if selected.is_empty() {
                if let Some(ui) = ui_weak.upgrade() {
                    ui.set_status_text("No row selected".into());
                }
                return;
            }

            let ports_snapshot = filtered_rows.borrow().clone();
            let targets = ports_snapshot
                .iter()
                .filter(|entry| selected.contains(&selection_key(entry)) && entry.pid != 0)
                .map(|entry| KillTarget {
                    port: entry.port,
                    protocol: entry.protocol.clone(),
                    pid: entry.pid,
                    local_addr: entry.local_addr.clone(),
                    remote_addr: entry.remote_addr.clone(),
                })
                .collect::<Vec<_>>();

            if targets.is_empty() {
                if let Some(ui) = ui_weak.upgrade() {
                    ui.set_status_text("Selected rows have no PID".into());
                }
                return;
            }

            let confirm_message = targets
                .iter()
                .map(|target| {
                    format!(
                        "端口 {} | PID {} | {} | {}",
                        target.port, target.pid, target.protocol, target.local_addr
                    )
                })
                .collect::<Vec<_>>()
                .join("\n");

            *pending_kill_targets.borrow_mut() = targets;
            if let Some(ui) = ui_weak.upgrade() {
                ui.set_confirm_message(confirm_message.into());
                ui.set_confirm_visible(true);
            }
        });
    }

    {
        let ui_weak = ui.as_weak();
        ui.on_cancel_kill(move || {
            if let Some(ui) = ui_weak.upgrade() {
                ui.set_confirm_visible(false);
            }
        });
    }

    {
        let filtered_rows = Rc::clone(&filtered_rows);
        let selected_keys = Rc::clone(&selected_keys);
        let pending_kill_targets = Rc::clone(&pending_kill_targets);
        let ui_weak = ui.as_weak();
        let refresh = refresh.clone();
        ui.on_confirm_kill(move || {
            let ports_snapshot = filtered_rows.borrow().clone();
            let targets = pending_kill_targets.borrow().clone();

            if let Some(ui) = ui_weak.upgrade() {
                ui.set_confirm_visible(false);
                ui.set_status_text(format!("Killing {} targets...", targets.len()).into());
            }

            match slint::spawn_local(async move {
                let mut messages = Vec::new();
                for target in &targets {
                    match ports::kill_process(target, &ports_snapshot).await {
                        Ok(message) => messages.push(message),
                        Err(error) => {
                            messages.push(format!("Kill failed for pid {}: {error}", target.pid))
                        }
                    }
                }
                messages
            })
            .ok()
            {
                Some(handle) => {
                    let ui_weak = ui_weak.clone();
                    let refresh = refresh.clone();
                    let selected_keys = selected_keys.clone();
                    let pending_kill_targets = pending_kill_targets.clone();
                    slint::spawn_local(async move {
                        let messages = handle.await;
                        selected_keys.borrow_mut().clear();
                        pending_kill_targets.borrow_mut().clear();
                        if let Some(ui) = ui_weak.upgrade() {
                            ui.set_has_selection(false);
                            ui.set_status_text(messages.join(" | ").into());
                        }
                        refresh();
                    })
                    .expect("failed to join kill task");
                }
                None => {
                    if let Some(ui) = ui_weak.upgrade() {
                        ui.set_status_text("Failed to schedule kill task".into());
                    }
                }
            }
        });
    }

    refresh();
    ui.run()
}

fn build_visible_rows(
    entries: &[PortInfo],
    filter: &str,
    status_filter: &str,
    protocol_filter: &str,
    sort_field: SortField,
    sort_direction: SortDirection,
) -> Vec<PortInfo> {
    let mut result = filter_ports(entries, filter, status_filter, protocol_filter);
    sort_ports(&mut result, sort_field, sort_direction);
    result
}

fn filter_ports(
    entries: &[PortInfo],
    filter: &str,
    status_filter: &str,
    protocol_filter: &str,
) -> Vec<PortInfo> {
    let query = filter.trim().to_ascii_lowercase();
    let normalized_status_filter = status_filter.trim();
    let normalized_protocol_filter = protocol_filter.trim();

    entries
        .iter()
        .filter(|entry| matches_status_filter(entry, normalized_status_filter))
        .filter(|entry| matches_protocol_filter(entry, normalized_protocol_filter))
        .filter(|entry| {
            if query.is_empty() {
                return true;
            }

            [
                entry.protocol.as_str(),
                entry.status.as_str(),
                entry.local_addr.as_str(),
                entry.remote_addr.as_str(),
                entry.process_name.as_str(),
                entry.user.as_str(),
            ]
            .iter()
            .any(|value| value.to_ascii_lowercase().contains(&query))
                || entry.port.to_string().contains(&query)
                || entry.pid.to_string().contains(&query)
        })
        .cloned()
        .collect()
}

fn sort_ports(entries: &mut [PortInfo], field: SortField, direction: SortDirection) {
    entries.sort_by(|left, right| {
        let ordering = match field {
            SortField::Port => left.port.cmp(&right.port),
            SortField::Protocol => left.protocol.cmp(&right.protocol),
            SortField::Status => left.status.cmp(&right.status),
            SortField::Process => left.process_name.cmp(&right.process_name),
            SortField::Pid => left.pid.cmp(&right.pid),
            SortField::User => left.user.cmp(&right.user),
        }
        .then_with(|| left.port.cmp(&right.port))
        .then_with(|| left.protocol.cmp(&right.protocol))
        .then_with(|| left.local_addr.cmp(&right.local_addr))
        .then_with(|| left.remote_addr.cmp(&right.remote_addr))
        .then_with(|| left.pid.cmp(&right.pid));

        match direction {
            SortDirection::Asc => ordering,
            SortDirection::Desc => ordering.reverse(),
        }
    });
}

fn matches_status_filter(entry: &PortInfo, status_filter: &str) -> bool {
    if status_filter.is_empty() || status_filter == "ALL" {
        return true;
    }

    entry.status.eq_ignore_ascii_case(status_filter)
        || (status_filter == "UDP" && entry.protocol.eq_ignore_ascii_case("UDP"))
}

fn matches_protocol_filter(entry: &PortInfo, protocol_filter: &str) -> bool {
    protocol_filter.is_empty()
        || protocol_filter == "ALL"
        || entry.protocol.eq_ignore_ascii_case(protocol_filter)
}

fn status_filter_value(index: i32, label: &str) -> String {
    if index <= 0 {
        return "ALL".to_string();
    }

    match label {
        "监听中" => "LISTEN",
        "已连接" => "ESTABLISHED",
        "等待中" => "TIME_WAIT",
        "关闭中" => "CLOSE_WAIT",
        "关闭阶段1" => "FIN_WAIT1",
        "关闭阶段2" => "FIN_WAIT2",
        "最后确认" => "LAST_ACK",
        "已关闭" => "CLOSE",
        "新连接中" => "NEW_SYN_RECV",
        "发起中" => "SYN_SENT",
        "连接中" => "SYN_RECV",
        "UDP" => "UDP",
        "未知" => "UNKNOWN",
        _ => "ALL",
    }
    .to_string()
}

fn protocol_filter_value(index: i32, label: &str) -> String {
    if index <= 0 {
        return "ALL".to_string();
    }

    match label {
        "TCP" => "TCP",
        "UDP" => "UDP",
        _ => "ALL",
    }
    .to_string()
}

fn current_status_filter(ui: &AppWindow) -> String {
    let index = ui.get_status_filter_index();
    let options = ui.get_status_options();
    let label = options.row_data(index as usize).unwrap_or_default();
    status_filter_value(index, &label)
}

fn current_protocol_filter(ui: &AppWindow) -> String {
    let index = ui.get_protocol_filter_index();
    let options = ui.get_protocol_options();
    let label = options.row_data(index as usize).unwrap_or_default();
    protocol_filter_value(index, &label)
}

fn retain_visible_selection(entries: &[PortInfo], selected_keys: &mut HashSet<String>) {
    let visible = entries.iter().map(selection_key).collect::<HashSet<_>>();
    selected_keys.retain(|key| visible.contains(key));
}

fn sync_current_detail(ui: &AppWindow, entries: &[PortInfo], current_detail_key: &mut String) {
    if current_detail_key.is_empty() {
        if let Some(entry) = entries.first() {
            *current_detail_key = selection_key(entry);
            set_detail_panel(ui, entry);
        } else {
            reset_detail_panel(ui);
        }
        return;
    }

    if let Some(entry) = entries.iter().find(|entry| selection_key(entry) == *current_detail_key) {
        set_detail_panel(ui, entry);
    } else if let Some(entry) = entries.first() {
        *current_detail_key = selection_key(entry);
        set_detail_panel(ui, entry);
    } else {
        current_detail_key.clear();
        reset_detail_panel(ui);
    }
}

fn selection_key(entry: &PortInfo) -> String {
    format!(
        "{}|{}|{}|{}|{}",
        entry.port, entry.protocol, entry.pid, entry.local_addr, entry.remote_addr
    )
}

fn set_detail_panel(ui: &AppWindow, entry: &PortInfo) {
    ui.set_detail_title(format!("{} {}", entry.protocol, entry.local_addr).into());
    ui.set_detail_port(entry.port.to_string().into());
    ui.set_detail_protocol(entry.protocol.clone().into());
    ui.set_detail_status(display_value(&entry.status).into());
    ui.set_detail_pid(display_number(entry.pid).into());
    ui.set_detail_process_name(display_value(&entry.process_name).into());
    ui.set_detail_user(display_value(&entry.user).into());
    ui.set_detail_local_addr(display_value(&entry.local_addr).into());
    ui.set_detail_remote_addr(display_value(&entry.remote_addr).into());

    let detail = fetch_detail_info(entry.pid);
    ui.set_detail_process_path(display_value(&detail.process_path).into());
    ui.set_detail_command_line(display_value(&detail.command_line).into());
    ui.set_detail_current_dir(display_value(&detail.current_dir).into());
    ui.set_detail_parent_pid(display_value(&detail.parent_pid).into());
    ui.set_detail_start_time(display_value(&detail.start_time).into());
    ui.set_detail_run_time(display_value(&detail.run_time).into());
    ui.set_detail_memory(display_value(&detail.memory).into());
}

fn reset_detail_panel(ui: &AppWindow) {
    ui.set_detail_title(SharedString::from("未选中端口记录"));
    ui.set_detail_port(SharedString::from("-"));
    ui.set_detail_protocol(SharedString::from("-"));
    ui.set_detail_status(SharedString::from("-"));
    ui.set_detail_pid(SharedString::from("-"));
    ui.set_detail_process_name(SharedString::from("-"));
    ui.set_detail_user(SharedString::from("-"));
    ui.set_detail_local_addr(SharedString::from("-"));
    ui.set_detail_remote_addr(SharedString::from("-"));
    ui.set_detail_process_path(SharedString::from("-"));
    ui.set_detail_command_line(SharedString::from("-"));
    ui.set_detail_current_dir(SharedString::from("-"));
    ui.set_detail_parent_pid(SharedString::from("-"));
    ui.set_detail_start_time(SharedString::from("-"));
    ui.set_detail_run_time(SharedString::from("-"));
    ui.set_detail_memory(SharedString::from("-"));
}

fn fetch_detail_info(pid: u32) -> DetailInfo {
    if pid == 0 {
        return DetailInfo::default();
    }

    let mut system = System::new();
    system.refresh_processes(ProcessesToUpdate::Some(&[Pid::from_u32(pid)]), true);
    let Some(process) = system.process(Pid::from_u32(pid)) else {
        return DetailInfo::default();
    };

    DetailInfo {
        process_path: process
            .exe()
            .map(|path| path.display().to_string())
            .unwrap_or_default(),
        command_line: process
            .cmd()
            .iter()
            .map(|arg| arg.to_string_lossy().to_string())
            .collect::<Vec<_>>()
            .join(" "),
        current_dir: process
            .cwd()
            .map(|path| path.display().to_string())
            .unwrap_or_default(),
        parent_pid: process
            .parent()
            .map(|parent| parent.as_u32().to_string())
            .unwrap_or_default(),
        start_time: format_timestamp(process.start_time()),
        run_time: format_duration(process.run_time()),
        memory: format_bytes(process.memory()),
    }
}

fn display_value(value: &str) -> String {
    if value.trim().is_empty() {
        "-".to_string()
    } else {
        value.to_string()
    }
}

fn display_number(value: u32) -> String {
    if value == 0 {
        "-".to_string()
    } else {
        value.to_string()
    }
}

fn format_duration(seconds: u64) -> String {
    if seconds == 0 {
        return String::new();
    }

    let hours = seconds / 3600;
    let minutes = (seconds % 3600) / 60;
    let secs = seconds % 60;

    if hours > 0 {
        format!("{}h {}m {}s", hours, minutes, secs)
    } else if minutes > 0 {
        format!("{}m {}s", minutes, secs)
    } else {
        format!("{}s", secs)
    }
}

fn format_bytes(bytes: u64) -> String {
    if bytes == 0 {
        return String::new();
    }

    const KB: u64 = 1024;
    const MB: u64 = 1024 * KB;
    const GB: u64 = 1024 * MB;

    if bytes >= GB {
        format!("{:.2} GB", bytes as f64 / GB as f64)
    } else if bytes >= MB {
        format!("{:.2} MB", bytes as f64 / MB as f64)
    } else if bytes >= KB {
        format!("{:.2} KB", bytes as f64 / KB as f64)
    } else {
        format!("{} B", bytes)
    }
}

fn format_timestamp(seconds_from_epoch: u64) -> String {
    if seconds_from_epoch == 0 {
        return String::new();
    }

    let Some(date_time) = DateTime::from_timestamp(seconds_from_epoch as i64, 0) else {
        return seconds_from_epoch.to_string();
    };

    date_time
        .with_timezone(&Local)
        .format("%Y-%m-%d %H:%M:%S")
        .to_string()
}

impl From<&PortInfo> for PortRow {
    fn from(value: &PortInfo) -> Self {
        Self::from_state(value, false, false)
    }
}

impl PortRow {
    fn from_state(value: &PortInfo, selected: bool, is_current: bool) -> Self {
        Self {
            port: value.port.to_string().into(),
            protocol: value.protocol.clone().into(),
            status: value.status.clone().into(),
            local_addr: value.local_addr.clone().into(),
            remote_addr: value.remote_addr.clone().into(),
            process_name: value.process_name.clone().into(),
            pid: value.pid.to_string().into(),
            user: value.user.clone().into(),
            selected,
            is_current,
        }
    }
}
