use super::types::{PortError, PortInfo, PortScanner};
use super::utils::run_command;

pub struct WindowsScanner;

fn parse_port_from_addr(addr: &str) -> Option<u16> {
    if addr.starts_with('[') {
        addr.rfind(']')
            .and_then(|end| addr.get(end + 2..))?
            .parse()
            .ok()
    } else {
        addr.rfind(':')
            .and_then(|pos| addr.get(pos + 1..))?
            .parse()
            .ok()
    }
}

impl PortScanner for WindowsScanner {
    fn scan(&self) -> Result<Vec<PortInfo>, PortError> {
        let output = run_command("netstat", &["-ano"])?;
        let mut ports = Vec::new();

        let mut started = false;
        for line in output.lines() {
            let trimmed = line.trim();
            if trimmed.is_empty() {
                continue;
            }

            if !started && trimmed.starts_with("Proto") {
                started = true;
                continue;
            }

            if !started {
                continue;
            }

            let parts: Vec<&str> = line.split_whitespace().collect();
            if parts.len() < 5 {
                continue;
            }

            let protocol = parts[0].to_uppercase();
            if !protocol.starts_with("TCP") && !protocol.starts_with("UDP") {
                continue;
            }

            let local_addr = parts[1];
            let port = match parse_port_from_addr(local_addr) {
                Some(p) => p,
                None => continue,
            };

            let (status, status_idx) = if protocol.starts_with("TCP") {
                if parts.len() >= 4 {
                    (parts[3].to_string(), 4)
                } else {
                    continue;
                }
            } else {
                ("-".to_string(), 3)
            };

            let pid = if parts.len() > status_idx {
                parts[status_idx].parse::<u32>().unwrap_or(0)
            } else {
                0
            };

            ports.push(PortInfo {
                port,
                protocol,
                pid,
                process_name: String::new(),
                status: if status == "LISTENING" {
                    "LISTEN".to_string()
                } else {
                    status
                },
                process_name_unknown: false,
                local_addr: String::new(),
                remote_addr: String::new(),
                user: String::new(),
            });
        }

        Ok(ports)
    }
}
