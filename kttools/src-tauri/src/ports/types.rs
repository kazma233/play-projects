use std::collections::HashMap;
use std::fmt;

use sysinfo::{Pid, ProcessesToUpdate, System};

#[derive(Debug, Clone)]
#[allow(dead_code)]
pub enum PortError {
    CommandFailed { cmd: String, reason: String },
    ProcessNotFound { port: u16, pid: u32 },
    ProcessKillFailed(u32),
    UnsupportedPlatform,
}

impl fmt::Display for PortError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::CommandFailed { cmd, reason } => {
                write!(f, "Command '{}' failed: {}", cmd, reason)
            }
            Self::ProcessNotFound { port, pid } => {
                write!(f, "Process {} on port {} is no longer available", pid, port)
            }
            Self::ProcessKillFailed(pid) => write!(f, "Failed to kill process {}", pid),
            Self::UnsupportedPlatform => write!(f, "Unsupported operating system"),
        }
    }
}

impl std::error::Error for PortError {}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct PortInfo {
    pub port: u16,
    pub protocol: String,
    pub pid: u32,
    pub process_name: String,
    pub status: String,
    pub process_name_unknown: bool,
    pub local_addr: String,
    pub remote_addr: String,
    pub user: String,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct KillTarget {
    pub port: u16,
    pub protocol: String,
    pub pid: u32,
    pub local_addr: String,
    pub remote_addr: String,
}

impl KillTarget {
    pub fn matches(&self, port_info: &PortInfo) -> bool {
        self.port == port_info.port
            && self.pid == port_info.pid
            && normalize_protocol(&self.protocol) == normalize_protocol(&port_info.protocol)
            && normalize_addr(&self.local_addr) == normalize_addr(&port_info.local_addr)
            && normalize_addr(&self.remote_addr) == normalize_addr(&port_info.remote_addr)
    }
}

impl PortInfo {
    fn dedupe_key(&self) -> (u16, String, String, String, u32, String) {
        (
            self.port,
            normalize_protocol(&self.protocol),
            normalize_addr(&self.local_addr),
            normalize_addr(&self.remote_addr),
            self.pid,
            self.status.clone(),
        )
    }

    fn completeness_score(&self) -> usize {
        usize::from(self.pid > 0)
            + usize::from(!self.process_name.trim().is_empty() && !self.process_name_unknown)
            + usize::from(!self.local_addr.trim().is_empty())
            + usize::from(!self.remote_addr.trim().is_empty())
            + usize::from(!self.user.trim().is_empty() && self.user != "unknown")
            + usize::from(!self.status.trim().is_empty())
    }
}

fn normalize_protocol(protocol: &str) -> String {
    let upper = protocol.trim().to_ascii_uppercase();
    if upper.starts_with("TCP") {
        "TCP".to_string()
    } else if upper.starts_with("UDP") {
        "UDP".to_string()
    } else {
        upper
    }
}

fn normalize_addr(addr: &str) -> String {
    addr.trim().to_ascii_lowercase()
}

fn lookup_process_name(system: &System, pid: u32) -> Option<String> {
    let process = system.process(Pid::from_u32(pid))?;
    let name = process.name().to_string_lossy().trim().to_string();
    if name.is_empty() {
        None
    } else {
        Some(name)
    }
}

pub trait PortScanner: Send + Sync {
    fn scan(&self) -> Result<Vec<PortInfo>, PortError>;
}

pub trait Enhance {
    fn enhance(self) -> Result<Vec<PortInfo>, PortError>;
}

impl Enhance for Vec<PortInfo> {
    fn enhance(self) -> Result<Vec<PortInfo>, PortError> {
        let needs_process_lookup = self
            .iter()
            .any(|port| port.pid > 0 && port.process_name.trim().is_empty());

        let system = needs_process_lookup.then(|| {
            let mut system = System::new();
            system.refresh_processes(ProcessesToUpdate::All, true);
            system
        });

        let mut unique_ports: HashMap<(u16, String, String, String, u32, String), PortInfo> =
            HashMap::new();

        for mut port in self {
            port.protocol = normalize_protocol(&port.protocol);

            if port.pid > 0 && port.process_name.trim().is_empty() {
                if let Some(system) = system.as_ref() {
                    if let Some(process_name) = lookup_process_name(system, port.pid) {
                        port.process_name = process_name;
                    }
                }
            }

            if port.process_name.trim().is_empty() {
                port.process_name = "unknown".to_string();
                port.process_name_unknown = true;
            } else {
                port.process_name_unknown = false;
            }

            if port.user.trim().is_empty() {
                port.user = "unknown".to_string();
            }

            let key = port.dedupe_key();

            match unique_ports.get_mut(&key) {
                Some(existing) => {
                    if port.completeness_score() >= existing.completeness_score() {
                        *existing = port;
                    }
                }
                None => {
                    unique_ports.insert(key, port);
                }
            }
        }

        let mut ports: Vec<PortInfo> = unique_ports.into_values().collect();
        ports.sort_by(|left, right| {
            left.port
                .cmp(&right.port)
                .then_with(|| left.protocol.cmp(&right.protocol))
                .then_with(|| left.local_addr.cmp(&right.local_addr))
                .then_with(|| left.remote_addr.cmp(&right.remote_addr))
                .then_with(|| left.pid.cmp(&right.pid))
                .then_with(|| left.status.cmp(&right.status))
        });

        Ok(ports)
    }
}
