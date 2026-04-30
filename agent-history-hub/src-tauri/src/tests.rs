use std::collections::HashSet;
use std::env;
use std::fs::{self, File};
use std::io::{BufRead, BufReader};
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::{Mutex, MutexGuard, OnceLock};

use anyhow::{anyhow, bail, Context, Result};
use rusqlite::{params, Connection};
use serde_json::{json, Value};
use std::io::Write;
use uuid::Uuid;

use crate::*;

struct HomeGuard {
    original_home: Option<String>,
    _guard: MutexGuard<'static, ()>,
}

impl HomeGuard {
    fn set(temp_home: &Path) -> Self {
        let guard = test_env_lock().lock().expect("test env lock poisoned");
        let original_home = env::var("HOME").ok();
        env::set_var("HOME", temp_home);
        Self {
            original_home,
            _guard: guard,
        }
    }
}

fn test_env_lock() -> &'static Mutex<()> {
    static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
    LOCK.get_or_init(|| Mutex::new(()))
}

impl Drop for HomeGuard {
    fn drop(&mut self) {
        match &self.original_home {
            Some(home) => env::set_var("HOME", home),
            None => env::remove_var("HOME"),
        }
    }
}

#[test]
fn cross_import_roundtrip_works_in_temp_home() -> Result<()> {
    let temp_home = env::temp_dir().join(format!("agent-session-hub-test-{}", Uuid::new_v4()));
    fs::create_dir_all(&temp_home)?;
    let _guard = HomeGuard::set(&temp_home);

    let codex_source_id = "11111111-1111-4111-8111-111111111111";
    let claude_source_id = "22222222-2222-4222-8222-222222222222";

    let _ = session::codex::write_session(&sample_detail(SourceApp::Codex), codex_source_id)?;
    let codex_to_claude = app::import_session_inner(
        SourceApp::Codex,
        codex_source_id,
        SourceApp::ClaudeCode,
        None,
    )?;
    let imported_claude = app::get_session_inner(
        SourceApp::ClaudeCode,
        &codex_to_claude.created_session_id,
        None,
    )?;

    assert!(!imported_claude.messages.is_empty());
    assert_eq!(imported_claude.summary.source_app, SourceApp::ClaudeCode);
    assert_eq!(
        codex_to_claude.resume_cwd.as_deref(),
        Some("/tmp/agent-session-hub/workspace")
    );

    let _ = session::claude_code::write_session(
        &sample_detail(SourceApp::ClaudeCode),
        claude_source_id,
    )?;
    let claude_to_codex = app::import_session_inner(
        SourceApp::ClaudeCode,
        claude_source_id,
        SourceApp::Codex,
        None,
    )?;
    let imported_codex =
        app::get_session_inner(SourceApp::Codex, &claude_to_codex.created_session_id, None)?;

    seed_opencode_session(&sample_detail(SourceApp::OpenCode), codex_source_id)?;

    let opencode_transcript_path = temp_home
        .join(".local/share/opencode/session")
        .join(format!("{codex_source_id}.opencode"));

    assert!(!imported_codex.messages.is_empty());
    assert_eq!(imported_codex.summary.source_app, SourceApp::Codex);
    assert_eq!(
        claude_to_codex.resume_cwd.as_deref(),
        Some("/tmp/agent-session-hub/workspace")
    );
    assert!(codex_timestamps_are_monotonic(
        &claude_to_codex.created_session_id
    )?);
    assert!(codex_has_event_type(
        &claude_to_codex.created_session_id,
        "user_message"
    )?);
    assert!(codex_has_event_type(
        &claude_to_codex.created_session_id,
        "agent_message"
    )?);
    assert!(codex_has_event_type(
        &claude_to_codex.created_session_id,
        "task_complete"
    )?);

    let opencode_to_codex = app::import_session_inner(
        SourceApp::OpenCode,
        codex_source_id,
        SourceApp::Codex,
        Some(opencode_transcript_path.to_string_lossy().as_ref()),
    )?;
    let imported_from_opencode_codex = app::get_session_inner(
        SourceApp::Codex,
        &opencode_to_codex.created_session_id,
        None,
    )?;

    assert!(!imported_from_opencode_codex.messages.is_empty());
    assert_eq!(
        imported_from_opencode_codex.summary.source_app,
        SourceApp::Codex
    );
    assert_eq!(
        opencode_to_codex.resume_cwd.as_deref(),
        Some("/tmp/agent-session-hub/workspace")
    );
    assert!(codex_timestamps_are_monotonic(
        &opencode_to_codex.created_session_id
    )?);
    assert!(codex_has_event_type(
        &opencode_to_codex.created_session_id,
        "user_message"
    )?);
    assert!(codex_has_event_type(
        &opencode_to_codex.created_session_id,
        "agent_message"
    )?);
    assert!(codex_has_event_type(
        &opencode_to_codex.created_session_id,
        "task_complete"
    )?);
    assert!(has_codex_function_call_pair(
        &opencode_to_codex.created_session_id,
        "call_import_1"
    )?);

    let opencode_to_claude = app::import_session_inner(
        SourceApp::OpenCode,
        codex_source_id,
        SourceApp::ClaudeCode,
        Some(opencode_transcript_path.to_string_lossy().as_ref()),
    )?;
    let imported_from_opencode_claude = app::get_session_inner(
        SourceApp::ClaudeCode,
        &opencode_to_claude.created_session_id,
        None,
    )?;

    assert!(!imported_from_opencode_claude.messages.is_empty());
    assert_eq!(
        imported_from_opencode_claude.summary.source_app,
        SourceApp::ClaudeCode
    );

    fs::remove_dir_all(&temp_home).ok();
    Ok(())
}

#[test]
fn opencode_root_session_aggregates_subagent_sessions() -> Result<()> {
    let temp_home = env::temp_dir().join(format!("agent-session-hub-test-{}", Uuid::new_v4()));
    fs::create_dir_all(&temp_home)?;
    let _guard = HomeGuard::set(&temp_home);

    let root_id = "ses_root_session";
    let child_id = "ses_child_session";
    seed_opencode_family(root_id, child_id)?;

    let root_path = temp_home
        .join(".local/share/opencode/session")
        .join(format!("{root_id}.opencode"));

    let summary = session::reader(SourceApp::OpenCode).parse_summary(&root_path)?;
    assert_eq!(summary.source_session_id, root_id);
    assert!(summary.title.contains("+1 subagents"));

    let detail = app::get_session_inner(
        SourceApp::OpenCode,
        root_id,
        Some(root_path.to_string_lossy().as_ref()),
    )?;

    assert!(detail
        .messages
        .iter()
        .any(|message| message.blocks.iter().any(|block| {
            block
                .text
                .as_deref()
                .is_some_and(|text| text.contains("Sub-agent session: Child task"))
        })));
    assert!(detail
        .events
        .iter()
        .any(|event| event.kind == "subagent_started"));
    assert!(detail
        .source_paths
        .iter()
        .any(|path| path.ends_with(&format!("{child_id}.opencode"))));

    fs::remove_dir_all(&temp_home).ok();
    Ok(())
}

#[test]
fn codex_ignores_agents_banner_when_picking_title() -> Result<()> {
    let temp_home = env::temp_dir().join(format!("agent-session-hub-test-{}", Uuid::new_v4()));
    fs::create_dir_all(&temp_home)?;
    let _guard = HomeGuard::set(&temp_home);

    let session_id = "33333333-3333-4333-8333-333333333333";
    let transcript_path = temp_home
        .join(".codex/sessions/2026/04/21")
        .join(format!("rollout-2026-04-21T12-00-00-{session_id}.jsonl"));
    if let Some(parent) = transcript_path.parent() {
        fs::create_dir_all(parent)?;
    }

    let transcript = vec![
        json!({
            "timestamp": "2026-04-21T12:00:00.000Z",
            "type": "session_meta",
            "payload": {
                "id": session_id,
                "cwd": "/tmp/agent-session-hub/workspace"
            }
        }),
        json!({
            "timestamp": "2026-04-21T12:00:01.000Z",
            "type": "response_item",
            "payload": {
                "type": "message",
                "role": "user",
                "content": [
                    {
                        "type": "text",
                        "text": "# AGENTS.md instructions for /tmp/agent-session-hub/workspace\n<system-reminder>\nYour operational mode has changed from plan to build.\nYou are no longer in read-only mode.\nYou are permitted to make file changes, run shell commands, and utilize your arsenal of tools as needed.\n</system-reminder>"
                    }
                ]
            }
        }),
        json!({
            "timestamp": "2026-04-21T12:00:02.000Z",
            "type": "response_item",
            "payload": {
                "type": "message",
                "role": "user",
                "content": [
                    {
                        "type": "text",
                        "text": "How do I list the current files?"
                    }
                ]
            }
        }),
    ];

    write_jsonl(&transcript_path, &transcript)?;

    let summary = session::reader(SourceApp::Codex).parse_summary(&transcript_path)?;

    assert_eq!(summary.source_session_id, session_id);
    assert_eq!(summary.title, "How do I list the current files?");

    fs::remove_dir_all(&temp_home).ok();
    Ok(())
}

#[test]
fn claude_ignores_agents_banner_when_picking_title() -> Result<()> {
    let temp_home = env::temp_dir().join(format!("agent-session-hub-test-{}", Uuid::new_v4()));
    fs::create_dir_all(&temp_home)?;
    let _guard = HomeGuard::set(&temp_home);

    let session_file = temp_home
        .join(".claude/projects/demo-project")
        .join("session-123.jsonl");
    let transcript = vec![
        json!({
            "type": "user",
            "message": {
                "content": "# AGENTS.md instructions for /tmp/agent-session-hub/workspace\n<system-reminder>\nYour operational mode has changed from plan to build.\nYou are no longer in read-only mode.\nYou are permitted to make file changes, run shell commands, and utilize your arsenal of tools as needed.\n</system-reminder>"
            }
        }),
        json!({
            "type": "user",
            "message": {
                "content": "How do I list the current files?"
            }
        }),
    ];

    write_jsonl(&session_file, &transcript)?;

    let summary = session::reader(SourceApp::ClaudeCode).parse_summary(&session_file)?;

    assert_eq!(summary.title, "How do I list the current files?");

    fs::remove_dir_all(&temp_home).ok();
    Ok(())
}

#[test]
fn claude_skips_unreadable_files_when_indexing() -> Result<()> {
    let temp_home = env::temp_dir().join(format!("agent-session-hub-test-{}", Uuid::new_v4()));
    fs::create_dir_all(&temp_home)?;
    let _guard = HomeGuard::set(&temp_home);

    let valid_path = temp_home
        .join(".claude/projects/demo-project")
        .join("session-123.jsonl");
    let invalid_path = temp_home
        .join(".claude/projects/demo-project")
        .join("broken.jsonl");

    write_jsonl(
        &valid_path,
        &[json!({
            "type": "user",
            "message": {
                "content": "Valid task"
            }
        })],
    )?;

    fs::write(&invalid_path, "{ not valid json }\n")?;

    let entries = session::reader(SourceApp::ClaudeCode).list_entries()?;
    assert_eq!(entries.len(), 1);
    assert_eq!(entries[0].path, valid_path);

    fs::remove_dir_all(&temp_home).ok();
    Ok(())
}

#[test]
fn claude_root_session_aggregates_subagent_sessions() -> Result<()> {
    let temp_home = env::temp_dir().join(format!("agent-session-hub-test-{}", Uuid::new_v4()));
    fs::create_dir_all(&temp_home)?;
    let _guard = HomeGuard::set(&temp_home);

    let root_id = "root-session";
    let child_id = "child-session";
    let project_dir = temp_home.join(".claude/projects/demo-project");
    let root_path = project_dir.join(format!("{root_id}.jsonl"));
    let child_path = project_dir
        .join("subagents")
        .join(format!("agent-{child_id}.jsonl"));
    let child_meta_path = project_dir
        .join("subagents")
        .join(format!("agent-{child_id}.meta.json"));

    write_jsonl(
        &root_path,
        &[
            json!({
                "timestamp": "2026-04-21T12:00:00.000Z",
                "sessionId": root_id,
                "type": "user",
                "message": {
                    "content": "Main task"
                }
            }),
            json!({
                "timestamp": "2026-04-21T12:00:01.000Z",
                "sessionId": root_id,
                "type": "assistant",
                "message": {
                    "content": "Root answer"
                }
            }),
        ],
    )?;

    write_jsonl(
        &child_path,
        &[
            json!({
                "timestamp": "2026-04-21T15:00:00.000Z",
                "sessionId": root_id,
                "type": "user",
                "message": {
                    "content": "Child task"
                }
            }),
            json!({
                "timestamp": "2026-04-21T15:00:01.000Z",
                "sessionId": root_id,
                "type": "assistant",
                "message": {
                    "content": "Child answer"
                }
            }),
        ],
    )?;

    fs::write(
        &child_meta_path,
        r#"{"agentType":"Explore","description":"Find fetch_rss scheduling code"}"#,
    )?;

    let reader = session::reader(SourceApp::ClaudeCode);
    let entries = reader.list_entries()?;
    assert_eq!(entries.len(), 1);
    assert_eq!(entries[0].path, root_path);
    assert!(entries[0]
        .summary
        .as_ref()
        .is_some_and(|summary| summary.title.contains("+1 subagents")));

    let overview = reader.parse_overview(&root_path)?;
    assert_eq!(overview.summary.source_session_id, root_id);
    assert_eq!(overview.agents.len(), 2);
    assert_eq!(
        overview
            .agents
            .iter()
            .map(|agent| agent.session_id.as_str())
            .collect::<HashSet<_>>()
            .len(),
        2
    );
    assert!(overview
        .agents
        .iter()
        .any(|agent| agent.is_root && agent.session_id == root_id && agent.label == "主 Agent"));
    assert!(overview.agents.iter().any(|agent| {
        !agent.is_root
            && agent.session_id == child_id
            && agent.label == "Find fetch_rss scheduling code(子)"
    }));

    let detail = reader.parse_detail(&root_path)?;
    let root_path_string = root_path.display().to_string();
    let child_path_string = child_path.display().to_string();
    assert_eq!(detail.source_paths.len(), 2);
    assert!(detail
        .source_paths
        .iter()
        .any(|path| path == &root_path_string));
    assert!(detail
        .source_paths
        .iter()
        .any(|path| path == &child_path_string));
    assert!(detail.messages.iter().any(|message| {
        message.blocks.iter().any(|block| {
            block.text.as_deref().is_some_and(|text| {
                text.contains("Sub-agent session: Find fetch_rss scheduling code")
            })
        })
    }));
    assert!(detail
        .events
        .iter()
        .any(|event| event.kind == "subagent_started"));

    let (_, exported_paths) = session::claude_code::write_session(&detail, "exported-session")?;
    let exported = fs::read_to_string(&exported_paths[0])?;
    assert!(!exported.contains("Sub-agent session:"));
    assert!(!exported.contains("\"subagent_started\""));

    fs::remove_dir_all(&temp_home).ok();
    Ok(())
}

#[test]
fn codex_root_session_aggregates_subagent_sessions() -> Result<()> {
    let temp_home = env::temp_dir().join(format!("agent-session-hub-test-{}", Uuid::new_v4()));
    fs::create_dir_all(&temp_home)?;
    let _guard = HomeGuard::set(&temp_home);

    let root_id = "44444444-4444-4444-8444-444444444444";
    let child_id = "55555555-5555-4555-8555-555555555555";
    let root_path = temp_home
        .join(".codex/sessions/2026/04/21")
        .join(format!("rollout-2026-04-21T12-00-00-{root_id}.jsonl"));
    let child_path = temp_home
        .join(".codex/sessions/2026/04/21")
        .join(format!("rollout-2026-04-21T15-00-00-{child_id}.jsonl"));

    write_jsonl(
        &root_path,
        &[
            json!({
                "timestamp": "2026-04-21T12:00:00.000Z",
                "type": "session_meta",
                "payload": {
                    "id": root_id,
                    "cwd": "/tmp/agent-session-hub/workspace"
                }
            }),
            json!({
                "timestamp": "2026-04-21T12:00:01.000Z",
                "type": "response_item",
                "payload": {
                    "type": "message",
                    "role": "user",
                    "content": [{ "type": "input_text", "text": "Main task" }]
                }
            }),
            json!({
                "timestamp": "2026-04-21T12:00:02.000Z",
                "type": "response_item",
                "payload": {
                    "type": "message",
                    "role": "assistant",
                    "content": [{ "type": "output_text", "text": "Main answer" }]
                }
            }),
        ],
    )?;

    write_jsonl(
        &child_path,
        &[
            json!({
                "timestamp": "2026-04-21T15:00:00.000Z",
                "type": "session_meta",
                "payload": {
                    "id": child_id,
                    "cwd": "/tmp/agent-session-hub/workspace",
                    "source": {
                        "subagent": {
                            "thread_spawn": {
                                "parent_thread_id": root_id,
                                "agent_nickname": "Plato",
                                "agent_role": "worker"
                            }
                        }
                    },
                    "agent_nickname": "Plato",
                    "agent_role": "worker"
                }
            }),
            json!({
                "timestamp": "2026-04-21T15:00:01.000Z",
                "type": "response_item",
                "payload": {
                    "type": "message",
                    "role": "user",
                    "content": [{ "type": "input_text", "text": "Child task" }]
                }
            }),
            json!({
                "timestamp": "2026-04-21T15:00:02.000Z",
                "type": "response_item",
                "payload": {
                    "type": "message",
                    "role": "assistant",
                    "content": [{ "type": "output_text", "text": "Child answer" }]
                }
            }),
        ],
    )?;

    let summary = session::reader(SourceApp::Codex).parse_summary(&root_path)?;
    assert_eq!(summary.source_session_id, root_id);
    assert!(summary.title.contains("+1 subagents"));

    let detail = app::get_session_inner(
        SourceApp::Codex,
        root_id,
        Some(root_path.to_string_lossy().as_ref()),
    )?;

    assert!(detail
        .messages
        .iter()
        .any(|message| message.blocks.iter().any(|block| {
            block
                .text
                .as_deref()
                .is_some_and(|text| text.contains("Sub-agent session: Plato"))
        })));
    assert!(detail
        .events
        .iter()
        .any(|event| event.kind == "subagent_started"));
    assert!(detail
        .source_paths
        .iter()
        .any(|path| path.ends_with(&format!("{child_id}.jsonl"))));

    fs::remove_dir_all(&temp_home).ok();
    Ok(())
}

fn write_jsonl(path: &Path, lines: &[Value]) -> Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }

    let mut file = File::create(path)?;
    for line in lines {
        serde_json::to_writer(&mut file, line)?;
        writeln!(&mut file)?;
    }

    Ok(())
}

#[test]
#[ignore = "Requires CODEX_TRANSCRIPT_PATH and the OpenCode CLI"]
fn imports_codex_transcript_into_opencode_cli_in_temp_home() -> Result<()> {
    let transcript_path = env::var("CODEX_TRANSCRIPT_PATH")
        .context("CODEX_TRANSCRIPT_PATH must point to a Codex transcript JSONL file")?;
    let transcript_path = PathBuf::from(transcript_path);
    let source_session_id = derive_session_id(&transcript_path).ok_or_else(|| {
        anyhow!(
            "Unable to derive session id from {}",
            transcript_path.display()
        )
    })?;
    let temp_home = env::temp_dir().join(format!(
        "agent-session-hub-opencode-import-test-{}",
        Uuid::new_v4()
    ));

    fs::create_dir_all(&temp_home)?;
    let _guard = HomeGuard::set(&temp_home);

    let import_result = app::import_session_inner(
        SourceApp::Codex,
        &source_session_id,
        SourceApp::OpenCode,
        Some(transcript_path.to_string_lossy().as_ref()),
    )?;
    let imported_detail =
        app::get_session_inner(SourceApp::OpenCode, &import_result.created_session_id, None)?;

    assert_eq!(imported_detail.summary.source_app, SourceApp::OpenCode);
    assert!(!imported_detail.messages.is_empty());

    fs::remove_dir_all(&temp_home).ok();
    Ok(())
}

#[test]
#[ignore = "Requires OPENCODE_SESSION_ID and a working Codex CLI login"]
fn imports_real_opencode_session_into_codex_and_resumes() -> Result<()> {
    let source_session_id = env::var("OPENCODE_SESSION_ID")
        .context("OPENCODE_SESSION_ID must be the OpenCode session ID (e.g. ses_xxx)")?;

    let session_file = format!("/tmp/{source_session_id}.opencode");

    let source_detail =
        app::get_session_inner(SourceApp::OpenCode, &source_session_id, Some(&session_file))?;
    assert!(!source_detail.messages.is_empty());

    let import_result = app::import_session_inner(
        SourceApp::OpenCode,
        &source_session_id,
        SourceApp::Codex,
        Some(&session_file),
    )?;
    let imported_detail =
        app::get_session_inner(SourceApp::Codex, &import_result.created_session_id, None)?;
    assert!(!imported_detail.messages.is_empty());

    let cwd = imported_detail
        .summary
        .cwd
        .clone()
        .unwrap_or(env::current_dir()?.display().to_string());
    let last_message_path = env::temp_dir().join(format!(
        "agent-session-hub-codex-resume-{}.txt",
        Uuid::new_v4()
    ));
    let output = Command::new("codex")
        .arg("exec")
        .arg("resume")
        .arg("--skip-git-repo-check")
        .arg("--dangerously-bypass-approvals-and-sandbox")
        .arg("-o")
        .arg(&last_message_path)
        .arg(&import_result.created_session_id)
        .arg("Reply with exactly VERIFIED")
        .current_dir(&cwd)
        .output()
        .context("Failed to execute codex exec resume")?;

    if !output.status.success() {
        let stdout = String::from_utf8_lossy(&output.stdout);
        let stderr = String::from_utf8_lossy(&output.stderr);
        bail!(
            "codex exec resume failed\nstdout:\n{}\nstderr:\n{}",
            stdout.trim(),
            stderr.trim()
        );
    }

    let last_message = fs::read_to_string(&last_message_path)
        .with_context(|| format!("Failed to read {}", last_message_path.display()))?;
    fs::remove_file(&last_message_path).ok();
    assert!(last_message.contains("VERIFIED"));
    assert!(codex_thread_exists(&import_result.created_session_id)?);

    Ok(())
}

fn seed_opencode_session(detail: &SessionDetail, session_id: &str) -> Result<()> {
    let db_path = session::opencode::db_path()?;

    if let Some(parent) = db_path.parent() {
        fs::create_dir_all(parent)?;
    }

    let connection = Connection::open(&db_path)?;
    connection.execute_batch(
        "
        CREATE TABLE session (
            id TEXT PRIMARY KEY,
            project_id TEXT NOT NULL,
            parent_id TEXT,
            slug TEXT NOT NULL,
            directory TEXT NOT NULL,
            title TEXT NOT NULL,
            version TEXT NOT NULL,
            share_url TEXT,
            summary_additions INTEGER,
            summary_deletions INTEGER,
            summary_files INTEGER,
            summary_diffs TEXT,
            revert TEXT,
            permission TEXT,
            time_created INTEGER NOT NULL,
            time_updated INTEGER NOT NULL,
            time_compacting INTEGER,
            time_archived INTEGER,
            workspace_id TEXT
        );
        CREATE TABLE message (
            id TEXT PRIMARY KEY,
            session_id TEXT NOT NULL,
            time_created INTEGER NOT NULL,
            time_updated INTEGER NOT NULL,
            data TEXT NOT NULL
        );
        CREATE TABLE part (
            id TEXT PRIMARY KEY,
            message_id TEXT NOT NULL,
            session_id TEXT NOT NULL,
            time_created INTEGER NOT NULL,
            time_updated INTEGER NOT NULL,
            data TEXT NOT NULL
        );
        ",
    )?;

    let created_at = detail.summary.created_at.unwrap_or(0);
    let updated_at = detail.summary.updated_at.unwrap_or(created_at);
    let cwd = detail
        .summary
        .cwd
        .clone()
        .unwrap_or_else(|| "/tmp".to_string());

    connection.execute(
        "INSERT INTO session (id, project_id, slug, directory, title, version, time_created, time_updated) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8)",
        params![
            session_id,
            "project-1",
            session_id,
            cwd,
            detail.summary.title,
            "1",
            created_at,
            updated_at
        ],
    )?;

    for (index, message) in detail.messages.iter().enumerate() {
        let message_time = message.timestamp.unwrap_or(created_at + index as i64);
        let message_data = json!({
            "role": message.role,
            "time": { "created": message_time }
        });

        connection.execute(
            "INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES (?1, ?2, ?3, ?4, ?5)",
            params![
                message.id,
                session_id,
                message_time,
                message_time,
                serde_json::to_string(&message_data)?
            ],
        )?;

        for (block_index, block) in message.blocks.iter().enumerate() {
            let part_id = format!("part-{index}-{block_index}");
            let part_data = match block.kind.as_str() {
                "thinking" => json!({ "type": "reasoning", "text": block.text }),
                "tool_use" => json!({
                    "type": "tool",
                    "tool": block.tool_name,
                    "callID": block.tool_call_id,
                    "state": {
                        "status": "completed",
                        "input": block.payload.as_ref().and_then(|payload| payload.get("input")).cloned().unwrap_or_else(|| json!({}))
                    }
                }),
                "function_call_output" | "tool_result" => json!({
                    "type": "tool",
                    "tool": block.tool_name,
                    "callID": block.tool_call_id,
                    "state": {
                        "status": "completed",
                        "output": block.text.clone().unwrap_or_default()
                    }
                }),
                "output_text" | "input_text" | "text" => {
                    json!({ "type": "text", "text": block.text })
                }
                _ => json!({ "type": block.kind, "text": block.text }),
            };

            connection.execute(
                "INSERT INTO part (id, message_id, session_id, time_created, time_updated, data) VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
                params![
                    part_id,
                    message.id,
                    session_id,
                    message_time,
                    message_time,
                    serde_json::to_string(&part_data)?
                ],
            )?;
        }
    }

    let opencode_session_dir = session::opencode::root()?.join("session");
    fs::create_dir_all(&opencode_session_dir)?;
    File::create(opencode_session_dir.join(format!("{session_id}.opencode")))?;

    Ok(())
}

fn seed_opencode_family(root_id: &str, child_id: &str) -> Result<()> {
    let db_path = session::opencode::db_path()?;

    if let Some(parent) = db_path.parent() {
        fs::create_dir_all(parent)?;
    }

    let connection = Connection::open(&db_path)?;
    connection.execute_batch(
        "
        CREATE TABLE session (
            id TEXT PRIMARY KEY,
            project_id TEXT NOT NULL,
            parent_id TEXT,
            slug TEXT NOT NULL,
            directory TEXT NOT NULL,
            title TEXT NOT NULL,
            version TEXT NOT NULL,
            share_url TEXT,
            summary_additions INTEGER,
            summary_deletions INTEGER,
            summary_files INTEGER,
            summary_diffs TEXT,
            revert TEXT,
            permission TEXT,
            time_created INTEGER NOT NULL,
            time_updated INTEGER NOT NULL,
            time_compacting INTEGER,
            time_archived INTEGER,
            workspace_id TEXT
        );
        CREATE TABLE message (
            id TEXT PRIMARY KEY,
            session_id TEXT NOT NULL,
            time_created INTEGER NOT NULL,
            time_updated INTEGER NOT NULL,
            data TEXT NOT NULL
        );
        CREATE TABLE part (
            id TEXT PRIMARY KEY,
            message_id TEXT NOT NULL,
            session_id TEXT NOT NULL,
            time_created INTEGER NOT NULL,
            time_updated INTEGER NOT NULL,
            data TEXT NOT NULL
        );
        ",
    )?;

    connection.execute(
        "INSERT INTO session (id, project_id, parent_id, slug, directory, title, version, time_created, time_updated) VALUES (?1, ?2, NULL, ?3, ?4, ?5, ?6, ?7, ?8)",
        params![
            root_id,
            "project-1",
            root_id,
            "/tmp/root",
            "Root task",
            "1",
            1_744_366_400_000_i64,
            1_744_366_450_000_i64
        ],
    )?;
    connection.execute(
        "INSERT INTO session (id, project_id, parent_id, slug, directory, title, version, time_created, time_updated) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)",
        params![
            child_id,
            "project-1",
            root_id,
            child_id,
            "/tmp/root",
            "Child task",
            "1",
            1_744_366_401_000_i64,
            1_744_366_460_000_i64
        ],
    )?;

    seed_opencode_message(
        &connection,
        root_id,
        "root-msg-1",
        1_744_366_400_000,
        "user",
        json!({ "type": "text", "text": "Root question" }),
    )?;
    seed_opencode_message(
        &connection,
        child_id,
        "child-msg-1",
        1_744_366_401_000,
        "assistant",
        json!({ "type": "text", "text": "Child answer" }),
    )?;

    let opencode_session_dir = session::opencode::root()?.join("session");
    fs::create_dir_all(&opencode_session_dir)?;
    File::create(opencode_session_dir.join(format!("{root_id}.opencode")))?;
    File::create(opencode_session_dir.join(format!("{child_id}.opencode")))?;

    Ok(())
}

fn seed_opencode_message(
    connection: &Connection,
    session_id: &str,
    message_id: &str,
    message_time: i64,
    role: &str,
    part_data: Value,
) -> Result<()> {
    let message_data = json!({
        "role": role,
        "time": { "created": message_time }
    });

    connection.execute(
        "INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES (?1, ?2, ?3, ?4, ?5)",
        params![
            message_id,
            session_id,
            message_time,
            message_time,
            serde_json::to_string(&message_data)?
        ],
    )?;

    connection.execute(
        "INSERT INTO part (id, message_id, session_id, time_created, time_updated, data) VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
        params![
            format!("part-{message_id}"),
            message_id,
            session_id,
            message_time,
            message_time,
            serde_json::to_string(&part_data)?
        ],
    )?;

    Ok(())
}

fn has_codex_function_call_pair(session_id: &str, call_id: &str) -> Result<bool> {
    let path = session::codex::find_session_file(session_id)?;
    let mut has_call = false;
    let mut has_output = false;

    for line in BufReader::new(File::open(path)?).lines() {
        let value: Value = serde_json::from_str(&line?)?;
        if value["type"] != "response_item" {
            continue;
        }

        let payload = &value["payload"];
        if payload["call_id"].as_str() != Some(call_id) {
            continue;
        }

        match payload["type"].as_str() {
            Some("function_call") => has_call = true,
            Some("function_call_output") => has_output = true,
            _ => {}
        }
    }

    Ok(has_call && has_output)
}

fn codex_has_event_type(session_id: &str, event_type: &str) -> Result<bool> {
    let path = session::codex::find_session_file(session_id)?;

    for line in BufReader::new(File::open(path)?).lines() {
        let value: Value = serde_json::from_str(&line?)?;
        if value["type"] != "event_msg" {
            continue;
        }

        if value["payload"]["type"].as_str() == Some(event_type) {
            return Ok(true);
        }
    }

    Ok(false)
}

fn codex_timestamps_are_monotonic(session_id: &str) -> Result<bool> {
    let path = session::codex::find_session_file(session_id)?;
    let mut previous = None;

    for line in BufReader::new(File::open(path)?).lines() {
        let value: Value = serde_json::from_str(&line?)?;
        let Some(raw_timestamp) = value["timestamp"].as_str() else {
            continue;
        };
        let Some(timestamp) = crate::parse_timestamp(raw_timestamp) else {
            continue;
        };

        if previous.is_some_and(|current| timestamp < current) {
            return Ok(false);
        }

        previous = Some(timestamp);
    }

    Ok(true)
}

fn codex_thread_exists(session_id: &str) -> Result<bool> {
    let state_db = latest_codex_state_db()?;
    let connection = Connection::open(state_db)?;
    let count = connection.query_row(
        "SELECT COUNT(*) FROM threads WHERE id = ?1",
        params![session_id],
        |row| row.get::<_, i64>(0),
    )?;

    Ok(count > 0)
}

fn latest_codex_state_db() -> Result<PathBuf> {
    let root = session::codex::root()?;
    let mut candidates = fs::read_dir(root)?
        .filter_map(|entry| entry.ok())
        .map(|entry| entry.path())
        .filter(|path| {
            path.file_name()
                .and_then(|name| name.to_str())
                .is_some_and(|name| name.starts_with("state_") && name.ends_with(".sqlite"))
        })
        .collect::<Vec<_>>();

    candidates.sort();
    candidates
        .pop()
        .ok_or_else(|| anyhow!("Could not find Codex state_*.sqlite"))
}

fn sample_detail(source_app: SourceApp) -> SessionDetail {
    SessionDetail {
        summary: SessionSummary {
            source_app,
            source_session_id: "source-session".to_string(),
            title: "Hello from importer".to_string(),
            cwd: Some("/tmp/agent-session-hub/workspace".to_string()),
            git_branch: Some("main".to_string()),
            transcript_path: "/tmp/source.jsonl".to_string(),
            created_at: Some(1_744_366_400_000),
            updated_at: Some(1_744_366_460_000),
        },
        source_paths: vec!["/tmp/source.jsonl".to_string()],
        messages: vec![
            SessionMessage {
                id: "message-1".to_string(),
                role: "user".to_string(),
                timestamp: Some(1_744_366_400_000),
                blocks: vec![ContentBlock {
                    kind: "text".to_string(),
                    text: Some("Hello from importer".to_string()),
                    tool_name: None,
                    tool_call_id: None,
                    payload: None,
                }],
                session_id: None,
            },
            SessionMessage {
                id: "message-2".to_string(),
                role: "assistant".to_string(),
                timestamp: Some(1_744_366_410_000),
                blocks: vec![
                    ContentBlock {
                        kind: "thinking".to_string(),
                        text: Some("Need to inspect the workspace.".to_string()),
                        tool_name: None,
                        tool_call_id: None,
                        payload: None,
                    },
                    ContentBlock {
                        kind: "tool_use".to_string(),
                        text: None,
                        tool_name: Some("Bash".to_string()),
                        tool_call_id: Some("call_import_1".to_string()),
                        payload: Some(json!({
                          "input": {
                            "command": "pwd"
                          }
                        })),
                    },
                ],
                session_id: None,
            },
            SessionMessage {
                id: "message-3".to_string(),
                role: "tool".to_string(),
                timestamp: Some(1_744_366_420_000),
                blocks: vec![ContentBlock {
                    kind: "function_call_output".to_string(),
                    text: Some("/tmp/agent-session-hub/workspace".to_string()),
                    tool_name: Some("Bash".to_string()),
                    tool_call_id: Some("call_import_1".to_string()),
                    payload: Some(json!({
                      "output": "/tmp/agent-session-hub/workspace"
                    })),
                }],
                session_id: None,
            },
            SessionMessage {
                id: "message-4".to_string(),
                role: "assistant".to_string(),
                timestamp: Some(1_744_366_430_000),
                blocks: vec![ContentBlock {
                    kind: "output_text".to_string(),
                    text: Some("Import finished.".to_string()),
                    tool_name: None,
                    tool_call_id: None,
                    payload: None,
                }],
                session_id: None,
            },
        ],
        events: Vec::new(),
    }
}
