mod app;
mod session;
mod shared;
mod state;

pub(crate) use shared::*;

pub fn run() {
    tauri::Builder::default()
        .manage(state::SessionIndexState::default())
        .invoke_handler(tauri::generate_handler![
            app::detect_sources,
            app::clear_session_caches,
            app::list_sessions,
            app::get_session_overview,
            app::get_session_messages,
            app::get_session_events,
            app::get_session,
            app::preview_import,
            app::import_session
        ])
        .run(tauri::generate_context!())
        .expect("failed to run Agent Session Hub");
}

#[cfg(test)]
mod tests;
