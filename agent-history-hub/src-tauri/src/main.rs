#[cfg(target_os = "linux")]
fn configure_linux_display_workarounds() {
    let session_type = std::env::var("XDG_SESSION_TYPE").ok();
    let has_wayland_display = std::env::var_os("WAYLAND_DISPLAY").is_some();
    let disable_dmabuf_already_set = std::env::var_os("WEBKIT_DISABLE_DMABUF_RENDERER").is_some();

    if !disable_dmabuf_already_set
        && (session_type.as_deref() == Some("wayland") || has_wayland_display)
    {
        // Work around WebKitGTK dmabuf + explicit sync crashes observed on Wayland,
        // especially on NVIDIA systems. Users can still override this manually.
        std::env::set_var("WEBKIT_DISABLE_DMABUF_RENDERER", "1");
    }
}

#[cfg(not(target_os = "linux"))]
fn configure_linux_display_workarounds() {}

fn main() {
    configure_linux_display_workarounds();
    agent_session_hub::run();
}
