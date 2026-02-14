mod types;
mod utils;
mod windows;
mod macos;
mod linux;

pub use types::{Enhance, PortError, PortInfo, PortScanner};

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

pub async fn kill_process(port: u16, ports: &[PortInfo]) -> Result<String, PortError> {
    let port_info = ports
        .iter()
        .find(|p| p.port == port)
        .ok_or(PortError::ProcessNotFound(port))?;
    
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
        Ok(format!("Successfully killed process {} on port {}", pid, port))
    } else {
        Err(PortError::ProcessKillFailed(pid))
    }
}
