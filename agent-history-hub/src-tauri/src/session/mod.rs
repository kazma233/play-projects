use std::path::{Path, PathBuf};

use anyhow::Result;

use crate::{
    SessionDetail, SessionDetailOverview, SessionEventPage, SessionFileEntry, SessionMessagePage,
    SessionSummary, SourceApp,
};

pub(crate) mod claude_code;
pub(crate) mod codex;
pub(crate) mod opencode;

pub(crate) trait SessionReader {
    fn list_entries(&self) -> Result<Vec<SessionFileEntry>>;

    fn clear_cache(&self) -> Result<()> {
        Ok(())
    }

    fn resolve_path(&self, source_session_id: &str) -> Result<PathBuf>;

    fn parse_summary(&self, path: &Path) -> Result<SessionSummary>;

    fn parse_overview(&self, path: &Path) -> Result<SessionDetailOverview>;

    fn parse_messages_page(
        &self,
        path: &Path,
        offset: usize,
        limit: usize,
    ) -> Result<SessionMessagePage>;

    fn parse_events_page(
        &self,
        path: &Path,
        offset: usize,
        limit: usize,
    ) -> Result<SessionEventPage>;

    fn parse_detail(&self, path: &Path) -> Result<SessionDetail>;
}

pub(crate) trait SessionExporter {
    fn planned_import_paths(
        &self,
        summary: &SessionSummary,
        session_id: &str,
    ) -> Result<Vec<String>>;

    fn export_session(
        &self,
        detail: &SessionDetail,
        new_session_id: &str,
    ) -> Result<(String, Vec<String>)>;
}

pub(crate) fn reader(source_app: SourceApp) -> &'static dyn SessionReader {
    match source_app {
        SourceApp::Codex => &codex::BACKEND,
        SourceApp::ClaudeCode => &claude_code::BACKEND,
        SourceApp::OpenCode => &opencode::BACKEND,
    }
}

pub(crate) fn clear_all_caches() -> Result<()> {
    reader(SourceApp::Codex).clear_cache()?;
    reader(SourceApp::ClaudeCode).clear_cache()?;
    reader(SourceApp::OpenCode).clear_cache()?;
    Ok(())
}

pub(crate) fn exporter(source_app: SourceApp) -> &'static dyn SessionExporter {
    match source_app {
        SourceApp::Codex => &codex::BACKEND,
        SourceApp::ClaudeCode => &claude_code::BACKEND,
        SourceApp::OpenCode => &opencode::BACKEND,
    }
}

fn sort_entries(entries: &mut [SessionFileEntry]) {
    entries.sort_by(|left, right| {
        right
            .sort_timestamp
            .cmp(&left.sort_timestamp)
            .then_with(|| left.path.cmp(&right.path))
    });
}
