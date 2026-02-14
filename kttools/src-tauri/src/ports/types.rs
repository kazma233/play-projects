use std::fmt;

#[derive(Debug, Clone)]
#[allow(dead_code)]
pub enum PortError {
    CommandFailed { cmd: String, reason: String },
    ProcessNotFound(u16),
    ProcessKillFailed(u32),
    UnsupportedPlatform,
}

impl fmt::Display for PortError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::CommandFailed { cmd, reason } => {
                write!(f, "Command '{}' failed: {}", cmd, reason)
            }
            Self::ProcessNotFound(port) => write!(f, "Port {} not found", port),
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

pub trait PortScanner: Send + Sync {
    fn scan(&self) -> Result<Vec<PortInfo>, PortError>;
}

pub trait Enhance {
    fn enhance(self) -> Result<Vec<PortInfo>, PortError>;
}

impl Enhance for Vec<PortInfo> {
    fn enhance(self) -> Result<Vec<PortInfo>, PortError> {
        let mut unique_ports: std::collections::HashMap<(u16, String, u32), PortInfo> =
            std::collections::HashMap::new();

        for mut port in self {
            if port.pid == 0 {
                port.process_name = "unknown".to_string();
                port.process_name_unknown = true;
            }

            if port.user.is_empty() {
                port.user = "unknown".to_string();
            }

            let key = (port.port, port.protocol.clone(), port.pid);
            unique_ports.insert(key, port);
        }

        Ok(unique_ports.into_values().collect())
    }
}
