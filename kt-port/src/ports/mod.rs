mod types;
mod utils;
#[cfg(target_os = "windows")]
mod windows;
#[cfg(target_os = "macos")]
mod macos;
#[cfg(target_os = "linux")]
mod linux;

pub use types::{Enhance, KillTarget, PortError, PortInfo, PortScanner};

use sysinfo::{Pid, ProcessesToUpdate, System};

fn get_platform_scanner() -> Result<Box<dyn PortScanner>, PortError> {
    #[cfg(target_os = "windows")]
    return Ok(Box::new(windows::WindowsScanner));

    #[cfg(target_os = "macos")]
    return Ok(Box::new(macos::MacOSScanner));

    #[cfg(target_os = "linux")]
    return Ok(Box::new(linux::LinuxScanner));

    #[cfg(not(any(target_os = "windows", target_os = "macos", target_os = "linux")))]
    Err(PortError::UnsupportedPlatform)
}

pub fn get_port_list() -> Result<Vec<PortInfo>, PortError> {
    let scanner = get_platform_scanner()?;
    let ports = scanner.scan()?;
    ports.enhance()
}

pub async fn kill_process(target: &KillTarget, ports: &[PortInfo]) -> Result<String, PortError> {
    let port_info = ports
        .iter()
        .find(|port_info| target.matches(port_info))
        .ok_or(PortError::ProcessNotFound {
            port: target.port,
            pid: target.pid,
        })?;

    let pid = port_info.pid;
    if pid == 0 {
        return Err(PortError::ProcessKillFailed(pid));
    }

    let mut system = System::new();
    system.refresh_processes(ProcessesToUpdate::All, true);

    let pid_sysinfo = Pid::from_u32(pid);
    let process = system
        .process(pid_sysinfo)
        .ok_or(PortError::ProcessKillFailed(pid))?;

    if process.kill() {
        Ok(format!(
            "Successfully killed process {} on {} {}",
            pid, port_info.protocol, port_info.local_addr
        ))
    } else {
        Err(PortError::ProcessKillFailed(pid))
    }
}
