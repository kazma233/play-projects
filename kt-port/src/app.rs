use std::collections::HashSet;
use std::fmt;

use chrono::{DateTime, Local};
use iced::alignment::{Horizontal, Vertical};
use iced::widget::{
    Column, Space, button, checkbox, column, container, mouse_area, pick_list, row, scrollable,
    stack, text, text_input,
};
use iced::widget::text::Wrapping;
use iced::{Alignment, Background, Border, Color, Element, Length, Padding, Task, Theme};
use sysinfo::{Pid, ProcessesToUpdate, System};

use crate::ports::{self, KillTarget, PortInfo};

const WINDOW_TITLE: &str = "kt-port";
const STATUS_OPTIONS: [StatusFilter; 14] = [
    StatusFilter::All,
    StatusFilter::Listen,
    StatusFilter::Established,
    StatusFilter::TimeWait,
    StatusFilter::CloseWait,
    StatusFilter::FinWait1,
    StatusFilter::FinWait2,
    StatusFilter::LastAck,
    StatusFilter::Close,
    StatusFilter::NewSynRecv,
    StatusFilter::SynSent,
    StatusFilter::SynRecv,
    StatusFilter::Udp,
    StatusFilter::Unknown,
];
const PROTOCOL_OPTIONS: [ProtocolFilter; 3] = [
    ProtocolFilter::All,
    ProtocolFilter::Tcp,
    ProtocolFilter::Udp,
];

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum SortField {
    Port,
    Protocol,
    Status,
    Process,
    Pid,
    User,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
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
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum StatusFilter {
    All,
    Listen,
    Established,
    TimeWait,
    CloseWait,
    FinWait1,
    FinWait2,
    LastAck,
    Close,
    NewSynRecv,
    SynSent,
    SynRecv,
    Udp,
    Unknown,
}

impl StatusFilter {
    fn label(self) -> &'static str {
        match self {
            Self::All => "全部状态",
            Self::Listen => "监听中",
            Self::Established => "已连接",
            Self::TimeWait => "等待中",
            Self::CloseWait => "关闭中",
            Self::FinWait1 => "关闭阶段1",
            Self::FinWait2 => "关闭阶段2",
            Self::LastAck => "最后确认",
            Self::Close => "已关闭",
            Self::NewSynRecv => "新连接中",
            Self::SynSent => "发起中",
            Self::SynRecv => "连接中",
            Self::Udp => "UDP",
            Self::Unknown => "未知",
        }
    }

    fn value(self) -> &'static str {
        match self {
            Self::All => "ALL",
            Self::Listen => "LISTEN",
            Self::Established => "ESTABLISHED",
            Self::TimeWait => "TIME_WAIT",
            Self::CloseWait => "CLOSE_WAIT",
            Self::FinWait1 => "FIN_WAIT1",
            Self::FinWait2 => "FIN_WAIT2",
            Self::LastAck => "LAST_ACK",
            Self::Close => "CLOSE",
            Self::NewSynRecv => "NEW_SYN_RECV",
            Self::SynSent => "SYN_SENT",
            Self::SynRecv => "SYN_RECV",
            Self::Udp => "UDP",
            Self::Unknown => "UNKNOWN",
        }
    }
}

impl fmt::Display for StatusFilter {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.label())
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ProtocolFilter {
    All,
    Tcp,
    Udp,
}

impl ProtocolFilter {
    fn label(self) -> &'static str {
        match self {
            Self::All => "全部协议",
            Self::Tcp => "TCP",
            Self::Udp => "UDP",
        }
    }

    fn value(self) -> &'static str {
        match self {
            Self::All => "ALL",
            Self::Tcp => "TCP",
            Self::Udp => "UDP",
        }
    }
}

impl fmt::Display for ProtocolFilter {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.label())
    }
}

#[derive(Debug, Clone, Default)]
struct DetailInfo {
    process_path: String,
    command_line: String,
    current_dir: String,
    parent_pid: String,
    start_time: String,
    run_time: String,
    memory: String,
}

#[derive(Debug, Clone)]
struct PortApp {
    all_rows: Vec<PortInfo>,
    filtered_rows: Vec<PortInfo>,
    selected_keys: HashSet<String>,
    pending_kill_targets: Vec<KillTarget>,
    current_detail_key: Option<String>,
    detail_info: DetailInfo,
    filter_text: String,
    status_filter: StatusFilter,
    protocol_filter: ProtocolFilter,
    only_pid_filter: bool,
    sort_field: SortField,
    sort_direction: SortDirection,
    status_text: String,
    active_refresh_id: u64,
    confirm_visible: bool,
}

#[derive(Debug, Clone)]
enum Message {
    RefreshRequested,
    RefreshFinished {
        request_id: u64,
        result: Result<Vec<PortInfo>, ports::PortError>,
    },
    FilterChanged(String),
    StatusFilterChanged(StatusFilter),
    ProtocolFilterChanged(ProtocolFilter),
    OnlyPidFilterChanged(bool),
    SortBy(SortField),
    ToggleRowSelection {
        key: String,
        selected: bool,
    },
    ShowRowDetails(String),
    KillSelected,
    CancelKill,
    ConfirmKill,
    KillFinished(Vec<String>),
}

pub fn run() -> iced::Result {
    iced::application(PortApp::boot, PortApp::update, PortApp::view)
        .title(app_title)
        .theme(app_theme)
        .window_size(iced::Size::new(1420.0, 900.0))
        .centered()
        .run()
}

impl Default for PortApp {
    fn default() -> Self {
        Self {
            all_rows: Vec::new(),
            filtered_rows: Vec::new(),
            selected_keys: HashSet::new(),
            pending_kill_targets: Vec::new(),
            current_detail_key: None,
            detail_info: DetailInfo::default(),
            filter_text: String::new(),
            status_filter: StatusFilter::Listen,
            protocol_filter: ProtocolFilter::All,
            only_pid_filter: true,
            sort_field: SortField::Port,
            sort_direction: SortDirection::Asc,
            status_text: "Ready".to_string(),
            active_refresh_id: 0,
            confirm_visible: false,
        }
    }
}

impl PortApp {
    fn boot() -> (Self, Task<Message>) {
        let mut app = Self::default();
        let task = app.schedule_refresh();
        (app, task)
    }

    fn update(&mut self, message: Message) -> Task<Message> {
        match message {
            Message::RefreshRequested => self.schedule_refresh(),
            Message::RefreshFinished { request_id, result } => {
                if request_id != self.active_refresh_id {
                    return Task::none();
                }

                match result {
                    Ok(entries) => {
                        let row_count = entries.len();
                        self.selected_keys.clear();
                        self.all_rows = entries;
                        self.recompute_filtered_rows(format!("Loaded {row_count} rows"));
                    }
                    Err(error) => {
                        self.status_text = format!("Refresh failed: {error}");
                    }
                }

                Task::none()
            }
            Message::FilterChanged(value) => {
                self.filter_text = value;
                self.recompute_filtered_rows("Filter updated".to_string());
                Task::none()
            }
            Message::StatusFilterChanged(value) => {
                self.status_filter = value;
                self.recompute_filtered_rows("Status filter updated".to_string());
                Task::none()
            }
            Message::ProtocolFilterChanged(value) => {
                self.protocol_filter = value;
                self.recompute_filtered_rows("Protocol filter updated".to_string());
                Task::none()
            }
            Message::OnlyPidFilterChanged(value) => {
                self.only_pid_filter = value;
                self.recompute_filtered_rows("PID filter updated".to_string());
                Task::none()
            }
            Message::SortBy(field) => {
                if field == self.sort_field {
                    self.sort_direction = self.sort_direction.toggle();
                } else {
                    self.sort_field = field;
                    self.sort_direction = SortDirection::Asc;
                }

                self.recompute_filtered_rows(format!(
                    "Sorted by {} {}",
                    self.sort_field.key(),
                    self.sort_direction.key()
                ));
                Task::none()
            }
            Message::ToggleRowSelection { key, selected } => {
                if !self
                    .filtered_rows
                    .iter()
                    .any(|entry| selection_key(entry) == key && entry.pid != 0)
                {
                    return Task::none();
                }

                if selected {
                    self.selected_keys.insert(key);
                } else {
                    self.selected_keys.remove(&key);
                }

                let count = self.selected_keys.len();
                self.status_text = if count == 0 {
                    "No rows selected".to_string()
                } else {
                    format!("Selected {count} rows")
                };
                Task::none()
            }
            Message::ShowRowDetails(key) => {
                if let Some(entry) = self
                    .filtered_rows
                    .iter()
                    .find(|entry| selection_key(entry) == key)
                    .cloned()
                {
                    self.current_detail_key = Some(key);
                    self.detail_info = fetch_detail_info(entry.pid);
                    self.status_text = format!("Viewing {} {}", entry.protocol, entry.local_addr);
                }
                Task::none()
            }
            Message::KillSelected => {
                if self.selected_keys.is_empty() {
                    self.status_text = "No row selected".to_string();
                    return Task::none();
                }

                let targets = self
                    .filtered_rows
                    .iter()
                    .filter(|entry| {
                        self.selected_keys.contains(&selection_key(entry)) && entry.pid != 0
                    })
                    .map(|entry| KillTarget {
                        port: entry.port,
                        protocol: entry.protocol.clone(),
                        pid: entry.pid,
                        local_addr: entry.local_addr.clone(),
                        remote_addr: entry.remote_addr.clone(),
                    })
                    .collect::<Vec<_>>();

                if targets.is_empty() {
                    self.status_text = "Selected rows have no PID".to_string();
                    return Task::none();
                }

                self.pending_kill_targets = targets;
                self.confirm_visible = true;
                Task::none()
            }
            Message::CancelKill => {
                self.confirm_visible = false;
                Task::none()
            }
            Message::ConfirmKill => {
                let targets = self.pending_kill_targets.clone();
                let ports_snapshot = self.filtered_rows.clone();

                self.confirm_visible = false;
                self.status_text = format!("Killing {} targets...", targets.len());

                Task::perform(
                    async move { kill_targets(targets, ports_snapshot).await },
                    Message::KillFinished,
                )
            }
            Message::KillFinished(messages) => {
                self.selected_keys.clear();
                self.pending_kill_targets.clear();
                self.status_text = messages.join(" | ");
                self.schedule_refresh()
            }
        }
    }

    fn view(&self) -> Element<'_, Message> {
        let base = container(
            column![
                self.filters_view(),
                self.table_header_view(),
                self.table_view(),
                self.detail_panel_view(),
                text(&self.status_text).color(color_text_muted()),
            ]
            .spacing(10)
            .padding(12),
        )
        .width(Length::Fill)
        .height(Length::Fill);

        if self.confirm_visible {
            stack([base.into(), self.confirm_overlay_view()])
                .width(Length::Fill)
                .height(Length::Fill)
                .into()
        } else {
            base.into()
        }
    }

    fn schedule_refresh(&mut self) -> Task<Message> {
        self.active_refresh_id = self.active_refresh_id.wrapping_add(1);
        let request_id = self.active_refresh_id;
        self.status_text = "Loading ports...".to_string();

        Task::perform(
            async move {
                let result = ports::get_port_list();
                (request_id, result)
            },
            |(request_id, result)| Message::RefreshFinished { request_id, result },
        )
    }

    fn recompute_filtered_rows(&mut self, status: String) {
        self.filtered_rows = build_visible_rows(
            &self.all_rows,
            &self.filter_text,
            self.status_filter.value(),
            self.protocol_filter.value(),
            self.only_pid_filter,
            self.sort_field,
            self.sort_direction,
        );

        retain_visible_selection(&self.filtered_rows, &mut self.selected_keys);
        self.sync_current_detail();
        self.status_text = status;
    }

    fn sync_current_detail(&mut self) {
        if self.filtered_rows.is_empty() {
            self.current_detail_key = None;
            self.detail_info = DetailInfo::default();
            return;
        }

        let next_entry = if let Some(current_key) = self.current_detail_key.as_deref() {
            self.filtered_rows
                .iter()
                .find(|entry| selection_key(entry) == current_key)
                .cloned()
                .or_else(|| self.filtered_rows.first().cloned())
        } else {
            self.filtered_rows.first().cloned()
        };

        if let Some(entry) = next_entry {
            self.current_detail_key = Some(selection_key(&entry));
            self.detail_info = fetch_detail_info(entry.pid);
        } else {
            self.current_detail_key = None;
            self.detail_info = DetailInfo::default();
        }
    }

    fn current_detail_entry(&self) -> Option<&PortInfo> {
        let key = self.current_detail_key.as_deref()?;
        self.filtered_rows
            .iter()
            .find(|entry| selection_key(entry) == key)
    }

    fn filters_view(&self) -> Element<'_, Message> {
        row![
            pick_list(
                PROTOCOL_OPTIONS.as_slice(),
                Some(self.protocol_filter),
                Message::ProtocolFilterChanged
            )
            .width(160),
            pick_list(
                STATUS_OPTIONS.as_slice(),
                Some(self.status_filter),
                Message::StatusFilterChanged
            )
            .width(160),
            checkbox(self.only_pid_filter)
                .label("有PID")
                .on_toggle(Message::OnlyPidFilterChanged)
                .width(132),
            text_input(
                "Search port, pid, protocol, state, process, address or user",
                &self.filter_text
            )
            .on_input(Message::FilterChanged)
            .width(Length::Fill),
            button(text("Refresh"))
                .style(button::secondary)
                .on_press(Message::RefreshRequested),
            if self.selected_keys.is_empty() {
                button(text("Kill Selected")).style(button::secondary)
            } else {
                button(text("Kill Selected"))
                    .style(button::danger)
                    .on_press(Message::KillSelected)
            },
        ]
        .spacing(8)
        .align_y(Alignment::Center)
        .into()
    }

    fn table_header_view(&self) -> Element<'_, Message> {
        container(
            row![
                selection_header_cell(80.0),
                self.sort_header_cell("Port", 70.0, SortField::Port),
                self.sort_header_cell("Proto", 70.0, SortField::Protocol),
                self.sort_header_cell("Status", 120.0, SortField::Status),
                header_label_cell("Local", 250.0),
                header_label_cell("Remote", 250.0),
                self.sort_header_cell("Process", 190.0, SortField::Process),
                self.sort_header_cell("PID", 90.0, SortField::Pid),
                self.sort_header_cell("User", 110.0, SortField::User),
            ]
            .spacing(6)
            .padding([0, 8])
            .clip(true),
        )
        .height(34)
        .style(|_| container::Style {
            background: Some(Background::Color(color_header_bg())),
            ..Default::default()
        })
        .into()
    }

    fn sort_header_cell(
        &self,
        label: &'static str,
        width: f32,
        field: SortField,
    ) -> iced::widget::Container<'_, Message> {
        let is_active = self.sort_field == field;
        let indicator = if is_active {
            match self.sort_direction {
                SortDirection::Asc => " ^",
                SortDirection::Desc => " v",
            }
        } else {
            ""
        };

        container(
            button(
                text(format!("{label}{indicator}"))
                    .align_x(Horizontal::Center)
                    .size(14)
                    .color(if is_active {
                        color_header_active_text()
                    } else {
                        color_text_primary()
                    }),
            )
            .padding([6, 6])
            .width(Length::Fill)
            .style(button::text)
            .on_press(Message::SortBy(field)),
        )
        .width(width)
        .height(34)
        .style(move |_| container::Style {
            background: Some(Background::Color(if is_active {
                color_header_active_bg()
            } else {
                Color::TRANSPARENT
            })),
            ..Default::default()
        })
    }

    fn table_view(&self) -> Element<'_, Message> {
        let rows = self
            .filtered_rows
            .iter()
            .enumerate()
            .map(|(index, entry)| self.port_row_view(index, entry))
            .collect::<Vec<_>>();

        let list = scrollable(Column::with_children(rows).spacing(0)).height(Length::Fill);

        container(list).height(Length::Fill).into()
    }

    fn port_row_view(&self, index: usize, entry: &PortInfo) -> Element<'_, Message> {
        let key = selection_key(entry);
        let is_selected = self.selected_keys.contains(&key);
        let is_current = self.current_detail_key.as_deref() == Some(key.as_str());
        let selectable = entry.pid != 0;

        let checkbox_control = if selectable {
            checkbox(is_selected)
                .on_toggle({
                    let key = key.clone();
                    move |selected| Message::ToggleRowSelection {
                        key: key.clone(),
                        selected,
                    }
                })
                .size(18)
        } else {
            checkbox(is_selected).size(18)
        };

        let row_content = row![
            container(checkbox_control)
                .width(80)
                .height(36)
                .align_x(Horizontal::Center)
                .align_y(Vertical::Center),
            centered_cell(entry.port.to_string(), 70.0),
            centered_cell(entry.protocol.clone(), 70.0),
            centered_cell(entry.status.clone(), 120.0),
            text_cell(entry.local_addr.clone(), 250.0),
            text_cell(entry.remote_addr.clone(), 250.0),
            text_cell(entry.process_name.clone(), 190.0),
            centered_cell(entry.pid.to_string(), 90.0),
            text_cell(entry.user.clone(), 110.0),
        ]
        .spacing(6)
        .padding([0, 8])
        .clip(true)
        .align_y(Alignment::Center);

        let background = if is_current {
            color_row_current_bg()
        } else if index.is_multiple_of(2) {
            color_row_even_bg()
        } else {
            color_row_odd_bg()
        };

        let opacity = if selectable { 1.0 } else { 0.6 };
        let row_widget = mouse_area(container(row_content).width(Length::Fill).height(36).style(
            move |_| container::Style {
                background: Some(Background::Color(background.scale_alpha(opacity))),
                border: Border {
                    color: color_border_light(),
                    width: 1.0,
                    radius: 0.0.into(),
                },
                ..Default::default()
            },
        ))
        .interaction(iced::mouse::Interaction::Pointer)
        .on_press(Message::ShowRowDetails(key));

        row_widget.into()
    }

    fn detail_panel_view(&self) -> Element<'_, Message> {
        let entry = self.current_detail_entry();
        let title = entry
            .map(|entry| format!("{} {}", entry.protocol, entry.local_addr))
            .unwrap_or_else(|| "未选中端口记录".to_string());

        let body = column![
            text(title).size(18).color(color_text_primary()),
            row![
                detail_item("Port", entry.map(|entry| entry.port.to_string())),
                detail_item("Protocol", entry.map(|entry| entry.protocol.clone())),
                detail_item("Status", entry.map(|entry| entry.status.clone())),
                detail_item("PID", entry.map(|entry| display_number(entry.pid))),
                detail_item("User", entry.map(|entry| entry.user.clone())),
            ]
            .spacing(8),
            row![
                detail_item("Process", entry.map(|entry| entry.process_name.clone()))
                    .width(Length::FillPortion(2)),
                detail_item(
                    "Parent PID",
                    Some(display_value(&self.detail_info.parent_pid))
                ),
                detail_item("Run Time", Some(display_value(&self.detail_info.run_time))),
                detail_item("Memory", Some(display_value(&self.detail_info.memory))),
            ]
            .spacing(8),
            row![
                detail_item("Local Address", entry.map(|entry| entry.local_addr.clone()))
                    .width(Length::Fill),
                detail_item(
                    "Remote Address",
                    entry.map(|entry| entry.remote_addr.clone())
                )
                .width(Length::Fill),
            ]
            .spacing(8),
            row![
                detail_item(
                    "Executable Path",
                    Some(display_value(&self.detail_info.process_path))
                )
                .width(Length::Fill),
                detail_item(
                    "Current Directory",
                    Some(display_value(&self.detail_info.current_dir))
                )
                .width(Length::Fill),
            ]
            .spacing(8),
            row![
                detail_item(
                    "Command Line",
                    Some(display_value(&self.detail_info.command_line))
                )
                .width(Length::FillPortion(2)),
                detail_item(
                    "Start Time",
                    Some(display_value(&self.detail_info.start_time))
                ),
            ]
            .spacing(8),
        ]
        .spacing(8)
        .padding(12);

        container(body)
            .width(Length::Fill)
            .style(|_| panel_style())
            .into()
    }

    fn confirm_overlay_view(&self) -> Element<'_, Message> {
        let confirm_message = self
            .pending_kill_targets
            .iter()
            .map(|target| {
                format!(
                    "端口 {} | PID {} | {} | {}",
                    target.port, target.pid, target.protocol, target.local_addr
                )
            })
            .collect::<Vec<_>>()
            .join("\n");

        let dialog = container(
            column![
                container(
                    text("确认终止")
                        .size(16)
                        .align_x(Horizontal::Center)
                        .align_y(Vertical::Center)
                        .color(Color::WHITE)
                )
                .height(36)
                .width(Length::Fill)
                .align_x(Horizontal::Center)
                .align_y(Vertical::Center)
                .style(|_| container::Style {
                    background: Some(Background::Color(color_danger_header())),
                    ..Default::default()
                }),
                column![
                    container(
                        text(confirm_message)
                            .size(14)
                            .color(color_text_primary())
                            .width(Length::Fill)
                    )
                    .padding(8)
                    .width(Length::Fill)
                    .style(|_| card_style()),
                    row![
                        Space::new().width(Length::Fill),
                        button(text("取消"))
                            .style(button::secondary)
                            .on_press(Message::CancelKill)
                            .width(96),
                        button(text("确认 Kill"))
                            .style(button::danger)
                            .on_press(Message::ConfirmKill)
                            .width(120),
                    ]
                    .spacing(8)
                    .align_y(Alignment::Center),
                ]
                .spacing(12)
                .padding(16),
            ]
            .spacing(0),
        )
        .width(560)
        .style(|_| container::Style {
            background: Some(Background::Color(Color::WHITE)),
            border: Border {
                color: color_danger_border(),
                width: 1.0,
                radius: 0.0.into(),
            },
            ..Default::default()
        });

        container(dialog)
            .width(Length::Fill)
            .height(Length::Fill)
            .center_x(Length::Fill)
            .center_y(Length::Fill)
            .style(|_| container::Style {
                background: Some(Background::Color(Color::from_rgba8(0, 0, 0, 0.53))),
                ..Default::default()
            })
            .into()
    }
}

impl SortField {
    fn key(self) -> &'static str {
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

impl SortDirection {
    fn key(self) -> &'static str {
        match self {
            Self::Asc => "asc",
            Self::Desc => "desc",
        }
    }
}

async fn kill_targets(targets: Vec<KillTarget>, ports_snapshot: Vec<PortInfo>) -> Vec<String> {
    let mut messages = Vec::new();
    for target in &targets {
        match ports::kill_process(target, &ports_snapshot).await {
            Ok(message) => messages.push(message),
            Err(error) => messages.push(format!("Kill failed for pid {}: {error}", target.pid)),
        }
    }
    messages
}

fn app_title(_: &PortApp) -> String {
    WINDOW_TITLE.to_string()
}

fn app_theme(_: &PortApp) -> Theme {
    Theme::Light
}

fn build_visible_rows(
    entries: &[PortInfo],
    filter: &str,
    status_filter: &str,
    protocol_filter: &str,
    only_pid_filter: bool,
    sort_field: SortField,
    sort_direction: SortDirection,
) -> Vec<PortInfo> {
    let mut result = filter_ports(
        entries,
        filter,
        status_filter,
        protocol_filter,
        only_pid_filter,
    );
    sort_ports(&mut result, sort_field, sort_direction);
    result
}

fn filter_ports(
    entries: &[PortInfo],
    filter: &str,
    status_filter: &str,
    protocol_filter: &str,
    only_pid_filter: bool,
) -> Vec<PortInfo> {
    let query = filter.trim().to_ascii_lowercase();
    let normalized_status_filter = status_filter.trim();
    let normalized_protocol_filter = protocol_filter.trim();

    entries
        .iter()
        .filter(|entry| matches_status_filter(entry, normalized_status_filter))
        .filter(|entry| matches_protocol_filter(entry, normalized_protocol_filter))
        .filter(|entry| !only_pid_filter || entry.pid != 0)
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

fn retain_visible_selection(entries: &[PortInfo], selected_keys: &mut HashSet<String>) {
    let visible = entries.iter().map(selection_key).collect::<HashSet<_>>();
    selected_keys.retain(|key| visible.contains(key));
}

fn selection_key(entry: &PortInfo) -> String {
    format!(
        "{}|{}|{}|{}|{}",
        entry.port, entry.protocol, entry.pid, entry.local_addr, entry.remote_addr
    )
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

fn selection_header_cell<'a>(width: f32) -> iced::widget::Container<'a, Message> {
    header_label_cell("Sel", width)
}

fn header_label_cell<'a>(label: &'static str, width: f32) -> iced::widget::Container<'a, Message> {
    container(
        text(label)
            .size(14)
            .width(Length::Fill)
            .wrapping(Wrapping::None)
            .align_x(Horizontal::Center)
            .align_y(Vertical::Center)
            .color(color_text_primary()),
    )
    .width(width)
    .height(34)
    .clip(true)
    .align_x(Horizontal::Center)
    .align_y(Vertical::Center)
}

fn text_cell<'a>(value: String, width: f32) -> iced::widget::Container<'a, Message> {
    container(
        text(value)
            .size(14)
            .width(Length::Fill)
            .wrapping(Wrapping::WordOrGlyph)
            .color(color_text_primary()),
    )
    .width(width)
    .height(36)
    .clip(true)
    .align_x(Horizontal::Center)
    .align_y(Vertical::Center)
}

fn centered_cell<'a>(value: String, width: f32) -> iced::widget::Container<'a, Message> {
    container(
        text(value)
            .size(14)
            .width(Length::Fill)
            .wrapping(Wrapping::WordOrGlyph)
            .align_x(Horizontal::Center)
            .align_y(Vertical::Center)
            .color(color_text_primary()),
    )
    .width(width)
    .height(36)
    .clip(true)
    .align_x(Horizontal::Center)
    .align_y(Vertical::Center)
}

fn detail_item<'a>(
    label: &'static str,
    value: Option<String>,
) -> iced::widget::Container<'a, Message> {
    container(
        column![
            text(label).size(12).color(color_text_muted()),
            text(display_value(&value.unwrap_or_default()))
                .size(14)
                .width(Length::Fill)
                .wrapping(Wrapping::WordOrGlyph)
                .color(color_text_primary())
        ]
        .spacing(5),
    )
    .padding(Padding::default().top(10).right(8).bottom(16).left(8))
    .height(Length::Shrink)
    .width(Length::Fill)
    .style(|_| card_style())
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
        format!("{hours}h {minutes}m {secs}s")
    } else if minutes > 0 {
        format!("{minutes}m {secs}s")
    } else {
        format!("{secs}s")
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
        format!("{bytes} B")
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

fn panel_style() -> container::Style {
    container::Style {
        background: Some(Background::Color(Color::WHITE)),
        border: Border {
            color: color_border_panel(),
            width: 1.0,
            radius: 0.0.into(),
        },
        ..Default::default()
    }
}

fn card_style() -> container::Style {
    container::Style {
        background: Some(Background::Color(color_card_bg())),
        border: Border {
            color: color_border_light(),
            width: 1.0,
            radius: 0.0.into(),
        },
        ..Default::default()
    }
}

fn color_header_bg() -> Color {
    Color::from_rgb8(0xE2, 0xE8, 0xF0)
}

fn color_header_active_bg() -> Color {
    Color::from_rgb8(0xDB, 0xEA, 0xFE)
}

fn color_header_active_text() -> Color {
    Color::from_rgb8(0x1D, 0x4E, 0xD8)
}

fn color_row_even_bg() -> Color {
    Color::WHITE
}

fn color_row_odd_bg() -> Color {
    Color::from_rgb8(0xF8, 0xFA, 0xFC)
}

fn color_row_current_bg() -> Color {
    Color::from_rgb8(0xEF, 0xF6, 0xFF)
}

fn color_card_bg() -> Color {
    Color::from_rgb8(0xF8, 0xFA, 0xFC)
}

fn color_border_light() -> Color {
    Color::from_rgb8(0xE2, 0xE8, 0xF0)
}

fn color_border_panel() -> Color {
    Color::from_rgb8(0xCB, 0xD5, 0xE1)
}

fn color_text_primary() -> Color {
    Color::from_rgb8(0x0F, 0x17, 0x2A)
}

fn color_text_muted() -> Color {
    Color::from_rgb8(0x64, 0x74, 0x8B)
}

fn color_danger_header() -> Color {
    Color::from_rgb8(0x99, 0x1B, 0x1B)
}

fn color_danger_border() -> Color {
    Color::from_rgb8(0x7F, 0x1D, 0x1D)
}
