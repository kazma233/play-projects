use std::collections::HashMap;
use std::fs::{self, File};
use std::io::{BufRead, BufReader};
use std::path::{Path, PathBuf};
use std::sync::{LazyLock, Mutex};

use anyhow::{anyhow, Context, Result};
use chrono::Utc;
use dirs::home_dir;
use serde::Deserialize;
use serde_json::{json, Map, Value};
use uuid::Uuid;

use crate::{
    ContentBlock, SessionAgent, SessionDetail, SessionDetailOverview, SessionEvent,
    SessionEventPage, SessionFileEntry, SessionMessage, SessionMessagePage, SessionSummary,
    SourceApp, SummaryAccumulator, TimelineRecord,
};

use super::{SessionExporter, SessionReader};

pub(crate) struct ClaudeCodeBackend;

pub(crate) static BACKEND: ClaudeCodeBackend = ClaudeCodeBackend;

#[derive(Clone, Debug)]
struct ClaudeSessionRow {
    path: PathBuf,
    summary: SessionSummary,
    agent_session_id: String,
    agent_label: Option<String>,
    is_root: bool,
}

#[derive(Clone, Debug)]
struct ClaudeSessionFamily {
    root: ClaudeSessionRow,
    members: Vec<ClaudeSessionRow>,
}

#[derive(Clone, Default)]
struct ClaudeTimelineCacheEntry {
    updated_at: i64,
    messages: Option<Vec<SessionMessage>>,
    events: Option<Vec<SessionEvent>>,
}

#[derive(Clone, Default)]
struct ClaudeFamilyIndexCacheEntry {
    updated_at: i64,
    families: Vec<ClaudeSessionFamily>,
    sessions_by_path: HashMap<String, ClaudeSessionFamily>,
}

#[derive(Deserialize)]
struct ClaudeAgentMeta {
    #[serde(rename = "agentType")]
    agent_type: Option<String>,
    description: Option<String>,
    name: Option<String>,
}

static CLAUDE_TIMELINE_CACHE: LazyLock<Mutex<HashMap<String, ClaudeTimelineCacheEntry>>> =
    LazyLock::new(|| Mutex::new(HashMap::new()));
static CLAUDE_FAMILY_INDEX_CACHE: LazyLock<Mutex<Option<ClaudeFamilyIndexCacheEntry>>> =
    LazyLock::new(|| Mutex::new(None));

fn lock_timeline_cache(
) -> Result<std::sync::MutexGuard<'static, HashMap<String, ClaudeTimelineCacheEntry>>> {
    CLAUDE_TIMELINE_CACHE
        .lock()
        .map_err(|_| anyhow!("Claude timeline cache lock was poisoned"))
}

fn lock_family_index_cache(
) -> Result<std::sync::MutexGuard<'static, Option<ClaudeFamilyIndexCacheEntry>>> {
    CLAUDE_FAMILY_INDEX_CACHE
        .lock()
        .map_err(|_| anyhow!("Claude family index cache lock was poisoned"))
}

impl SessionReader for ClaudeCodeBackend {
    fn list_entries(&self) -> Result<Vec<SessionFileEntry>> {
        let mut entries = list_session_families()?
            .into_iter()
            .map(|family| {
                let summary = family_summary(&family);
                let sort_timestamp = family_updated_at(&family);
                SessionFileEntry {
                    path: family.root.path.clone(),
                    sort_timestamp,
                    summary: Some(summary),
                }
            })
            .collect::<Vec<_>>();

        super::sort_entries(&mut entries);
        Ok(entries)
    }

    fn clear_cache(&self) -> Result<()> {
        lock_timeline_cache()?.clear();
        *lock_family_index_cache()? = None;
        Ok(())
    }

    fn resolve_path(&self, source_session_id: &str) -> Result<PathBuf> {
        find_session_file(source_session_id)
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

impl SessionExporter for ClaudeCodeBackend {
    fn planned_import_paths(
        &self,
        summary: &SessionSummary,
        session_id: &str,
    ) -> Result<Vec<String>> {
        let cwd = crate::effective_cwd(summary.cwd.as_deref())?;
        let path = root()?
            .join("projects")
            .join(project_slug(&cwd))
            .join(format!("{session_id}.jsonl"));
        Ok(vec![path.display().to_string()])
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
        .join(".claude"))
}

pub(crate) fn find_session_file(source_session_id: &str) -> Result<PathBuf> {
    crate::find_session_file(&root()?.join("projects"), source_session_id)
}

pub(crate) fn count_sessions() -> Result<usize> {
    Ok(list_session_families()?.len())
}

fn list_session_rows() -> Result<Vec<ClaudeSessionRow>> {
    let mut rows = Vec::new();

    for path in crate::enumerate_jsonl_files(&root()?.join("projects"))? {
        if let Ok(row) = parse_session_row(&path) {
            rows.push(row);
        }
    }

    Ok(rows)
}

fn list_session_families() -> Result<Vec<ClaudeSessionFamily>> {
    Ok(family_index()?.families)
}

fn family_index() -> Result<ClaudeFamilyIndexCacheEntry> {
    let updated_at = claude_projects_timestamp()?;

    if let Some(entry) = lock_family_index_cache()?
        .as_ref()
        .filter(|entry| entry.updated_at == updated_at)
        .cloned()
    {
        return Ok(entry);
    }

    let rows = list_session_rows()?;
    let by_id = rows
        .iter()
        .cloned()
        .map(|row| (row.summary.source_session_id.clone(), row))
        .collect::<HashMap<_, _>>();
    let mut grouped = HashMap::<String, Vec<ClaudeSessionRow>>::new();

    for row in rows {
        grouped
            .entry(row.summary.source_session_id.clone())
            .or_default()
            .push(row);
    }

    let mut families = grouped
        .into_iter()
        .filter_map(|(root_id, mut members)| {
            members.sort_by(|left, right| {
                left.summary
                    .created_at
                    .cmp(&right.summary.created_at)
                    .then_with(|| left.path.cmp(&right.path))
            });

            let root = members
                .iter()
                .find(|row| row.is_root)
                .cloned()
                .or_else(|| {
                    by_id
                        .get(&root_id)
                        .cloned()
                        .or_else(|| members.first().cloned())
                })?;

            Some(ClaudeSessionFamily { root, members })
        })
        .collect::<Vec<_>>();

    families.sort_by(|left, right| {
        family_updated_at(right)
            .cmp(&family_updated_at(left))
            .then_with(|| left.root.path.cmp(&right.root.path))
    });

    let mut sessions_by_path = HashMap::new();

    for family in &families {
        for member in &family.members {
            sessions_by_path.insert(member.path.display().to_string(), family.clone());
        }
    }

    let entry = ClaudeFamilyIndexCacheEntry {
        updated_at,
        families,
        sessions_by_path,
    };

    *lock_family_index_cache()? = Some(entry.clone());

    Ok(entry)
}

fn session_family_for_path(path: &Path) -> Result<ClaudeSessionFamily> {
    let key = path.display().to_string();
    family_index()?
        .sessions_by_path
        .get(&key)
        .cloned()
        .ok_or_else(|| anyhow!("Could not find Claude Code session for {}", path.display()))
}

fn parse_session_row(path: &Path) -> Result<ClaudeSessionRow> {
    let mut summary = SummaryAccumulator::default();
    let is_root = is_root_transcript(path);

    for line in BufReader::new(File::open(path)?).lines() {
        let value = crate::parse_json_line(&line?)?;
        crate::update_summary_timestamp(&mut summary, &value);

        if summary.session_id.is_none() {
            summary.session_id = crate::json_string(&value, &["sessionId"]);
        }
        if summary.cwd.is_none() {
            summary.cwd = crate::json_string(&value, &["cwd"]);
        }
        if summary.git_branch.is_none() {
            summary.git_branch = crate::json_string(&value, &["gitBranch"]);
        }

        if crate::json_type(&value) == Some("user") && summary.title.is_none() {
            summary.title = extract_title(value.get("message"));
        }
    }

    let summary = crate::build_summary(SourceApp::ClaudeCode, path, summary)?;
    let agent_session_id = if is_root {
        summary.source_session_id.clone()
    } else {
        subagent_session_id_from_path(path).unwrap_or_else(|| summary.source_session_id.clone())
    };
    let agent_label = if is_root {
        None
    } else {
        subagent_label_from_path(path)
    };

    Ok(ClaudeSessionRow {
        path: path.to_path_buf(),
        summary,
        agent_session_id,
        agent_label,
        is_root,
    })
}

fn is_root_transcript(path: &Path) -> bool {
    path.parent()
        .and_then(|parent| parent.file_name())
        .and_then(|name| name.to_str())
        != Some("subagents")
}

fn subagent_session_id_from_path(path: &Path) -> Option<String> {
    path.file_stem()
        .and_then(|stem| stem.to_str())
        .map(|stem| stem.strip_prefix("agent-").unwrap_or(stem).to_string())
}

fn subagent_label_from_path(path: &Path) -> Option<String> {
    let metadata_path = path.with_extension("meta.json");
    let metadata = fs::read_to_string(metadata_path).ok()?;
    let value = serde_json::from_str::<Value>(&metadata).ok()?;
    subagent_label_from_metadata(&value)
}

fn subagent_label_from_metadata(value: &Value) -> Option<String> {
    let metadata = serde_json::from_value::<ClaudeAgentMeta>(value.clone()).ok()?;
    metadata
        .description
        .or(metadata.agent_type)
        .or(metadata.name)
        .map(crate::normalize_title)
        .filter(|label| !label.is_empty())
}

fn family_summary(family: &ClaudeSessionFamily) -> SessionSummary {
    let mut summary = family.root.summary.clone();
    let child_count = family.members.len().saturating_sub(1);

    if child_count > 0 {
        summary.title = format!("{} (+{} subagents)", summary.title, child_count);
    }

    summary.transcript_path = family.root.path.display().to_string();
    summary.created_at = family_created_at(family);
    summary.updated_at = Some(family_updated_at(family));
    summary
}

fn family_created_at(family: &ClaudeSessionFamily) -> Option<i64> {
    family
        .members
        .iter()
        .filter_map(|row| row.summary.created_at)
        .min()
}

fn family_updated_at(family: &ClaudeSessionFamily) -> i64 {
    family
        .members
        .iter()
        .filter_map(|row| row.summary.updated_at)
        .max()
        .unwrap_or_default()
}

fn family_source_paths(family: &ClaudeSessionFamily) -> Vec<String> {
    family
        .members
        .iter()
        .map(|row| row.path.display().to_string())
        .collect()
}

fn family_member_display_name(row: &ClaudeSessionRow) -> String {
    row.agent_label
        .clone()
        .or_else(|| {
            row.path
                .file_stem()
                .and_then(|stem| stem.to_str())
                .map(|stem| stem.strip_prefix("agent-").unwrap_or(stem).to_string())
        })
        .unwrap_or_else(|| row.summary.title.clone())
}

fn family_agents(family: &ClaudeSessionFamily) -> Vec<SessionAgent> {
    family
        .members
        .iter()
        .map(|row| SessionAgent {
            session_id: row.agent_session_id.clone(),
            label: if row.is_root {
                "主 Agent".to_string()
            } else {
                format!("{}(子)", family_member_display_name(row))
            },
            is_root: row.is_root,
        })
        .collect()
}

fn claude_path_timestamp(path: &Path) -> Result<i64> {
    let mut latest = crate::file_modified_timestamp_millis(path)?;

    if let Some(parent) = path.parent() {
        latest = latest.max(crate::file_modified_timestamp_millis(parent)?);
    }

    Ok(latest)
}

fn claude_projects_timestamp() -> Result<i64> {
    let mut latest = 0;

    for path in crate::enumerate_jsonl_files(&root()?.join("projects"))? {
        latest = latest.max(claude_path_timestamp(&path)?);

        if !is_root_transcript(&path) {
            let metadata_path = path.with_extension("meta.json");
            if metadata_path.exists() {
                latest = latest.max(claude_path_timestamp(&metadata_path)?);
            }
        }
    }

    Ok(latest)
}

fn family_timestamp(family: &ClaudeSessionFamily) -> Result<i64> {
    family.members.iter().try_fold(0, |latest, row| {
        let mut row_latest = crate::file_modified_timestamp_millis(&row.path)?;

        if !row.is_root {
            if let Some(parent) = row.path.parent() {
                row_latest = row_latest.max(crate::file_modified_timestamp_millis(parent)?);
            }
        }

        Ok(latest.max(row_latest))
    })
}

fn cached_messages_for_family(family: &ClaudeSessionFamily) -> Result<Vec<SessionMessage>> {
    let cache_key = family.root.summary.source_session_id.clone();
    let updated_at = family_timestamp(family)?;

    if let Some(messages) = lock_timeline_cache()?.get(&cache_key).and_then(|entry| {
        (entry.updated_at == updated_at)
            .then(|| entry.messages.clone())
            .flatten()
    }) {
        return Ok(messages);
    }

    let messages = load_messages_for_family(family)?;
    let mut cache = lock_timeline_cache()?;
    let entry = cache.entry(cache_key).or_default();
    entry.updated_at = updated_at;
    entry.messages = Some(messages.clone());
    Ok(messages)
}

fn cached_events_for_family(family: &ClaudeSessionFamily) -> Result<Vec<SessionEvent>> {
    let cache_key = family.root.summary.source_session_id.clone();
    let updated_at = family_timestamp(family)?;

    if let Some(events) = lock_timeline_cache()?.get(&cache_key).and_then(|entry| {
        (entry.updated_at == updated_at)
            .then(|| entry.events.clone())
            .flatten()
    }) {
        return Ok(events);
    }

    let events = load_events_for_family(family)?;
    let mut cache = lock_timeline_cache()?;
    let entry = cache.entry(cache_key).or_default();
    entry.updated_at = updated_at;
    entry.events = Some(events.clone());
    Ok(events)
}

fn subagent_marker_message(row: &ClaudeSessionRow) -> SessionMessage {
    SessionMessage {
        id: format!("claude-subagent-start-{}", row.agent_session_id),
        role: "assistant".to_string(),
        timestamp: row.summary.created_at,
        blocks: vec![ContentBlock {
            kind: "output_text".to_string(),
            text: Some(format!(
                "Sub-agent session: {}\n{}",
                family_member_display_name(row),
                row.agent_session_id
            )),
            tool_name: None,
            tool_call_id: None,
            payload: Some(json!({
                "type": "subagent_started",
                "session_id": row.agent_session_id,
                "title": family_member_display_name(row),
                "transcript_path": row.path.display().to_string(),
                "is_sidechain": true,
            })),
        }],
        session_id: Some(row.agent_session_id.clone()),
    }
}

fn subagent_marker_event(row: &ClaudeSessionRow) -> SessionEvent {
    SessionEvent {
        id: format!("claude-subagent-event-{}", row.agent_session_id),
        kind: "subagent_started".to_string(),
        timestamp: row.summary.created_at,
        summary: format!(
            "Sub-agent session started: {}",
            family_member_display_name(row)
        ),
        payload: Some(json!({
            "session_id": row.agent_session_id,
            "title": family_member_display_name(row),
            "transcript_path": row.path.display().to_string(),
            "is_sidechain": true,
        })),
        session_id: Some(row.agent_session_id.clone()),
    }
}

fn parse_summary(path: &Path) -> Result<SessionSummary> {
    let family = session_family_for_path(path)?;
    Ok(family_summary(&family))
}

fn parse_overview(path: &Path) -> Result<SessionDetailOverview> {
    let family = session_family_for_path(path)?;
    let summary = family_summary(&family);
    let messages = cached_messages_for_family(&family)?;
    let events = cached_events_for_family(&family)?;

    Ok(SessionDetailOverview {
        summary,
        source_paths: family_source_paths(&family),
        message_count: Some(messages.len()),
        event_count: Some(events.len()),
        agents: family_agents(&family),
    })
}

fn parse_messages_page(path: &Path, offset: usize, limit: usize) -> Result<SessionMessagePage> {
    let family = session_family_for_path(path)?;
    let all_messages = cached_messages_for_family(&family)?;
    let total_count = all_messages.len();
    let start = offset.min(total_count);
    let end = start.saturating_add(limit).min(total_count);
    let messages = all_messages[start..end].to_vec();
    let next_offset = (end < total_count).then_some(end);

    Ok(SessionMessagePage {
        messages,
        offset: start,
        limit,
        next_offset,
        total_count,
        has_more: next_offset.is_some(),
    })
}

fn parse_events_page(path: &Path, offset: usize, limit: usize) -> Result<SessionEventPage> {
    let family = session_family_for_path(path)?;
    let all_events = cached_events_for_family(&family)?;
    let total_count = all_events.len();
    let start = offset.min(total_count);
    let end = start.saturating_add(limit).min(total_count);
    let events = all_events[start..end].to_vec();
    let next_offset = (end < total_count).then_some(end);

    Ok(SessionEventPage {
        events,
        offset: start,
        limit,
        next_offset,
        total_count,
        has_more: next_offset.is_some(),
    })
}

fn parse_detail(path: &Path) -> Result<SessionDetail> {
    let family = session_family_for_path(path)?;
    let summary = family_summary(&family);
    let messages = cached_messages_for_family(&family)?;
    let events = cached_events_for_family(&family)?;

    Ok(SessionDetail {
        summary,
        source_paths: family_source_paths(&family),
        messages,
        events,
    })
}

fn parse_timeline_record(index: usize, value: &Value, session_id: &str) -> Option<TimelineRecord> {
    let timestamp = value
        .get("timestamp")
        .and_then(Value::as_str)
        .and_then(crate::parse_timestamp);

    match crate::json_type(value) {
        Some("user") | Some("assistant") => {
            let message = &value["message"];
            let role = crate::json_string(message, &["role"])
                .or_else(|| crate::json_type(value).map(str::to_string))
                .unwrap_or_else(|| "unknown".to_string());
            let mut blocks = parse_message_blocks(message.get("content"), &role);

            if role == "user" {
                blocks = crate::sanitize_user_blocks(blocks);
            }

            if !blocks.is_empty() {
                return Some(TimelineRecord::Message(SessionMessage {
                    id: crate::json_string(value, &["uuid"])
                        .unwrap_or_else(|| format!("claude-message-{index}")),
                    role,
                    timestamp,
                    blocks,
                    session_id: Some(session_id.to_string()),
                }));
            }
        }
        _ => {
            let kind = crate::json_type(value).unwrap_or("unknown").to_string();
            return Some(TimelineRecord::Event(SessionEvent {
                id: format!("claude-event-{index}"),
                kind: kind.clone(),
                timestamp,
                summary: crate::summarize_event(kind.as_str(), value),
                payload: Some(value.clone()),
                session_id: Some(session_id.to_string()),
            }));
        }
    }

    None
}

fn load_messages(path: &Path, session_id: &str) -> Result<Vec<SessionMessage>> {
    let mut messages = Vec::new();

    for (index, line) in BufReader::new(File::open(path)?).lines().enumerate() {
        let value = crate::parse_json_line(&line?)?;

        if let Some(TimelineRecord::Message(message)) =
            parse_timeline_record(index, &value, session_id)
        {
            messages.push(message);
        }
    }

    Ok(messages)
}

fn load_events(path: &Path, session_id: &str) -> Result<Vec<SessionEvent>> {
    let mut events = Vec::new();

    for (index, line) in BufReader::new(File::open(path)?).lines().enumerate() {
        let value = crate::parse_json_line(&line?)?;

        if let Some(TimelineRecord::Event(event)) = parse_timeline_record(index, &value, session_id)
        {
            events.push(event);
        }
    }

    Ok(events)
}

fn load_messages_for_family(family: &ClaudeSessionFamily) -> Result<Vec<SessionMessage>> {
    let mut messages = Vec::new();

    for row in &family.members {
        if !row.is_root {
            messages.push(subagent_marker_message(row));
        }

        messages.extend(load_messages(&row.path, &row.agent_session_id)?);
    }

    messages.sort_by(|left, right| {
        left.timestamp
            .cmp(&right.timestamp)
            .then_with(|| left.id.cmp(&right.id))
    });

    Ok(messages)
}

fn load_events_for_family(family: &ClaudeSessionFamily) -> Result<Vec<SessionEvent>> {
    let mut events = Vec::new();

    for row in &family.members {
        if !row.is_root {
            events.push(subagent_marker_event(row));
        }

        events.extend(load_events(&row.path, &row.agent_session_id)?);
    }

    events.sort_by(|left, right| {
        left.timestamp
            .cmp(&right.timestamp)
            .then_with(|| left.id.cmp(&right.id))
    });

    Ok(events)
}

fn parse_message_blocks(content: Option<&Value>, role: &str) -> Vec<ContentBlock> {
    let Some(content) = content else {
        return Vec::new();
    };

    match content {
        Value::String(text) => {
            if role == "user" && crate::is_transport_message(text) {
                return Vec::new();
            }

            vec![ContentBlock {
                kind: "text".to_string(),
                text: Some(text.clone()),
                tool_name: None,
                tool_call_id: None,
                payload: None,
            }]
        }
        Value::Array(items) => items
            .iter()
            .filter_map(|item| {
                let kind =
                    crate::json_string(item, &["type"]).unwrap_or_else(|| "unknown".to_string());
                let text = crate::json_string(item, &["text"])
                    .or_else(|| crate::json_string(item, &["thinking"]))
                    .or_else(|| crate::json_string(item, &["content"]));

                Some(ContentBlock {
                    kind,
                    text,
                    tool_name: crate::json_string(item, &["name"]),
                    tool_call_id: crate::json_string(item, &["id"])
                        .or_else(|| crate::json_string(item, &["tool_use_id"])),
                    payload: Some(item.clone()),
                })
            })
            .collect(),
        _ => Vec::new(),
    }
}

fn extract_title(message: Option<&Value>) -> Option<String> {
    let content = message.and_then(|value| value.get("content"))?;

    parse_message_blocks(Some(content), "user")
        .into_iter()
        .filter_map(|block| block.text)
        .find_map(|text| crate::title_candidate_from_text(&text))
}

pub(crate) fn write_session(
    detail: &SessionDetail,
    new_session_id: &str,
) -> Result<(String, Vec<String>)> {
    let cwd = crate::effective_cwd(detail.summary.cwd.as_deref())?;
    let project_slug = project_slug(&cwd);
    let target_path = root()?
        .join("projects")
        .join(project_slug)
        .join(format!("{new_session_id}.jsonl"));
    let git_branch = detail.summary.git_branch.clone();
    let prompt_id = Uuid::new_v4().to_string();
    let mut previous_uuid: Option<String> = None;
    let mut first_record_uuid: Option<String> = None;
    let mut tool_call_sources: HashMap<String, String> = HashMap::new();
    let mut lines = Vec::new();

    for message in &detail.messages {
        if is_subagent_marker_message(message) {
            continue;
        }

        append_records(
            &mut lines,
            message,
            new_session_id,
            &cwd,
            git_branch.as_deref(),
            &prompt_id,
            &mut previous_uuid,
            &mut first_record_uuid,
            &mut tool_call_sources,
        )?;
    }

    if let Some(first_uuid) = first_record_uuid {
        lines.insert(
            0,
            json!({
              "type": "file-history-snapshot",
              "messageId": first_uuid,
              "snapshot": {
                "messageId": first_uuid,
                "trackedFileBackups": {},
                "timestamp": crate::iso_timestamp_utc(Utc::now())
              },
              "isSnapshotUpdate": false
            }),
        );
    }

    crate::write_jsonl_file(&target_path, &lines)?;
    Ok((
        new_session_id.to_string(),
        vec![target_path.display().to_string()],
    ))
}

#[allow(clippy::too_many_arguments)]
fn append_records(
    lines: &mut Vec<Value>,
    message: &SessionMessage,
    session_id: &str,
    cwd: &str,
    git_branch: Option<&str>,
    prompt_id: &str,
    previous_uuid: &mut Option<String>,
    first_record_uuid: &mut Option<String>,
    tool_call_sources: &mut HashMap<String, String>,
) -> Result<()> {
    let timestamp = message
        .timestamp
        .map(crate::utc_timestamp_from_millis)
        .unwrap_or_else(|| crate::iso_timestamp_utc(Utc::now()));
    let assistant_message_id = Uuid::new_v4().simple().to_string();

    for block in &message.blocks {
        match message.role.as_str() {
            "user" => match block.kind.as_str() {
                "text" | "input_text" => {
                    let text = match crate::block_text(block) {
                        Some(text) => text,
                        None => continue,
                    };
                    let record_uuid = Uuid::new_v4().to_string();
                    let record = user_text_record(
                        &record_uuid,
                        previous_uuid.as_deref(),
                        &timestamp,
                        session_id,
                        cwd,
                        git_branch,
                        prompt_id,
                        &text,
                    );
                    push_record(
                        lines,
                        record,
                        &record_uuid,
                        previous_uuid,
                        first_record_uuid,
                    );
                }
                "tool_result" | "function_call_output" => {
                    let record_uuid = Uuid::new_v4().to_string();
                    let call_id = block
                        .tool_call_id
                        .clone()
                        .unwrap_or_else(|| format!("call_{}", Uuid::new_v4().simple()));
                    let tool_text = crate::block_text(block).unwrap_or_default();
                    let source_tool_assistant_uuid = tool_call_sources.get(&call_id).cloned();
                    let record = tool_result_record(
                        &record_uuid,
                        previous_uuid.as_deref(),
                        &timestamp,
                        session_id,
                        cwd,
                        git_branch,
                        prompt_id,
                        &call_id,
                        &tool_text,
                        source_tool_assistant_uuid.as_deref(),
                    );
                    push_record(
                        lines,
                        record,
                        &record_uuid,
                        previous_uuid,
                        first_record_uuid,
                    );
                }
                _ => {}
            },
            "assistant" => match block.kind.as_str() {
                "thinking" | "text" | "output_text" => {
                    let record_uuid = Uuid::new_v4().to_string();
                    let assistant_block = assistant_text_block(block);
                    let record = assistant_record(
                        &record_uuid,
                        previous_uuid.as_deref(),
                        &timestamp,
                        session_id,
                        cwd,
                        git_branch,
                        &assistant_message_id,
                        assistant_block,
                        false,
                    );
                    push_record(
                        lines,
                        record,
                        &record_uuid,
                        previous_uuid,
                        first_record_uuid,
                    );
                }
                "tool_use" | "function_call" => {
                    let record_uuid = Uuid::new_v4().to_string();
                    let call_id = block
                        .tool_call_id
                        .clone()
                        .unwrap_or_else(|| format!("call_{}", Uuid::new_v4().simple()));
                    let assistant_block = tool_use_block(block, &call_id);
                    let record = assistant_record(
                        &record_uuid,
                        previous_uuid.as_deref(),
                        &timestamp,
                        session_id,
                        cwd,
                        git_branch,
                        &assistant_message_id,
                        assistant_block,
                        true,
                    );
                    tool_call_sources.insert(call_id, record_uuid.clone());
                    push_record(
                        lines,
                        record,
                        &record_uuid,
                        previous_uuid,
                        first_record_uuid,
                    );
                }
                _ => {}
            },
            "tool" => match block.kind.as_str() {
                "function_call" | "tool_use" => {
                    let record_uuid = Uuid::new_v4().to_string();
                    let call_id = block
                        .tool_call_id
                        .clone()
                        .unwrap_or_else(|| format!("call_{}", Uuid::new_v4().simple()));
                    let assistant_block = tool_use_block(block, &call_id);
                    let record = assistant_record(
                        &record_uuid,
                        previous_uuid.as_deref(),
                        &timestamp,
                        session_id,
                        cwd,
                        git_branch,
                        &assistant_message_id,
                        assistant_block,
                        true,
                    );
                    tool_call_sources.insert(call_id, record_uuid.clone());
                    push_record(
                        lines,
                        record,
                        &record_uuid,
                        previous_uuid,
                        first_record_uuid,
                    );
                }
                "function_call_output" | "tool_result" => {
                    let record_uuid = Uuid::new_v4().to_string();
                    let call_id = block
                        .tool_call_id
                        .clone()
                        .unwrap_or_else(|| format!("call_{}", Uuid::new_v4().simple()));
                    let tool_text = crate::block_text(block).unwrap_or_default();
                    let source_tool_assistant_uuid = tool_call_sources.get(&call_id).cloned();
                    let record = tool_result_record(
                        &record_uuid,
                        previous_uuid.as_deref(),
                        &timestamp,
                        session_id,
                        cwd,
                        git_branch,
                        prompt_id,
                        &call_id,
                        &tool_text,
                        source_tool_assistant_uuid.as_deref(),
                    );
                    push_record(
                        lines,
                        record,
                        &record_uuid,
                        previous_uuid,
                        first_record_uuid,
                    );
                }
                _ => {}
            },
            _ => {}
        }
    }

    Ok(())
}

fn is_subagent_marker_message(message: &SessionMessage) -> bool {
    if message.role != "assistant" {
        return false;
    }

    message.blocks.iter().any(|block| {
        block
            .payload
            .as_ref()
            .and_then(|payload| payload.get("type"))
            .and_then(Value::as_str)
            == Some("subagent_started")
    })
}

fn push_record(
    lines: &mut Vec<Value>,
    record: Value,
    record_uuid: &str,
    previous_uuid: &mut Option<String>,
    first_record_uuid: &mut Option<String>,
) {
    if first_record_uuid.is_none() {
        *first_record_uuid = Some(record_uuid.to_string());
    }

    lines.push(record);
    *previous_uuid = Some(record_uuid.to_string());
}

fn user_text_record(
    record_uuid: &str,
    parent_uuid: Option<&str>,
    timestamp: &str,
    session_id: &str,
    cwd: &str,
    git_branch: Option<&str>,
    prompt_id: &str,
    text: &str,
) -> Value {
    let mut record = base_record(
        "user",
        record_uuid,
        parent_uuid,
        timestamp,
        session_id,
        cwd,
        git_branch,
    );
    record.insert("promptId".to_string(), Value::String(prompt_id.to_string()));
    record.insert(
        "message".to_string(),
        json!({
          "role": "user",
          "content": text
        }),
    );
    Value::Object(record)
}

fn tool_result_record(
    record_uuid: &str,
    parent_uuid: Option<&str>,
    timestamp: &str,
    session_id: &str,
    cwd: &str,
    git_branch: Option<&str>,
    prompt_id: &str,
    call_id: &str,
    content: &str,
    source_tool_assistant_uuid: Option<&str>,
) -> Value {
    let mut record = base_record(
        "user",
        record_uuid,
        parent_uuid,
        timestamp,
        session_id,
        cwd,
        git_branch,
    );
    record.insert("promptId".to_string(), Value::String(prompt_id.to_string()));
    record.insert(
        "message".to_string(),
        json!({
          "role": "user",
          "content": [
            {
              "tool_use_id": call_id,
              "type": "tool_result",
              "content": content,
              "is_error": false
            }
          ]
        }),
    );
    record.insert(
        "toolUseResult".to_string(),
        json!({
          "type": "text",
          "content": content
        }),
    );

    if let Some(uuid) = source_tool_assistant_uuid {
        record.insert(
            "sourceToolAssistantUUID".to_string(),
            Value::String(uuid.to_string()),
        );
    }

    Value::Object(record)
}

fn assistant_record(
    record_uuid: &str,
    parent_uuid: Option<&str>,
    timestamp: &str,
    session_id: &str,
    cwd: &str,
    git_branch: Option<&str>,
    message_id: &str,
    block: Value,
    tool_use: bool,
) -> Value {
    let mut record = base_record(
        "assistant",
        record_uuid,
        parent_uuid,
        timestamp,
        session_id,
        cwd,
        git_branch,
    );
    record.insert(
        "message".to_string(),
        json!({
          "id": message_id,
          "type": "message",
          "role": "assistant",
          "content": [block],
          "model": "imported",
          "stop_reason": if tool_use { Value::String("tool_use".to_string()) } else { Value::Null },
          "stop_sequence": Value::Null,
          "service_tier": "imported"
        }),
    );
    Value::Object(record)
}

fn base_record(
    record_type: &str,
    record_uuid: &str,
    parent_uuid: Option<&str>,
    timestamp: &str,
    session_id: &str,
    cwd: &str,
    git_branch: Option<&str>,
) -> Map<String, Value> {
    let mut record = Map::new();
    record.insert(
        "parentUuid".to_string(),
        parent_uuid
            .map(|value| Value::String(value.to_string()))
            .unwrap_or(Value::Null),
    );
    record.insert("isSidechain".to_string(), Value::Bool(false));
    record.insert("type".to_string(), Value::String(record_type.to_string()));
    record.insert("uuid".to_string(), Value::String(record_uuid.to_string()));
    record.insert(
        "timestamp".to_string(),
        Value::String(timestamp.to_string()),
    );
    record.insert(
        "userType".to_string(),
        Value::String("external".to_string()),
    );
    record.insert(
        "entrypoint".to_string(),
        Value::String("agent-session-hub".to_string()),
    );
    record.insert("cwd".to_string(), Value::String(cwd.to_string()));
    record.insert(
        "sessionId".to_string(),
        Value::String(session_id.to_string()),
    );
    record.insert(
        "version".to_string(),
        Value::String(crate::IMPORTER_VERSION.to_string()),
    );

    if let Some(branch) = git_branch {
        record.insert("gitBranch".to_string(), Value::String(branch.to_string()));
    }

    record
}

fn assistant_text_block(block: &ContentBlock) -> Value {
    match block.kind.as_str() {
        "thinking" => json!({
          "type": "thinking",
          "thinking": crate::block_text(block).unwrap_or_default()
        }),
        _ => json!({
          "type": "text",
          "text": crate::block_text(block).unwrap_or_default()
        }),
    }
}

fn tool_use_block(block: &ContentBlock, call_id: &str) -> Value {
    json!({
      "type": "tool_use",
      "id": call_id,
      "name": block.tool_name.clone().unwrap_or_else(|| "imported_tool".to_string()),
      "input": tool_input(block)
    })
}

fn tool_input(block: &ContentBlock) -> Value {
    if let Some(payload) = block.payload.as_ref() {
        if let Some(input) = payload.get("input") {
            return input.clone();
        }

        if let Some(arguments) = payload.get("arguments") {
            if arguments.is_object() {
                return arguments.clone();
            }

            if let Some(text) = arguments.as_str() {
                if let Ok(parsed) = serde_json::from_str::<Value>(text) {
                    if parsed.is_object() {
                        return parsed;
                    }
                }

                return json!({ "raw": text });
            }
        }
    }

    if let Some(text) = block.text.as_deref() {
        if let Ok(parsed) = serde_json::from_str::<Value>(text) {
            if parsed.is_object() {
                return parsed;
            }
        }

        return json!({ "raw": text });
    }

    json!({})
}

fn project_slug(cwd: &str) -> String {
    cwd.replace('/', "-")
}
