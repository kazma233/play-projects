// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

#[cfg(target_os = "linux")]
fn configure_linux_wayland_compatibility_env() {
    use std::path::Path;

    let has_wayland_display = std::env::var_os("WAYLAND_DISPLAY").is_some();

    if !has_wayland_display {
        return;
    }

    // These are WebKitGTK compatibility workarounds, not backend selection.
    if std::env::var_os("WEBKIT_DISABLE_DMABUF_RENDERER").is_none() {
        unsafe {
            std::env::set_var("WEBKIT_DISABLE_DMABUF_RENDERER", "1");
        }
        eprintln!("kt-tools: enabled WEBKIT_DISABLE_DMABUF_RENDERER=1 for Wayland compatibility");
    }

    if Path::new("/proc/driver/nvidia/version").exists()
        && std::env::var_os("__NV_DISABLE_EXPLICIT_SYNC").is_none()
    {
        unsafe {
            std::env::set_var("__NV_DISABLE_EXPLICIT_SYNC", "1");
        }
        eprintln!(
            "kt-tools: enabled __NV_DISABLE_EXPLICIT_SYNC=1 for NVIDIA Wayland compatibility"
        );
    }
}

#[cfg(not(target_os = "linux"))]
fn configure_linux_wayland_compatibility_env() {}

fn main() {
    configure_linux_wayland_compatibility_env();
    kt_tools_lib::run()
}
