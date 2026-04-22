use std::collections::{HashMap, HashSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::{LazyLock, Mutex};

use anyhow::{anyhow, bail, Context, Result};
use dirs::home_dir;
use rusqlite::{params, Connection};
use serde_json::{json, Value};
use uuid::Uuid;

use crate::{
    ContentBlock, SessionAgent, SessionDetail, SessionDetailOverview, SessionEvent, SessionEventPage,
    SessionFileEntry, SessionMessage, SessionMessagePage, SessionSummary, SourceApp,
};

use super::{SessionExporter, SessionReader};

#[derive(Clone, Debug)]
struct OpenCodeSessionRow {
    id: String,
    parent_id: Option<String>,
    directory: String,
    title: String,
    time_created: i64,
    time_updated: i64,
}

#[derive(Clone, Debug)]
struct OpenCodeSessionFamily {
    root: OpenCodeSessionRow,
    members: Vec<OpenCodeSessionRow>,
}

#[derive(Default)]
struct OpenCodeImportStats {
    additions: usize,
    deletions: usize,
    files: usize,
}

#[derive(Clone, Default)]
struct OpenCodeTimelineCacheEntry {
    updated_at: i64,
    messages: Option<Vec<SessionMessage>>,
    events: Option<Vec<SessionEvent>>,
}

static OPEN_CODE_TIMELINE_CACHE: LazyLock<Mutex<HashMap<String, OpenCodeTimelineCacheEntry>>> =
    LazyLock::new(|| Mutex::new(HashMap::new()));

pub(crate) struct OpenCodeBackend;

pub(crate) static BACKEND: OpenCodeBackend = OpenCodeBackend;

impl SessionReader for OpenCodeBackend {
    fn list_entries(&self) -> Result<Vec<SessionFileEntry>> {
        let mut entries = list_session_families()?
            .into_iter()
            .map(|family| SessionFileEntry {
                path: session_path(&family.root.id),
                sort_timestamp: family.root.time_updated,
                summary: None,
            })
            .collect::<Vec<_>>();

        super::sort_entries(&mut entries);
        Ok(entries)
    }

    fn clear_cache(&self) -> Result<()> {
        OPEN_CODE_TIMELINE_CACHE
            .lock()
            .map_err(|_| anyhow!("OpenCode timeline cache lock was poisoned"))?
            .clear();
        Ok(())
    }

    fn resolve_path(&self, source_session_id: &str) -> Result<PathBuf> {
        Ok(session_path(source_session_id))
    }

    fn parse_summary(&self, path: &Path) -> Result<SessionSummary> {
        self::parse_summary(path)
    }

    fn parse_overview(&self, path: &Path) -> Result<SessionDetailOverview> {
        self::parse_overview(path)
    }

    fn parse_messages_page(
        &self,
        path: &Path,
        offset: usize,
        limit: usize,
    ) -> Result<SessionMessagePage> {
        self::parse_messages_page(path, offset, limit)
    }

    fn parse_events_page(
        &self,
        path: &Path,
        offset: usize,
        limit: usize,
    ) -> Result<SessionEventPage> {
        self::parse_events_page(path, offset, limit)
    }

    fn parse_detail(&self, path: &Path) -> Result<SessionDetail> {
        self::parse_detail(path)
    }
}

impl SessionExporter for OpenCodeBackend {
    fn planned_import_paths(
        &self,
        _summary: &SessionSummary,
        _session_id: &str,
    ) -> Result<Vec<String>> {
        Ok(vec![db_path()?.display().to_string()])
    }

    fn export_session(
        &self,
        detail: &SessionDetail,
        new_session_id: &str,
    ) -> Result<(String, Vec<String>)> {
        write_session(detail, new_session_id)
    }
}

pub(crate) fn root() -> Result<PathBuf> {
    Ok(home_dir()
        .context("Unable to determine home directory")?
        .join(".local")
        .join("share")
        .join("opencode"))
}

pub(crate) fn db_path() -> Result<PathBuf> {
    Ok(root()?.join("opencode.db"))
}

pub(crate) fn session_path(session_id: &str) -> PathBuf {
    root()
        .unwrap_or_else(|_| PathBuf::from("/tmp"))
        .join("session")
        .join(format!("{session_id}.opencode"))
}

pub(crate) fn count_sessions() -> Result<usize> {
    Ok(list_session_families()?.len())
}

fn open_connection() -> Result<Connection> {
    Connection::open(db_path()?).context("Failed to open OpenCode sqlite database")
}

fn session_id_from_path(path: &Path) -> Result<String> {
    path.file_stem()
        .and_then(|value| value.to_str())
        .map(ToString::to_string)
        .ok_or_else(|| {
            anyhow!(
                "Unable to determine OpenCode session id from {}",
                path.display()
            )
        })
}

fn list_session_rows() -> Result<Vec<OpenCodeSessionRow>> {
    let connection = open_connection()?;
    let mut statement = connection.prepare(
        "SELECT id, parent_id, directory, title, time_created, time_updated FROM session ORDER BY time_updated DESC",
    )?;
    let rows = statement.query_map([], |row| {
        Ok(OpenCodeSessionRow {
            id: row.get(0)?,
            parent_id: row.get(1)?,
            directory: row.get(2)?,
            title: row.get(3)?,
            time_created: row.get(4)?,
            time_updated: row.get(5)?,
        })
    })?;

    rows.collect::<std::result::Result<Vec<_>, _>>()
        .context("Failed to read OpenCode sessions")
}

fn list_session_families() -> Result<Vec<OpenCodeSessionFamily>> {
    let rows = list_session_rows()?;
    let by_id = rows
        .iter()
        .cloned()
        .map(|row| (row.id.clone(), row))
        .collect::<HashMap<_, _>>();
    let mut grouped = HashMap::<String, Vec<OpenCodeSessionRow>>::new();

    for row in rows {
        let family_root_id = root_session_id(&row, &by_id);
        grouped.entry(family_root_id).or_default().push(row);
    }

    let mut families = grouped
        .into_iter()
        .filter_map(|(root_id, mut members)| {
            members.sort_by(|left, right| {
                left.time_created
                    .cmp(&right.time_created)
                    .then_with(|| left.id.cmp(&right.id))
            });

            let root = members
                .iter()
                .find(|row| row.id == root_id)
                .cloned()
                .or_else(|| members.first().cloned())?;

            Some(OpenCodeSessionFamily { root, members })
        })
        .collect::<Vec<_>>();

    families.sort_by(|left, right| {
        right
            .root
            .time_updated
            .cmp(&left.root.time_updated)
            .then_with(|| left.root.id.cmp(&right.root.id))
    });

    Ok(families)
}

fn root_session_id(
    row: &OpenCodeSessionRow,
    by_id: &HashMap<String, OpenCodeSessionRow>,
) -> String {
    let mut current_id = row.id.clone();
    let mut parent_id = row.parent_id.clone();
    let mut visited = HashSet::from([current_id.clone()]);

    while let Some(next_parent_id) = parent_id {
        if !visited.insert(next_parent_id.clone()) {
            break;
        }

        let Some(parent) = by_id.get(&next_parent_id) else {
            current_id = next_parent_id;
            break;
        };

        current_id = parent.id.clone();
        parent_id = parent.parent_id.clone();
    }

    current_id
}

fn parse_summary(path: &Path) -> Result<SessionSummary> {
    let session_id = session_id_from_path(path)?;
    let family = session_family(&session_id)?;
    let row = &family.root;

    Ok(SessionSummary {
        source_app: SourceApp::OpenCode,
        source_session_id: row.id.clone(),
        title: family_title(&family),
        cwd: Some(row.directory.clone()),
        git_branch: None,
        transcript_path: path.display().to_string(),
        created_at: Some(family_created_at(&family)),
        updated_at: Some(family_updated_at(&family)),
    })
}

fn parse_overview(path: &Path) -> Result<SessionDetailOverview> {
    let session_id = session_id_from_path(path)?;
    let summary = parse_summary(path)?;
    let family = session_family(&session_id)?;
    let member_ids: Vec<&str> = family.members.iter().map(|row| row.id.as_str()).collect();
    let marker_count = family.members.len().saturating_sub(1);
    let (message_count, event_count) = count_family_records(&member_ids)?;

    Ok(SessionDetailOverview {
        summary,
        source_paths: family_source_paths(&family),
        message_count: Some(message_count + marker_count),
        event_count: Some(event_count + marker_count),
        agents: family
            .members
            .iter()
            .map(|row| SessionAgent {
                session_id: row.id.clone(),
                label: if row.id == family.root.id {
                    "主 Agent".to_string()
                } else {
                    row.title.clone()
                },
                is_root: row.id == family.root.id,
            })
            .collect(),
    })
}

fn parse_messages_page(path: &Path, offset: usize, limit: usize) -> Result<SessionMessagePage> {
    let session_id = session_id_from_path(path)?;
    let family = session_family(&session_id)?;
    let all_messages = cached_messages_for_family(&family)?;
    let total_count = all_messages.len();
    let start = offset.min(total_count);
    let end = start.saturating_add(limit).min(total_count);
    let page_messages = all_messages[start..end].to_vec();
    let next_offset = (end < total_count).then_some(end);

    Ok(SessionMessagePage {
        messages: page_messages,
        offset: start,
        limit,
        next_offset,
        total_count,
        has_more: next_offset.is_some(),
    })
}

fn parse_events_page(path: &Path, offset: usize, limit: usize) -> Result<SessionEventPage> {
    let session_id = session_id_from_path(path)?;
    let family = session_family(&session_id)?;
    let all_events = cached_events_for_family(&family)?;
    let total_count = all_events.len();
    let start = offset.min(total_count);
    let end = start.saturating_add(limit).min(total_count);
    let page_events = all_events[start..end].to_vec();
    let next_offset = (end < total_count).then_some(end);

    Ok(SessionEventPage {
        events: page_events,
        offset: start,
        limit,
        next_offset,
        total_count,
        has_more: next_offset.is_some(),
    })
}

fn parse_detail(path: &Path) -> Result<SessionDetail> {
    let session_id = session_id_from_path(path)?;
    let summary = parse_summary(path)?;
    let family = session_family(&session_id)?;
    let messages = cached_messages_for_family(&family)?;
    let events = cached_events_for_family(&family)?;

    Ok(SessionDetail {
        summary,
        source_paths: family_source_paths(&family),
        messages,
        events,
    })
}

fn session_family(session_id: &str) -> Result<OpenCodeSessionFamily> {
    let rows = list_session_rows()?;
    let by_id = rows
        .iter()
        .cloned()
        .map(|row| (row.id.clone(), row))
        .collect::<HashMap<_, _>>();
    let target = by_id
        .get(session_id)
        .cloned()
        .ok_or_else(|| anyhow!("Could not find OpenCode session {session_id}"))?;
    let root_id = root_session_id(&target, &by_id);
    let mut members = rows
        .into_iter()
        .filter(|row| root_session_id(row, &by_id) == root_id)
        .collect::<Vec<_>>();

    members.sort_by(|left, right| {
        left.time_created
            .cmp(&right.time_created)
            .then_with(|| left.id.cmp(&right.id))
    });

    let root = by_id
        .get(&root_id)
        .cloned()
        .ok_or_else(|| anyhow!("Could not find OpenCode root session {root_id}"))?;

    Ok(OpenCodeSessionFamily { root, members })
}

fn family_title(family: &OpenCodeSessionFamily) -> String {
    let child_count = family.members.len().saturating_sub(1);

    if child_count == 0 {
        return family.root.title.clone();
    }

    format!("{} (+{} subagents)", family.root.title, child_count)
}

fn family_created_at(family: &OpenCodeSessionFamily) -> i64 {
    family
        .members
        .iter()
        .map(|row| row.time_created)
        .min()
        .unwrap_or(family.root.time_created)
}

fn family_updated_at(family: &OpenCodeSessionFamily) -> i64 {
    family
        .members
        .iter()
        .map(|row| row.time_updated)
        .max()
        .unwrap_or(family.root.time_updated)
}

fn family_source_paths(family: &OpenCodeSessionFamily) -> Vec<String> {
    let mut paths = vec![db_path()
        .map(|path| path.display().to_string())
        .unwrap_or_else(|_| "<opencode-db>".to_string())];

    paths.extend(
        family
            .members
            .iter()
            .map(|row| session_path(&row.id).display().to_string()),
    );

    paths
}

fn cached_messages_for_family(family: &OpenCodeSessionFamily) -> Result<Vec<SessionMessage>> {
    let cache_key = family.root.id.clone();
    let updated_at = family_updated_at(family);

    if let Some(messages) = OPEN_CODE_TIMELINE_CACHE
        .lock()
        .map_err(|_| anyhow!("OpenCode timeline cache lock was poisoned"))?
        .get(&cache_key)
        .and_then(|entry| {
            (entry.updated_at == updated_at)
                .then(|| entry.messages.clone())
                .flatten()
        })
    {
        return Ok(messages);
    }

    let messages = load_messages_for_family(family)?;
    let mut cache = OPEN_CODE_TIMELINE_CACHE
        .lock()
        .map_err(|_| anyhow!("OpenCode timeline cache lock was poisoned"))?;
    let entry = cache.entry(cache_key).or_default();
    entry.updated_at = updated_at;
    entry.messages = Some(messages.clone());
    Ok(messages)
}

fn cached_events_for_family(family: &OpenCodeSessionFamily) -> Result<Vec<SessionEvent>> {
    let cache_key = family.root.id.clone();
    let updated_at = family_updated_at(family);

    if let Some(events) = OPEN_CODE_TIMELINE_CACHE
        .lock()
        .map_err(|_| anyhow!("OpenCode timeline cache lock was poisoned"))?
        .get(&cache_key)
        .and_then(|entry| {
            (entry.updated_at == updated_at)
                .then(|| entry.events.clone())
                .flatten()
        })
    {
        return Ok(events);
    }

    let events = load_events_for_family(family)?;
    let mut cache = OPEN_CODE_TIMELINE_CACHE
        .lock()
        .map_err(|_| anyhow!("OpenCode timeline cache lock was poisoned"))?;
    let entry = cache.entry(cache_key).or_default();
    entry.updated_at = updated_at;
    entry.events = Some(events.clone());
    Ok(events)
}

fn count_family_records(member_ids: &[&str]) -> Result<(usize, usize)> {
    let connection = open_connection()?;
    let placeholders: Vec<String> = member_ids.iter().enumerate().map(|(i, _)| format!("?{}", i + 1)).collect::<Vec<_>>();
    let placeholders_joined = placeholders.join(",");
    let params: Vec<Box<dyn rusqlite::types::ToSql>> = member_ids.iter().map(|id| Box::new(id.to_string()) as Box<dyn rusqlite::types::ToSql>).collect();
    let param_refs: Vec<&dyn rusqlite::types::ToSql> = params.iter().map(|p| p.as_ref()).collect();

    let message_count: usize = connection.query_row(
        &format!("SELECT COUNT(*) FROM message WHERE session_id IN ({placeholders_joined})"),
        param_refs.as_slice(),
        |row| row.get(0),
    ).unwrap_or(0);

    let or_clauses: Vec<String> = member_ids.iter().enumerate().map(|(i, _)| format!("session_id = ?{}", i + 1)).collect::<Vec<_>>();
    let or_joined = or_clauses.join(" OR ");
    let event_count: usize = connection.query_row(
        &format!(
            "SELECT COUNT(*) FROM part WHERE ({or_joined}) AND type NOT IN ('text','reasoning','tool','patch','file','step-start','step-finish')"
        ),
        param_refs.as_slice(),
        |row| row.get(0),
    ).unwrap_or(0);

    Ok((message_count, event_count))
}

fn load_messages(session_id: &str) -> Result<Vec<SessionMessage>> {
    let connection = open_connection()?;
    let mut statement = connection.prepare(
        "SELECT id, time_created, data FROM message WHERE session_id = ?1 ORDER BY time_created ASC",
    )?;
    let rows = statement.query_map(params![session_id], |row| {
        Ok((
            row.get::<_, String>(0)?,
            row.get::<_, i64>(1)?,
            row.get::<_, String>(2)?,
        ))
    })?;

    let mut messages = Vec::new();

    for row in rows {
        let (message_id, time_created, raw_data) = row?;
        let value: Value = serde_json::from_str(&raw_data)
            .with_context(|| format!("Invalid OpenCode message JSON for {message_id}"))?;
        let role = crate::json_string(&value, &["role"]).unwrap_or_else(|| "unknown".to_string());
        let timestamp = value
            .get("time")
            .and_then(|time| time.get("created"))
            .and_then(Value::as_i64)
            .or(Some(time_created));
        let mut blocks = load_message_blocks(&connection, &message_id, session_id)?;

        if role == "user" {
            blocks = crate::sanitize_user_blocks(blocks);
        }

        if !blocks.is_empty() {
            messages.push(SessionMessage {
                id: message_id,
                role,
                timestamp,
                blocks,
                session_id: Some(session_id.to_string()),
            });
        }
    }

    Ok(messages)
}

fn load_messages_for_family(family: &OpenCodeSessionFamily) -> Result<Vec<SessionMessage>> {
    let mut messages = Vec::new();

    for row in &family.members {
        if row.id != family.root.id {
            messages.push(session_marker_message(row, "subagent_started"));
        }

        messages.extend(load_messages(&row.id)?);
    }

    messages.sort_by(|left, right| {
        left.timestamp
            .cmp(&right.timestamp)
            .then_with(|| left.id.cmp(&right.id))
    });

    Ok(messages)
}

fn load_message_blocks(
    connection: &Connection,
    message_id: &str,
    session_id: &str,
) -> Result<Vec<ContentBlock>> {
    let mut statement = connection.prepare(
        "SELECT id, data FROM part WHERE session_id = ?1 AND message_id = ?2 ORDER BY time_created ASC",
    )?;
    let rows = statement.query_map(params![session_id, message_id], |row| {
        Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))
    })?;

    let mut blocks = Vec::new();

    for row in rows {
        let (_part_id, raw_data) = row?;
        let value: Value = serde_json::from_str(&raw_data)
            .with_context(|| format!("Invalid OpenCode part JSON for message {message_id}"))?;
        let kind = crate::json_string(&value, &["type"]).unwrap_or_else(|| "unknown".to_string());

        if matches!(kind.as_str(), "step-start" | "step-finish") {
            continue;
        }

        if kind == "tool" {
            blocks.extend(tool_blocks(&value));
            continue;
        }

        let normalized_kind = match kind.as_str() {
            "reasoning" => "thinking".to_string(),
            _ => kind.clone(),
        };

        let text = match kind.as_str() {
            "patch" => value.get("files").and_then(Value::as_array).map(|files| {
                let mut lines = vec!["变更文件：".to_string()];
                lines.extend(
                    files
                        .iter()
                        .filter_map(Value::as_str)
                        .map(|file| format!("- {file}")),
                );
                lines.join("\n")
            }),
            "file" => {
                let filename = crate::json_string(&value, &["filename"]);
                let mime = crate::json_string(&value, &["mime"]);

                match (filename, mime) {
                    (Some(filename), Some(mime)) => Some(format!("文件：{filename}\n类型：{mime}")),
                    (Some(filename), None) => Some(format!("文件：{filename}")),
                    (None, Some(mime)) => Some(format!("文件类型：{mime}")),
                    (None, None) => None,
                }
            }
            _ => crate::json_string(&value, &["text"]),
        };

        blocks.push(ContentBlock {
            kind: normalized_kind,
            text,
            tool_name: crate::json_string(&value, &["tool"]),
            tool_call_id: crate::json_string(&value, &["callID"]),
            payload: Some(value),
        });
    }

    Ok(blocks)
}

fn load_events(session_id: &str) -> Result<Vec<SessionEvent>> {
    let connection = open_connection()?;
    let mut statement = connection.prepare(
        "SELECT id, time_created, data FROM part WHERE session_id = ?1 ORDER BY time_created ASC",
    )?;
    let rows = statement.query_map(params![session_id], |row| {
        Ok((
            row.get::<_, String>(0)?,
            row.get::<_, i64>(1)?,
            row.get::<_, String>(2)?,
        ))
    })?;

    let mut events = Vec::new();

    for row in rows {
        let (part_id, time_created, raw_data) = row?;
        let value: Value = serde_json::from_str(&raw_data)
            .with_context(|| format!("Invalid OpenCode event JSON for {part_id}"))?;
        let kind = crate::json_string(&value, &["type"]).unwrap_or_else(|| "unknown".to_string());

        if matches!(
            kind.as_str(),
            "text" | "reasoning" | "tool" | "patch" | "file"
        ) {
            continue;
        }

        events.push(SessionEvent {
            id: part_id,
            kind: kind.clone(),
            timestamp: value
                .get("time")
                .and_then(|time| time.get("created"))
                .and_then(Value::as_i64)
                .or(Some(time_created)),
            summary: crate::summarize_event(kind.as_str(), &value),
            payload: Some(value),
            session_id: Some(session_id.to_string()),
        });
    }

    Ok(events)
}

fn load_events_for_family(family: &OpenCodeSessionFamily) -> Result<Vec<SessionEvent>> {
    let mut events = Vec::new();

    for row in &family.members {
        if row.id != family.root.id {
            events.push(session_marker_event(row, "subagent_started"));
        }

        events.extend(load_events(&row.id)?);
    }

    events.sort_by(|left, right| {
        left.timestamp
            .cmp(&right.timestamp)
            .then_with(|| left.id.cmp(&right.id))
    });

    Ok(events)
}

fn tool_blocks(value: &Value) -> Vec<ContentBlock> {
    let tool_name = crate::json_string(value, &["tool"]);
    let tool_call_id = crate::json_string(value, &["callID"]);
    let input = value.get("state").and_then(|state| state.get("input"));
    let output = value.get("state").and_then(|state| state.get("output"));
    let mut blocks = Vec::new();

    if input.is_some_and(|item| !item.is_null()) {
        let mut payload = value.clone();
        if let Some(state) = payload.get_mut("state").and_then(Value::as_object_mut) {
            state.remove("output");
        }

        blocks.push(ContentBlock {
            kind: "function_call".to_string(),
            text: tool_input_text(input),
            tool_name: tool_name.clone(),
            tool_call_id: tool_call_id.clone(),
            payload: Some(tool_payload(&payload, input.cloned(), None)),
        });
    }

    if output.is_some_and(|item| !item.is_null()) {
        let mut payload = value.clone();
        if let Some(state) = payload.get_mut("state").and_then(Value::as_object_mut) {
            state.remove("input");
        }

        blocks.push(ContentBlock {
            kind: "function_call_output".to_string(),
            text: tool_output_text(output),
            tool_name,
            tool_call_id,
            payload: Some(tool_payload(&payload, input.cloned(), output.cloned())),
        });
    }

    blocks
}

fn tool_payload(base: &Value, input: Option<Value>, output: Option<Value>) -> Value {
    let mut payload = base.clone();

    if let Some(object) = payload.as_object_mut() {
        if let Some(input) = input {
            object.insert("input".to_string(), input);
        }

        if let Some(output) = output {
            object.insert("output".to_string(), output);
        }
    }

    payload
}

fn tool_input_text(input: Option<&Value>) -> Option<String> {
    let Some(input) = input else {
        return None;
    };

    if let Some(text) = input.as_str() {
        return Some(text.to_string());
    }

    crate::stringify_json(input)
}

fn tool_output_text(output: Option<&Value>) -> Option<String> {
    let Some(output) = output else {
        return None;
    };

    if let Some(text) = output.as_str() {
        return Some(text.to_string());
    }

    crate::stringify_json(output)
}

fn session_marker_message(row: &OpenCodeSessionRow, kind: &str) -> SessionMessage {
    SessionMessage {
        id: format!("opencode-{}-{}", kind, row.id),
        role: "assistant".to_string(),
        timestamp: Some(row.time_created),
        blocks: vec![ContentBlock {
            kind: "output_text".to_string(),
            text: Some(format!("Sub-agent session: {}\n{}", row.title, row.id)),
            tool_name: None,
            tool_call_id: None,
            payload: Some(json!({
                "type": kind,
                "session_id": row.id,
                "title": row.title,
                "directory": row.directory,
                "parent_id": row.parent_id,
            })),
        }],
        session_id: Some(row.id.clone()),
    }
}

fn session_marker_event(row: &OpenCodeSessionRow, kind: &str) -> SessionEvent {
    SessionEvent {
        id: format!("opencode-{}-{}", kind, row.id),
        kind: kind.to_string(),
        timestamp: Some(row.time_created),
        summary: format!("Sub-agent session started: {}", row.title),
        payload: Some(json!({
            "session_id": row.id,
            "title": row.title,
            "directory": row.directory,
            "parent_id": row.parent_id,
        })),
        session_id: Some(row.id.clone()),
    }
}

fn write_session(detail: &SessionDetail, new_session_id: &str) -> Result<(String, Vec<String>)> {
    let sessions_before = list_session_rows().unwrap_or_default();
    let payload = import_payload(detail, new_session_id)?;
    let import_file = std::env::temp_dir().join(format!(
        "agent-session-hub-opencode-import-{}.json",
        Uuid::new_v4().simple()
    ));

    if let Some(parent) = import_file.parent() {
        fs::create_dir_all(parent)?;
    }

    let serialized = serde_json::to_string_pretty(&payload)?;
    fs::write(&import_file, serialized)
        .with_context(|| format!("Failed to write {}", import_file.display()))?;

    let output = Command::new("opencode")
        .arg("import")
        .arg(&import_file)
        .output()
        .context("Failed to execute opencode import")?;

    fs::remove_file(&import_file).ok();

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        let stdout = String::from_utf8_lossy(&output.stdout);
        let exit_code = output
            .status
            .code()
            .map(|code| code.to_string())
            .unwrap_or_else(|| "terminated by signal".to_string());
        bail!(
            "OpenCode import command failed. exit_code: {exit_code}\nstdout:\n{}\nstderr:\n{}",
            stdout.trim(),
            stderr.trim()
        );
    }

    let sessions_after = list_session_rows().unwrap_or_default();
    let created_session_id = resolve_imported_session_id(
        &output.stdout,
        &output.stderr,
        &sessions_before,
        &sessions_after,
        detail,
    )?;

    Ok((created_session_id, vec![db_path()?.display().to_string()]))
}

fn import_payload(detail: &SessionDetail, session_id: &str) -> Result<Value> {
    let created_at = detail
        .summary
        .created_at
        .unwrap_or_else(|| chrono::Utc::now().timestamp_millis());
    let updated_at = detail.summary.updated_at.unwrap_or(created_at);
    let cwd = crate::effective_cwd(detail.summary.cwd.as_deref())?;
    let project_id = format!("project_{}", Uuid::new_v4().simple());
    let mut stats = OpenCodeImportStats::default();
    let mut messages = Vec::new();
    let mut last_user_message_id: Option<String> = None;
    let mut previous_message_id: Option<String> = None;

    for (message_index, message) in detail.messages.iter().enumerate() {
        let message_id = format!("msg_{}", Uuid::new_v4().simple());
        let message_timestamp = message
            .timestamp
            .unwrap_or(created_at + message_index as i64);
        let parent_message_id = if message.role == "user" {
            None
        } else {
            Some(
                last_user_message_id
                    .clone()
                    .or_else(|| previous_message_id.clone())
                    .unwrap_or_else(|| session_id.to_string()),
            )
        };
        let parts = message_parts(
            session_id,
            &message_id,
            message,
            message_timestamp,
            &mut stats,
        );

        if parts.is_empty() {
            continue;
        }

        messages.push(json!({
            "info": message_info(
                session_id,
                &message_id,
                message,
                message_timestamp,
                &cwd,
                parent_message_id.as_deref(),
            ),
            "parts": parts,
        }));

        if message.role == "user" {
            last_user_message_id = Some(message_id.clone());
        }

        previous_message_id = Some(message_id);
    }

    Ok(json!({
        "info": {
            "id": session_id,
            "slug": slugify_title(&detail.summary.title),
            "projectID": project_id,
            "directory": cwd,
            "title": detail.summary.title,
            "version": "1.4.3",
            "summary": {
                "additions": stats.additions,
                "deletions": stats.deletions,
                "files": stats.files,
            },
            "time": {
                "created": created_at,
                "updated": updated_at,
            }
        },
        "messages": messages,
    }))
}

fn message_info(
    session_id: &str,
    message_id: &str,
    message: &SessionMessage,
    timestamp: i64,
    cwd: &str,
    parent_message_id: Option<&str>,
) -> Value {
    match message.role.as_str() {
        "user" => json!({
            "id": message_id,
            "sessionID": session_id,
            "role": "user",
            "time": {
                "created": timestamp,
            },
            "agent": "build",
            "model": {
                "providerID": "imported",
                "modelID": "imported",
                "variant": "default",
            }
        }),
        _ => json!({
            "id": message_id,
            "sessionID": session_id,
            "parentID": parent_message_id.unwrap_or(session_id),
            "role": "assistant",
            "mode": "build",
            "agent": "build",
            "variant": "default",
            "path": {
                "cwd": cwd,
                "root": cwd,
            },
            "cost": 0,
            "tokens": {
                "total": 0,
                "input": 0,
                "output": 0,
                "reasoning": 0,
                "cache": {
                    "read": 0,
                    "write": 0,
                }
            },
            "modelID": "imported",
            "providerID": "imported",
            "time": {
                "created": timestamp,
                "completed": timestamp,
            },
            "finish": "stop",
        }),
    }
}

fn message_parts(
    session_id: &str,
    message_id: &str,
    message: &SessionMessage,
    timestamp: i64,
    stats: &mut OpenCodeImportStats,
) -> Vec<Value> {
    let mut parts = Vec::new();

    for block in &message.blocks {
        match block.kind.as_str() {
            "text" | "input_text" | "output_text" => {
                if let Some(text) = crate::block_text(block) {
                    parts.push(json!({
                        "id": format!("prt_{}", Uuid::new_v4().simple()),
                        "sessionID": session_id,
                        "messageID": message_id,
                        "type": "text",
                        "text": text,
                        "time": {
                            "start": timestamp,
                            "end": timestamp,
                        }
                    }));
                }
            }
            "thinking" | "reasoning" => {
                if let Some(text) = crate::block_text(block) {
                    parts.push(json!({
                        "id": format!("prt_{}", Uuid::new_v4().simple()),
                        "sessionID": session_id,
                        "messageID": message_id,
                        "type": "reasoning",
                        "text": text,
                        "time": {
                            "start": timestamp,
                            "end": timestamp,
                        }
                    }));
                }
            }
            "tool_use" | "function_call" => {
                let title = block
                    .tool_name
                    .clone()
                    .unwrap_or_else(|| "imported_tool".to_string());
                parts.push(json!({
                    "id": format!("prt_{}", Uuid::new_v4().simple()),
                    "sessionID": session_id,
                    "messageID": message_id,
                    "type": "tool",
                    "tool": title,
                    "callID": block.tool_call_id.clone().unwrap_or_else(|| format!("call_{}", Uuid::new_v4().simple())),
                    "state": {
                        "status": "running",
                        "input": tool_input(block),
                        "title": block.tool_name.clone().unwrap_or_else(|| "imported_tool".to_string()),
                        "metadata": {},
                        "time": {
                            "start": timestamp,
                        }
                    }
                }));
            }
            "tool_result" | "function_call_output" => {
                let title = block
                    .tool_name
                    .clone()
                    .unwrap_or_else(|| "imported_tool".to_string());
                parts.push(json!({
                    "id": format!("prt_{}", Uuid::new_v4().simple()),
                    "sessionID": session_id,
                    "messageID": message_id,
                    "type": "tool",
                    "tool": title,
                    "callID": block.tool_call_id.clone().unwrap_or_else(|| format!("call_{}", Uuid::new_v4().simple())),
                    "state": {
                        "status": "completed",
                        "input": tool_input(block),
                        "output": tool_output(block),
                        "title": block.tool_name.clone().unwrap_or_else(|| "imported_tool".to_string()),
                        "metadata": {},
                        "time": {
                            "start": timestamp,
                            "end": timestamp,
                        }
                    }
                }));
            }
            "patch" => {
                if let Some(files) = patch_files(block) {
                    stats.files += files.len();
                    parts.push(json!({
                        "id": format!("prt_{}", Uuid::new_v4().simple()),
                        "sessionID": session_id,
                        "messageID": message_id,
                        "type": "patch",
                        "hash": format!("patch_{}", Uuid::new_v4().simple()),
                        "files": files,
                    }));
                }
            }
            "file" => {
                let mut file_part = json!({
                    "id": format!("prt_{}", Uuid::new_v4().simple()),
                    "sessionID": session_id,
                    "messageID": message_id,
                    "type": "file",
                });

                if let Some(payload) = block.payload.as_ref() {
                    if let Some(filename) = crate::json_string(payload, &["filename"]) {
                        file_part["filename"] = Value::String(filename);
                    }
                    if let Some(mime) = crate::json_string(payload, &["mime"]) {
                        file_part["mime"] = Value::String(mime);
                    }
                    if let Some(url) = crate::json_string(payload, &["url"]) {
                        file_part["url"] = Value::String(url);
                    }
                }

                parts.push(file_part);
            }
            _ => {}
        }
    }

    parts
}

fn tool_input(block: &ContentBlock) -> Value {
    if let Some(payload) = block.payload.as_ref() {
        if let Some(input) = payload.get("input") {
            return input.clone();
        }
    }

    block
        .text
        .as_deref()
        .map(|text| json!({ "raw": text }))
        .unwrap_or_else(|| json!({}))
}

fn tool_output(block: &ContentBlock) -> String {
    if let Some(payload) = block.payload.as_ref() {
        if let Some(output) = payload.get("output") {
            if let Some(text) = output.as_str() {
                return text.to_string();
            }

            return crate::stringify_json(output).unwrap_or_default();
        }
    }

    block.text.clone().unwrap_or_default()
}

fn patch_files(block: &ContentBlock) -> Option<Vec<String>> {
    block.payload.as_ref().and_then(|payload| {
        payload
            .get("files")
            .and_then(Value::as_array)
            .map(|files| {
                files
                    .iter()
                    .filter_map(Value::as_str)
                    .map(ToString::to_string)
                    .collect::<Vec<_>>()
            })
            .filter(|files| !files.is_empty())
    })
}

fn parse_imported_session_id(stdout: &[u8], stderr: &[u8]) -> Option<String> {
    let combined = format!(
        "{}\n{}",
        String::from_utf8_lossy(stdout),
        String::from_utf8_lossy(stderr)
    );

    combined
        .lines()
        .find_map(|line| line.strip_prefix("Imported session: "))
        .map(|value| value.trim().to_string())
}

fn resolve_imported_session_id(
    stdout: &[u8],
    stderr: &[u8],
    sessions_before: &[OpenCodeSessionRow],
    sessions_after: &[OpenCodeSessionRow],
    detail: &SessionDetail,
) -> Result<String> {
    if let Some(session_id) = parse_imported_session_id(stdout, stderr) {
        return Ok(session_id);
    }

    let known_ids = sessions_before
        .iter()
        .map(|row| row.id.clone())
        .collect::<HashSet<_>>();
    let mut imported_candidates = sessions_after
        .iter()
        .filter(|row| !known_ids.contains(&row.id))
        .collect::<Vec<_>>();

    imported_candidates.sort_by(|left, right| right.time_updated.cmp(&left.time_updated));

    if let Some(candidate) = imported_candidates.into_iter().find(|row| {
        row.title == detail.summary.title
            || detail
                .summary
                .cwd
                .as_deref()
                .is_some_and(|cwd| row.directory == cwd)
    }) {
        return Ok(candidate.id.clone());
    }

    if let Some(candidate) = sessions_after.iter().max_by(|left, right| {
        left.time_updated
            .cmp(&right.time_updated)
            .then_with(|| left.time_created.cmp(&right.time_created))
    }) {
        if !known_ids.contains(&candidate.id) {
            return Ok(candidate.id.clone());
        }
    }

    let stdout_text = String::from_utf8_lossy(stdout);
    let stderr_text = String::from_utf8_lossy(stderr);
    bail!(
        "OpenCode import may have succeeded, but the created session id could not be determined.\nstdout:\n{}\nstderr:\n{}",
        stdout_text.trim(),
        stderr_text.trim()
    )
}

fn slugify_title(title: &str) -> String {
    let mut slug = String::new();
    let mut previous_dash = false;

    for ch in title.chars() {
        let normalized = if ch.is_ascii_alphanumeric() {
            Some(ch.to_ascii_lowercase())
        } else if ch.is_whitespace() || matches!(ch, '-' | '_' | '/' | '.') {
            Some('-')
        } else {
            None
        };

        match normalized {
            Some('-') if !previous_dash && !slug.is_empty() => {
                slug.push('-');
                previous_dash = true;
            }
            Some(value) if value != '-' => {
                slug.push(value);
                previous_dash = false;
            }
            _ => {}
        }
    }

    let trimmed = slug.trim_matches('-');

    if trimmed.is_empty() {
        return "imported-session".to_string();
    }

    trimmed.to_string()
}
