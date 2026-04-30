use std::path::{Path, PathBuf};
use std::str::FromStr;

use anyhow::{bail, Context, Result};

use crate::session;
use crate::state::{SessionFileCatalog, SessionIndexState};
use crate::{
    generate_target_session_id, ImportPreview, ImportResult, SessionDetail, SessionDetailOverview,
    SessionEventPage, SessionFileEntry, SessionMessagePage, SessionPage, SessionSummary, SourceApp,
    SourceStatus,
};

const DEFAULT_SESSION_PAGE_SIZE: usize = 20;
const MAX_SESSION_PAGE_SIZE: usize = 200;
const DEFAULT_DETAIL_PAGE_SIZE: usize = 40;
const MAX_DETAIL_PAGE_SIZE: usize = 100;

struct ImportAssessment {
    supported: bool,
    import_level: &'static str,
    warnings: Vec<String>,
}

#[tauri::command]
pub(crate) async fn detect_sources() -> std::result::Result<Vec<SourceStatus>, String> {
    run_blocking(detect_sources_inner).await
}

#[tauri::command]
pub(crate) async fn clear_session_caches(
    state: tauri::State<'_, SessionIndexState>,
) -> std::result::Result<(), String> {
    let session_index_state = state.inner().clone();
    run_blocking(move || clear_session_caches_inner(&session_index_state)).await
}

#[tauri::command]
pub(crate) async fn list_sessions(
    source_app: String,
    offset: Option<usize>,
    limit: Option<usize>,
    refresh: Option<bool>,
    state: tauri::State<'_, SessionIndexState>,
) -> std::result::Result<SessionPage, String> {
    let source = SourceApp::from_str(&source_app).map_err(|error| error.to_string())?;
    let session_index_state = state.inner().clone();
    let offset = offset.unwrap_or_default();
    let limit = limit
        .unwrap_or(DEFAULT_SESSION_PAGE_SIZE)
        .clamp(1, MAX_SESSION_PAGE_SIZE);
    let refresh = refresh.unwrap_or(offset == 0);

    run_blocking(move || list_sessions_inner(&session_index_state, source, offset, limit, refresh))
        .await
}

#[tauri::command]
pub(crate) async fn get_session_overview(
    source_app: String,
    source_session_id: String,
    transcript_path: Option<String>,
) -> std::result::Result<SessionDetailOverview, String> {
    let source = SourceApp::from_str(&source_app).map_err(|error| error.to_string())?;
    run_blocking(move || {
        get_session_overview_inner(source, &source_session_id, transcript_path.as_deref())
    })
    .await
}

#[tauri::command]
pub(crate) async fn get_session_messages(
    source_app: String,
    source_session_id: String,
    transcript_path: Option<String>,
    offset: Option<usize>,
    limit: Option<usize>,
) -> std::result::Result<SessionMessagePage, String> {
    let source = SourceApp::from_str(&source_app).map_err(|error| error.to_string())?;
    let offset = offset.unwrap_or_default();
    let limit = limit
        .unwrap_or(DEFAULT_DETAIL_PAGE_SIZE)
        .clamp(1, MAX_DETAIL_PAGE_SIZE);

    run_blocking(move || {
        get_session_messages_inner(
            source,
            &source_session_id,
            transcript_path.as_deref(),
            offset,
            limit,
        )
    })
    .await
}

#[tauri::command]
pub(crate) async fn get_session_events(
    source_app: String,
    source_session_id: String,
    transcript_path: Option<String>,
    offset: Option<usize>,
    limit: Option<usize>,
) -> std::result::Result<SessionEventPage, String> {
    let source = SourceApp::from_str(&source_app).map_err(|error| error.to_string())?;
    let offset = offset.unwrap_or_default();
    let limit = limit
        .unwrap_or(DEFAULT_DETAIL_PAGE_SIZE)
        .clamp(1, MAX_DETAIL_PAGE_SIZE);

    run_blocking(move || {
        get_session_events_inner(
            source,
            &source_session_id,
            transcript_path.as_deref(),
            offset,
            limit,
        )
    })
    .await
}

#[tauri::command]
pub(crate) async fn get_session(
    source_app: String,
    source_session_id: String,
    transcript_path: Option<String>,
) -> std::result::Result<SessionDetail, String> {
    let source = SourceApp::from_str(&source_app).map_err(|error| error.to_string())?;
    run_blocking(move || get_session_inner(source, &source_session_id, transcript_path.as_deref()))
        .await
}

#[tauri::command]
pub(crate) async fn preview_import(
    source_app: String,
    source_session_id: String,
    target_app: String,
    transcript_path: Option<String>,
) -> std::result::Result<ImportPreview, String> {
    let source = SourceApp::from_str(&source_app).map_err(|error| error.to_string())?;
    let target = SourceApp::from_str(&target_app).map_err(|error| error.to_string())?;
    run_blocking(move || {
        preview_import_inner(
            source,
            &source_session_id,
            target,
            transcript_path.as_deref(),
        )
    })
    .await
}

#[tauri::command]
pub(crate) async fn import_session(
    source_app: String,
    source_session_id: String,
    target_app: String,
    transcript_path: Option<String>,
) -> std::result::Result<ImportResult, String> {
    let source = SourceApp::from_str(&source_app).map_err(|error| error.to_string())?;
    let target = SourceApp::from_str(&target_app).map_err(|error| error.to_string())?;
    run_blocking(move || {
        import_session_inner(
            source,
            &source_session_id,
            target,
            transcript_path.as_deref(),
        )
    })
    .await
}

async fn run_blocking<T, F>(operation: F) -> std::result::Result<T, String>
where
    T: Send + 'static,
    F: FnOnce() -> Result<T> + Send + 'static,
{
    tauri::async_runtime::spawn_blocking(move || operation().map_err(|error| error.to_string()))
        .await
        .map_err(|error| error.to_string())?
}

fn detect_sources_inner() -> Result<Vec<SourceStatus>> {
    let codex_root = session::codex::root()?;
    let claude_root = session::claude_code::root()?;
    let opencode_root = session::opencode::root()?;

    Ok(vec![
        SourceStatus {
            app: SourceApp::Codex,
            available: codex_root.exists(),
            root_path: codex_root
                .exists()
                .then(|| codex_root.display().to_string()),
            session_count: session::codex::count_sessions().unwrap_or_default(),
            note: codex_root
                .exists()
                .then(|| "Using ~/.codex/sessions as the primary transcript source.".to_string()),
        },
        SourceStatus {
            app: SourceApp::ClaudeCode,
            available: claude_root.exists(),
            root_path: claude_root
                .exists()
                .then(|| claude_root.display().to_string()),
            session_count: session::claude_code::count_sessions().unwrap_or_default(),
            note: claude_root
                .exists()
                .then(|| "Reading project-level session JSONL files.".to_string()),
        },
        SourceStatus {
            app: SourceApp::OpenCode,
            available: opencode_root.exists(),
            root_path: opencode_root
                .exists()
                .then(|| opencode_root.display().to_string()),
            session_count: session::opencode::count_sessions().unwrap_or_default(),
            note: if opencode_root.exists() {
                Some("Reading sessions from ~/.local/share/opencode/opencode.db.".to_string())
            } else {
                None
            },
        },
    ])
}

fn clear_session_caches_inner(state: &SessionIndexState) -> Result<()> {
    state.clear()?;
    session::clear_all_caches()
}

fn list_sessions_inner(
    state: &SessionIndexState,
    source_app: SourceApp,
    offset: usize,
    limit: usize,
    refresh: bool,
) -> Result<SessionPage> {
    match build_session_page(state, source_app, offset, limit, refresh) {
        Ok(page) => Ok(page),
        Err(error) if !refresh => build_session_page(state, source_app, offset, limit, true)
            .with_context(|| format!("Failed to load cached session page: {error}")),
        Err(error) => Err(error),
    }
}

fn build_session_page(
    state: &SessionIndexState,
    source_app: SourceApp,
    offset: usize,
    limit: usize,
    refresh: bool,
) -> Result<SessionPage> {
    let entries = session_catalog_entries(state, source_app, refresh)?;
    let total_count = entries.len();
    let offset = offset.min(total_count);
    let end = offset.saturating_add(limit).min(total_count);
    let sessions = entries[offset..end]
        .iter()
        .map(|entry| {
            entry
                .summary
                .clone()
                .map(Ok)
                .unwrap_or_else(|| parse_session_summary(source_app, &entry.path))
        })
        .collect::<Result<Vec<_>>>()?;
    let has_more = end < total_count;

    Ok(SessionPage {
        source_app,
        sessions,
        offset,
        limit,
        next_offset: has_more.then_some(end),
        total_count,
        has_more,
    })
}

fn session_catalog_entries(
    state: &SessionIndexState,
    source_app: SourceApp,
    refresh: bool,
) -> Result<Vec<SessionFileEntry>> {
    if !refresh {
        if let Some(entries) = state.catalog(source_app)? {
            return Ok(entries);
        }
    }

    let catalog = build_session_catalog(source_app)?;
    let entries = catalog.entries.clone();
    state.store_catalog(source_app, catalog)?;
    Ok(entries)
}

fn build_session_catalog(source_app: SourceApp) -> Result<SessionFileCatalog> {
    Ok(SessionFileCatalog {
        entries: session::reader(source_app).list_entries()?,
    })
}

fn parse_session_summary(source_app: SourceApp, path: &Path) -> Result<SessionSummary> {
    session::reader(source_app).parse_summary(path)
}

fn get_session_overview_inner(
    source_app: SourceApp,
    source_session_id: &str,
    transcript_path: Option<&str>,
) -> Result<SessionDetailOverview> {
    let path = resolve_session_path(source_app, source_session_id, transcript_path)?;
    session::reader(source_app).parse_overview(&path)
}

fn get_session_messages_inner(
    source_app: SourceApp,
    source_session_id: &str,
    transcript_path: Option<&str>,
    offset: usize,
    limit: usize,
) -> Result<SessionMessagePage> {
    let path = resolve_session_path(source_app, source_session_id, transcript_path)?;
    session::reader(source_app).parse_messages_page(&path, offset, limit)
}

fn get_session_events_inner(
    source_app: SourceApp,
    source_session_id: &str,
    transcript_path: Option<&str>,
    offset: usize,
    limit: usize,
) -> Result<SessionEventPage> {
    let path = resolve_session_path(source_app, source_session_id, transcript_path)?;
    session::reader(source_app).parse_events_page(&path, offset, limit)
}

pub(crate) fn get_session_inner(
    source_app: SourceApp,
    source_session_id: &str,
    transcript_path: Option<&str>,
) -> Result<SessionDetail> {
    let path = resolve_session_path(source_app, source_session_id, transcript_path)?;
    session::reader(source_app).parse_detail(&path)
}

fn resolve_session_path(
    source_app: SourceApp,
    source_session_id: &str,
    transcript_path: Option<&str>,
) -> Result<PathBuf> {
    if let Some(path) = transcript_path {
        return Ok(PathBuf::from(path));
    }

    session::reader(source_app).resolve_path(source_session_id)
}

fn preview_import_inner(
    source_app: SourceApp,
    source_session_id: &str,
    target_app: SourceApp,
    transcript_path: Option<&str>,
) -> Result<ImportPreview> {
    let detail = get_session_inner(source_app, source_session_id, transcript_path)?;
    let assessment = assess_import(&detail, target_app);
    let placeholder_session_id = "<generated-session-id>";

    Ok(ImportPreview {
        source_app,
        source_session_id: source_session_id.to_string(),
        target_app,
        supported: assessment.supported,
        import_level: assessment.import_level.to_string(),
        warnings: assessment.warnings,
        created_paths: session::exporter(target_app)
            .planned_import_paths(&detail.summary, placeholder_session_id)?,
        backup_paths: Vec::new(),
    })
}

pub(crate) fn import_session_inner(
    source_app: SourceApp,
    source_session_id: &str,
    target_app: SourceApp,
    transcript_path: Option<&str>,
) -> Result<ImportResult> {
    let detail = get_session_inner(source_app, source_session_id, transcript_path)?;
    let assessment = assess_import(&detail, target_app);

    if !assessment.supported {
        bail!(
            "Import from {} to {} is not supported in the current MVP.",
            source_app.as_str(),
            target_app.as_str()
        );
    }

    let new_session_id = generate_target_session_id(target_app);
    let (created_session_id, created_paths) =
        session::exporter(target_app).export_session(&detail, &new_session_id)?;

    get_session_inner(target_app, &created_session_id, None).with_context(|| {
        format!(
            "Imported session was written, but read-back validation failed for {}.",
            created_session_id
        )
    })?;

    Ok(ImportResult {
        target_app,
        created_session_id,
        created_paths,
        backup_paths: Vec::new(),
        resume_cwd: detail.summary.cwd.clone(),
        warnings: assessment.warnings,
    })
}

fn assess_import(detail: &SessionDetail, target_app: SourceApp) -> ImportAssessment {
    let source_app = detail.summary.source_app;

    if detail.messages.is_empty() {
        return ImportAssessment {
            supported: false,
            import_level: "unsupported",
            warnings: vec!["The source session has no importable messages.".to_string()],
        };
    }

    if source_app == target_app {
        return ImportAssessment {
            supported: false,
            import_level: "unsupported",
            warnings: vec![
                "The current MVP only exposes cross-program import. Same-app cloning is intentionally disabled."
                    .to_string(),
            ],
        };
    }

    let mut warnings = vec![
        "Only the normalized message timeline is imported. Side-channel runtime events are skipped."
            .to_string(),
        "The target program may render the imported conversation differently from the original UI."
            .to_string(),
        "This import path creates a brand-new session file, so backup paths are usually empty."
            .to_string(),
    ];

    if detail.summary.cwd.is_none() {
        warnings.push(
            "The source session does not expose a cwd, so the importer will fall back to the home directory."
                .to_string(),
        );
    }

    if !detail.events.is_empty() {
        warnings.push("Non-message events are not recreated in the target program.".to_string());
    }

    match (source_app, target_app) {
        (SourceApp::Codex, SourceApp::ClaudeCode)
        | (SourceApp::OpenCode, SourceApp::ClaudeCode) => warnings.push(
            "Tool calls are mapped into Claude tool_use/tool_result records on a best-effort basis."
                .to_string(),
        ),
        (SourceApp::ClaudeCode, SourceApp::Codex)
        | (SourceApp::OpenCode, SourceApp::Codex) => warnings.push(
            "Codex import writes transcript JSONL only. Agent Session Hub validates the written transcript, but Codex still needs to resume the session once before its internal SQLite state is populated."
                .to_string(),
        ),
        (_, SourceApp::OpenCode) => warnings.push(
            "OpenCode import is delegated to the official CLI importer using generated session JSON."
                .to_string(),
        ),
        _ => {}
    }

    ImportAssessment {
        supported: true,
        import_level: "partial",
        warnings,
    }
}
