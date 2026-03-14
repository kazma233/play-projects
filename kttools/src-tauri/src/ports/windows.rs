use super::types::{PortError, PortInfo, PortScanner};
use super::utils::{normalize_protocol_label, normalize_socket_addr, run_command};

pub struct WindowsScanner;

fn parse_port_from_addr(addr: &str) -> Option<u16> {
    if addr.starts_with('[') {
        let end = addr.rfind(']')?;
        return addr.get(end + 2..)?.parse().ok();
    }

    addr.rsplit_once(':')?.1.parse().ok()
}

fn normalize_windows_state(state: &str) -> String {
    match state.to_ascii_uppercase().as_str() {
        "LISTENING" => "LISTEN".to_string(),
        other => other.to_string(),
    }
}

impl PortScanner for WindowsScanner {
    fn scan(&self) -> Result<Vec<PortInfo>, PortError> {
        let output = run_command("netstat", &["-ano"])?;
        let mut ports = Vec::new();

        for line in output.lines() {
            let trimmed = line.trim();
            if trimmed.is_empty() {
                continue;
            }

            let parts: Vec<&str> = trimmed.split_whitespace().collect();
            let Some(protocol_part) = parts.first() else {
                continue;
            };

            let protocol = normalize_protocol_label(protocol_part);
            if protocol != "TCP" && protocol != "UDP" {
                continue;
            }

            let (local_addr, remote_addr, status, pid) = if protocol == "TCP" {
                if parts.len() < 5 {
                    continue;
                }

                (
                    normalize_socket_addr(parts[1]),
                    normalize_socket_addr(parts[2]),
                    normalize_windows_state(parts[3]),
                    parts[4].parse::<u32>().unwrap_or(0),
                )
            } else {
                if parts.len() < 4 {
                    continue;
                }

                (
                    normalize_socket_addr(parts[1]),
                    normalize_socket_addr(parts[2]),
                    String::new(),
                    parts[3].parse::<u32>().unwrap_or(0),
                )
            };

            let Some(port) = parse_port_from_addr(parts[1]) else {
                continue;
            };

            ports.push(PortInfo {
                port,
                protocol,
                pid,
                process_name: String::new(),
                status,
                process_name_unknown: false,
                local_addr,
                remote_addr,
                user: String::new(),
            });
        }

        Ok(ports)
    }
}
