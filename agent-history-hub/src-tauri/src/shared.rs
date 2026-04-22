use std::fs::{self, File};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::str::FromStr;
use std::time::SystemTime;

use anyhow::{anyhow, Context, Result};
use chrono::{DateTime, SecondsFormat, Utc};
use dirs::home_dir;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use uuid::Uuid;
use walkdir::WalkDir;

pub(crate) const IMPORTER_VERSION: &str = "agent-session-hub/0.1.0";

#[derive(Clone, Copy, Debug, Deserialize, Eq, Hash, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub(crate) enum SourceApp {
    Codex,
    ClaudeCode,
    #[serde(rename = "opencode")]
    OpenCode,
}

impl FromStr for SourceApp {
    type Err = anyhow::Error;

    fn from_str(value: &str) -> Result<Self> {
        match value {
            "codex" => Ok(Self::Codex),
            "claude_code" => Ok(Self::ClaudeCode),
            "opencode" => Ok(Self::OpenCode),
            _ => Err(anyhow!("Unsupported source app: {value}")),
        }
    }
}

impl SourceApp {
    pub(crate) fn as_str(self) -> &'static str {
        match self {
            Self::Codex => "codex",
            Self::ClaudeCode => "claude_code",
            Self::OpenCode => "opencode",
        }
    }
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SourceStatus {
    pub(crate) app: SourceApp,
    pub(crate) available: bool,
    pub(crate) root_path: Option<String>,
    pub(crate) session_count: usize,
    pub(crate) note: Option<String>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SessionSummary {
    pub(crate) source_app: SourceApp,
    pub(crate) source_session_id: String,
    pub(crate) title: String,
    pub(crate) cwd: Option<String>,
    pub(crate) git_branch: Option<String>,
    pub(crate) transcript_path: String,
    pub(crate) created_at: Option<i64>,
    pub(crate) updated_at: Option<i64>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct ContentBlock {
    pub(crate) kind: String,
    pub(crate) text: Option<String>,
    pub(crate) tool_name: Option<String>,
    pub(crate) tool_call_id: Option<String>,
    pub(crate) payload: Option<Value>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SessionMessage {
    pub(crate) id: String,
    pub(crate) role: String,
    pub(crate) timestamp: Option<i64>,
    pub(crate) blocks: Vec<ContentBlock>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) session_id: Option<String>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SessionEvent {
    pub(crate) id: String,
    pub(crate) kind: String,
    pub(crate) timestamp: Option<i64>,
    pub(crate) summary: String,
    pub(crate) payload: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) session_id: Option<String>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SessionAgent {
    pub(crate) session_id: String,
    pub(crate) label: String,
    pub(crate) is_root: bool,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SessionDetail {
    pub(crate) summary: SessionSummary,
    pub(crate) source_paths: Vec<String>,
    pub(crate) messages: Vec<SessionMessage>,
    pub(crate) events: Vec<SessionEvent>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SessionDetailOverview {
    pub(crate) summary: SessionSummary,
    pub(crate) source_paths: Vec<String>,
    pub(crate) message_count: Option<usize>,
    pub(crate) event_count: Option<usize>,
    pub(crate) agents: Vec<SessionAgent>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct ImportPreview {
    pub(crate) source_app: SourceApp,
    pub(crate) source_session_id: String,
    pub(crate) target_app: SourceApp,
    pub(crate) supported: bool,
    pub(crate) import_level: String,
    pub(crate) warnings: Vec<String>,
    pub(crate) created_paths: Vec<String>,
    pub(crate) backup_paths: Vec<String>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct ImportResult {
    pub(crate) target_app: SourceApp,
    pub(crate) created_session_id: String,
    pub(crate) created_paths: Vec<String>,
    pub(crate) backup_paths: Vec<String>,
    pub(crate) resume_cwd: Option<String>,
    pub(crate) warnings: Vec<String>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SessionMessagePage {
    pub(crate) messages: Vec<SessionMessage>,
    pub(crate) offset: usize,
    pub(crate) limit: usize,
    pub(crate) next_offset: Option<usize>,
    pub(crate) total_count: usize,
    pub(crate) has_more: bool,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SessionEventPage {
    pub(crate) events: Vec<SessionEvent>,
    pub(crate) offset: usize,
    pub(crate) limit: usize,
    pub(crate) next_offset: Option<usize>,
    pub(crate) total_count: usize,
    pub(crate) has_more: bool,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SessionPage {
    pub(crate) source_app: SourceApp,
    pub(crate) sessions: Vec<SessionSummary>,
    pub(crate) offset: usize,
    pub(crate) limit: usize,
    pub(crate) next_offset: Option<usize>,
    pub(crate) total_count: usize,
    pub(crate) has_more: bool,
}

#[derive(Clone, Debug)]
pub(crate) struct SessionFileEntry {
    pub(crate) path: PathBuf,
    pub(crate) sort_timestamp: i64,
    pub(crate) summary: Option<SessionSummary>,
}

pub(crate) enum TimelineRecord {
    Message(SessionMessage),
    Event(SessionEvent),
}

#[derive(Default)]
pub(crate) struct SummaryAccumulator {
    pub(crate) session_id: Option<String>,
    pub(crate) title: Option<String>,
    pub(crate) cwd: Option<String>,
    pub(crate) git_branch: Option<String>,
    pub(crate) created_at: Option<i64>,
    pub(crate) updated_at: Option<i64>,
}

pub(crate) fn index_session_file(path: PathBuf) -> Result<SessionFileEntry> {
    let sort_timestamp = fs::metadata(&path)
        .with_context(|| format!("Failed to read metadata for {}", path.display()))?
        .modified()
        .ok()
        .and_then(system_time_to_timestamp_millis)
        .unwrap_or_default();

    Ok(SessionFileEntry {
        path,
        sort_timestamp,
        summary: None,
    })
}

pub(crate) fn build_summary(
    source_app: SourceApp,
    path: &Path,
    summary: SummaryAccumulator,
) -> Result<SessionSummary> {
    let source_session_id = summary
        .session_id
        .or_else(|| derive_session_id(path))
        .ok_or_else(|| anyhow!("Unable to determine session id for {}", path.display()))?;

    let title = summary
        .title
        .filter(|value| !value.is_empty())
        .unwrap_or_else(|| source_session_id.clone());

    Ok(SessionSummary {
        source_app,
        source_session_id,
        title,
        cwd: summary.cwd,
        git_branch: summary.git_branch,
        transcript_path: path.display().to_string(),
        created_at: summary.created_at,
        updated_at: summary.updated_at,
    })
}

pub(crate) fn summarize_event(kind: &str, value: &Value) -> String {
    if let Some(message) = json_string(value, &["message"]) {
        return normalize_title(message);
    }

    if let Some(event_type) = json_string(value, &["type"]) {
        if event_type != kind {
            return format!("{kind}: {}", normalize_title(event_type));
        }
    }

    if let Some(name) = json_string(value, &["name"]) {
        return format!("{kind}: {}", normalize_title(name));
    }

    kind.to_string()
}

pub(crate) fn tool_text(payload: &Value) -> Option<String> {
    if let Some(arguments) = json_string(payload, &["arguments"]) {
        return Some(arguments);
    }

    if let Some(output) = payload.get("output") {
        return stringify_json(output);
    }

    None
}

pub(crate) fn generate_target_session_id(target_app: SourceApp) -> String {
    match target_app {
        SourceApp::OpenCode => format!("ses_{}", Uuid::new_v4().simple()),
        SourceApp::Codex | SourceApp::ClaudeCode => Uuid::new_v4().to_string(),
    }
}

pub(crate) fn effective_cwd(cwd: Option<&str>) -> Result<String> {
    if let Some(cwd) = cwd {
        return Ok(cwd.to_string());
    }

    Ok(home_dir()
        .context("Unable to determine home directory")?
        .display()
        .to_string())
}

pub(crate) fn write_jsonl_file(path: &Path, lines: &[Value]) -> Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .with_context(|| format!("Failed to create {}", parent.display()))?;
    }

    let mut file =
        File::create(path).with_context(|| format!("Failed to create {}", path.display()))?;

    for line in lines {
        serde_json::to_writer(&mut file, line)?;
        writeln!(&mut file)?;
    }

    Ok(())
}

pub(crate) fn update_summary_timestamp(summary: &mut SummaryAccumulator, value: &Value) {
    let Some(timestamp) = value
        .get("timestamp")
        .and_then(Value::as_str)
        .and_then(parse_timestamp)
    else {
        return;
    };

    summary.created_at = match summary.created_at {
        Some(current) => Some(current.min(timestamp)),
        None => Some(timestamp),
    };

    summary.updated_at = match summary.updated_at {
        Some(current) => Some(current.max(timestamp)),
        None => Some(timestamp),
    };
}

pub(crate) fn find_session_file(root: &Path, source_session_id: &str) -> Result<PathBuf> {
    enumerate_jsonl_files(root)?
        .into_iter()
        .find(|path| {
            path.file_name()
                .and_then(|name| name.to_str())
                .is_some_and(|name| name.contains(source_session_id))
        })
        .ok_or_else(|| anyhow!("Could not find session file for {source_session_id}"))
}

pub(crate) fn enumerate_jsonl_files(root: &Path) -> Result<Vec<PathBuf>> {
    if !root.exists() {
        return Ok(Vec::new());
    }

    let mut files = Vec::new();

    for entry in WalkDir::new(root)
        .into_iter()
        .filter_map(|entry| entry.ok())
    {
        if entry.file_type().is_file()
            && entry.path().extension().and_then(|value| value.to_str()) == Some("jsonl")
        {
            files.push(entry.into_path());
        }
    }

    Ok(files)
}

pub(crate) fn count_jsonl_files(root: &Path) -> usize {
    enumerate_jsonl_files(root)
        .map(|files| files.len())
        .unwrap_or_default()
}

pub(crate) fn parse_json_line(line: &str) -> Result<Value> {
    serde_json::from_str(line).with_context(|| format!("Invalid JSONL line: {line}"))
}

pub(crate) fn json_type(value: &Value) -> Option<&str> {
    value.get("type").and_then(Value::as_str)
}

pub(crate) fn json_string(value: &Value, keys: &[&str]) -> Option<String> {
    let mut current = value;

    for key in keys {
        current = current.get(*key)?;
    }

    current.as_str().map(ToString::to_string)
}

pub(crate) fn parse_timestamp(raw: &str) -> Option<i64> {
    DateTime::parse_from_rfc3339(raw)
        .ok()
        .map(|value| value.timestamp_millis())
}

fn system_time_to_timestamp_millis(raw: SystemTime) -> Option<i64> {
    raw.duration_since(SystemTime::UNIX_EPOCH)
        .ok()
        .and_then(|value| i64::try_from(value.as_millis()).ok())
}

pub(crate) fn file_modified_timestamp_millis(path: &Path) -> Result<i64> {
    Ok(fs::metadata(path)
        .with_context(|| format!("Failed to read metadata for {}", path.display()))?
        .modified()
        .ok()
        .and_then(system_time_to_timestamp_millis)
        .unwrap_or_default())
}

pub(crate) fn stringify_json(value: &Value) -> Option<String> {
    serde_json::to_string_pretty(value).ok()
}

pub(crate) fn derive_session_id(path: &Path) -> Option<String> {
    let file_stem = path.file_stem()?.to_str()?;

    if file_stem.len() >= 36 {
        return Some(file_stem[file_stem.len() - 36..].to_string());
    }

    Some(file_stem.to_string())
}

pub(crate) fn block_text(block: &ContentBlock) -> Option<String> {
    block
        .text
        .clone()
        .or_else(|| block.payload.as_ref().and_then(stringify_json))
}

pub(crate) fn iso_timestamp_utc(value: DateTime<Utc>) -> String {
    value.to_rfc3339_opts(SecondsFormat::Millis, true)
}

pub(crate) fn utc_timestamp_from_millis(timestamp: i64) -> String {
    DateTime::<Utc>::from_timestamp_millis(timestamp)
        .map(iso_timestamp_utc)
        .unwrap_or_else(|| iso_timestamp_utc(Utc::now()))
}

pub(crate) fn is_transport_message(text: &str) -> bool {
    let trimmed = text.trim();

    trimmed.starts_with("<environment_context>")
        || trimmed.starts_with("<turn_aborted>")
        || trimmed.starts_with("<permissions instructions>")
        || trimmed.starts_with("<collaboration_mode>")
        || trimmed.starts_with("<skills_instructions>")
        || trimmed.starts_with("<subagent_notification>")
}

pub(crate) fn is_title_noise(text: &str) -> bool {
    let trimmed = text.trim_start();

    if trimmed.is_empty() {
        return false;
    }

    let stripped = trimmed.trim_start_matches('#').trim_start();
    stripped.starts_with("AGENTS.md instructions for")
        || trimmed.starts_with("<system-reminder>")
        || trimmed.starts_with("</system-reminder>")
}

pub(crate) fn title_candidate_from_text(text: &str) -> Option<String> {
    let trimmed = text.trim();

    if trimmed.is_empty() {
        return None;
    }

    if !is_title_noise(trimmed) {
        return Some(normalize_title(trimmed.to_string()));
    }

    let mut inside_reminder = false;

    for line in trimmed.lines().map(str::trim) {
        if line.is_empty() || is_title_noise(line) || is_transport_message(line) {
            inside_reminder |= line.starts_with("<system-reminder>");
            if line.starts_with("</system-reminder>") {
                inside_reminder = false;
            }
            continue;
        }

        if inside_reminder {
            if line.starts_with("</system-reminder>") {
                inside_reminder = false;
            }
            continue;
        }

        if line.starts_with('<') && line.ends_with('>') {
            continue;
        }

        return Some(normalize_title(line.to_string()));
    }

    None
}

pub(crate) fn sanitize_user_message_text(text: &str) -> Option<String> {
    let mut lines = Vec::new();
    let mut skip_until: Option<&str> = None;

    for raw_line in text.lines() {
        let line = raw_line.trim();

        if let Some(end_tag) = skip_until {
            if line.starts_with(end_tag) {
                skip_until = None;
            }
            continue;
        }

        if line.is_empty() {
            lines.push(String::new());
            continue;
        }

        if is_title_noise(line) || is_transport_message(line) {
            continue;
        }

        if line.starts_with("<INSTRUCTIONS>") {
            skip_until = Some("</INSTRUCTIONS>");
            continue;
        }

        if line.starts_with("<system-reminder>") {
            skip_until = Some("</system-reminder>");
            continue;
        }

        if line.starts_with("<environment_context>") {
            skip_until = Some("</environment_context>");
            continue;
        }

        if line.starts_with("<subagent_notification>") {
            skip_until = Some("</subagent_notification>");
            continue;
        }

        if line.starts_with('<') && line.ends_with('>') {
            continue;
        }

        lines.push(raw_line.trim_end().to_string());
    }

    let sanitized = lines.join("\n");
    let trimmed = sanitized.trim();
    (!trimmed.is_empty()).then(|| trimmed.to_string())
}

pub(crate) fn sanitize_user_blocks(blocks: Vec<ContentBlock>) -> Vec<ContentBlock> {
    blocks
        .into_iter()
        .filter_map(|mut block| {
            block.text = block
                .text
                .take()
                .and_then(|text| sanitize_user_message_text(&text));

            (block.text.is_some()
                || matches!(
                    block.kind.as_str(),
                    "tool_use" | "tool_result" | "function_call" | "function_call_output"
                ))
            .then_some(block)
        })
        .collect()
}

pub(crate) fn normalize_title(text: String) -> String {
    let normalized = text.replace('\n', " ").trim().to_string();

    if normalized.chars().count() <= 72 {
        return normalized;
    }

    normalized.chars().take(72).collect::<String>() + "..."
}
