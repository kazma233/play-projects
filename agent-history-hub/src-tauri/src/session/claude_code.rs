use std::collections::HashMap;
use std::fs::File;
use std::io::{BufRead, BufReader};
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use chrono::Utc;
use dirs::home_dir;
use serde_json::{json, Map, Value};
use uuid::Uuid;

use crate::{
    ContentBlock, SessionAgent, SessionDetail, SessionDetailOverview, SessionEvent, SessionEventPage,
    SessionFileEntry, SessionMessage, SessionMessagePage, SessionSummary, SourceApp,
    SummaryAccumulator, TimelineRecord,
};

use super::{SessionExporter, SessionReader};

pub(crate) struct ClaudeCodeBackend;

pub(crate) static BACKEND: ClaudeCodeBackend = ClaudeCodeBackend;

impl SessionReader for ClaudeCodeBackend {
    fn list_entries(&self) -> Result<Vec<SessionFileEntry>> {
        let root = root()?.join("projects");
        let mut entries = crate::enumerate_jsonl_files(&root)?
            .into_iter()
            .map(crate::index_session_file)
            .collect::<Result<Vec<_>>>()?;

        super::sort_entries(&mut entries);
        Ok(entries)
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

fn parse_summary(path: &Path) -> Result<SessionSummary> {
    let mut summary = SummaryAccumulator::default();

    for line in BufReader::new(File::open(path)?).lines() {
        let value = crate::parse_json_line(&line?)?;
        crate::update_summary_timestamp(&mut summary, &value);

        match crate::json_type(&value) {
            Some("user") | Some("assistant") => {
                summary.session_id = summary
                    .session_id
                    .or_else(|| crate::json_string(&value, &["sessionId"]));
                summary.cwd = summary.cwd.or_else(|| crate::json_string(&value, &["cwd"]));
                summary.git_branch = summary
                    .git_branch
                    .or_else(|| crate::json_string(&value, &["gitBranch"]));

                if crate::json_type(&value) == Some("user") && summary.title.is_none() {
                    summary.title = extract_title(value.get("message"));
                }
            }
            _ => {}
        }
    }

    crate::build_summary(SourceApp::ClaudeCode, path, summary)
}

fn parse_overview(path: &Path) -> Result<SessionDetailOverview> {
    let summary = parse_summary(path)?;
    let root_session_id = summary.source_session_id.clone();
    let mut message_count = 0;
    let mut event_count = 0;

    for (index, line) in BufReader::new(File::open(path)?).lines().enumerate() {
        let value = crate::parse_json_line(&line?)?;

        match parse_timeline_record(index, &value) {
            Some(TimelineRecord::Message(_)) => message_count += 1,
            Some(TimelineRecord::Event(_)) => event_count += 1,
            None => {}
        }
    }

    Ok(SessionDetailOverview {
        summary,
        source_paths: vec![path.display().to_string()],
        message_count: Some(message_count),
        event_count: Some(event_count),
        agents: vec![SessionAgent {
            session_id: root_session_id,
            label: "主 Agent".to_string(),
            is_root: true,
        }],
    })
}

fn parse_messages_page(path: &Path, offset: usize, limit: usize) -> Result<SessionMessagePage> {
    let mut messages = Vec::new();
    let mut total_count = 0;

    for (index, line) in BufReader::new(File::open(path)?).lines().enumerate() {
        let value = crate::parse_json_line(&line?)?;

        if let Some(TimelineRecord::Message(message)) = parse_timeline_record(index, &value) {
            if total_count >= offset && messages.len() < limit {
                messages.push(message);
            }

            total_count += 1;
        }
    }

    let next_offset = (offset + messages.len() < total_count).then_some(offset + messages.len());

    Ok(SessionMessagePage {
        messages,
        offset,
        limit,
        next_offset,
        total_count,
        has_more: next_offset.is_some(),
    })
}

fn parse_events_page(path: &Path, offset: usize, limit: usize) -> Result<SessionEventPage> {
    let mut events = Vec::new();
    let mut total_count = 0;

    for (index, line) in BufReader::new(File::open(path)?).lines().enumerate() {
        let value = crate::parse_json_line(&line?)?;

        if let Some(TimelineRecord::Event(event)) = parse_timeline_record(index, &value) {
            if total_count >= offset && events.len() < limit {
                events.push(event);
            }

            total_count += 1;
        }
    }

    let next_offset = (offset + events.len() < total_count).then_some(offset + events.len());

    Ok(SessionEventPage {
        events,
        offset,
        limit,
        next_offset,
        total_count,
        has_more: next_offset.is_some(),
    })
}

fn parse_detail(path: &Path) -> Result<SessionDetail> {
    let summary = parse_summary(path)?;
    let mut messages = Vec::new();
    let mut events = Vec::new();

    for (index, line) in BufReader::new(File::open(path)?).lines().enumerate() {
        let value = crate::parse_json_line(&line?)?;
        match parse_timeline_record(index, &value) {
            Some(TimelineRecord::Message(message)) => messages.push(message),
            Some(TimelineRecord::Event(event)) => events.push(event),
            None => {}
        }
    }

    Ok(SessionDetail {
        summary,
        source_paths: vec![path.display().to_string()],
        messages,
        events,
    })
}

fn parse_timeline_record(index: usize, value: &Value) -> Option<TimelineRecord> {
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
                    session_id: crate::json_string(value, &["sessionId"]),
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
                session_id: crate::json_string(value, &["sessionId"]),
            }));
        }
    }

    None
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
