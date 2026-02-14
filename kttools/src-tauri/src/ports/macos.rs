use super::types::{PortError, PortInfo, PortScanner};
use super::utils::run_command;

pub struct MacOSScanner;

impl PortScanner for MacOSScanner {
    fn scan(&self) -> Result<Vec<PortInfo>, PortError> {
        let tcp_output = run_command("lsof", &["-P", "-n", "-iTCP", "-F", "pcfnTu"])?;
        let udp_output = run_command("lsof", &["-P", "-n", "-iUDP", "-F", "pcfnTu"])?;

        let mut tcp_ports = self.parse_lsof_output(&tcp_output, "TCP");
        let mut udp_ports = self.parse_lsof_output(&udp_output, "UDP");

        tcp_ports.append(&mut udp_ports);

        Ok(tcp_ports)
    }
}

#[derive(Debug, Default)]
struct ParseState {
    pid: Option<u32>,
    process_name: Option<String>,
    user: Option<String>,
    fd: Option<u32>,
    address: Option<String>,
    status: Option<String>,
}

impl ParseState {
    fn is_complete(&self) -> bool {
        self.pid.is_some()
            && self.process_name.is_some()
            && self.fd.is_some()
            && self.address.is_some()
    }

    fn try_create_port(&mut self, protocol: &str) -> Option<PortInfo> {
        if !self.is_complete() {
            return None;
        }

        let pid = self.pid?;
        let process_name = self.process_name.clone()?;
        let address = self.address.clone()?;
        let user = self.user.clone().unwrap_or_default();

        let (local_addr, remote_addr) = extract_addresses(&address);

        let port = extract_port(&address)?;

        let status = self.status.take().unwrap_or_else(|| {
            if protocol == "UDP" {
                "*".to_string()
            } else if address.contains("->") {
                "ESTABLISHED".to_string()
            } else {
                "LISTEN".to_string()
            }
        });

        self.fd = None;
        self.address = None;

        Some(PortInfo {
            port,
            protocol: protocol.to_string(),
            pid,
            process_name: process_name.clone(),
            status,
            process_name_unknown: false,
            local_addr,
            remote_addr,
            user,
        })
    }

    fn reset_process(&mut self) {
        self.pid = None;
        self.process_name = None;
        self.fd = None;
        self.address = None;
        self.status = None;
    }
}

impl MacOSScanner {
    fn parse_lsof_output(&self, output: &str, protocol: &str) -> Vec<PortInfo> {
        let mut state = ParseState::default();
        let mut ports = Vec::new();

        for line in output.lines() {
            if line.is_empty() {
                continue;
            }

            let field = line.chars().next().unwrap();
            let value = &line[1..];

            match field {
                'p' => {
                    if let Some(port) = state.try_create_port(protocol) {
                        ports.push(port);
                    }
                    state.reset_process();
                    state.pid = value.parse().ok();
                    state.user = None;
                }
                'c' => {
                    state.process_name = Some(value.to_string());
                }
                'u' => {
                    state.user = Some(value.to_string());
                }
                'f' => {
                    if let Some(port) = state.try_create_port(protocol) {
                        ports.push(port);
                    }
                    state.address = None;
                    state.status = None;
                    state.fd = value.parse().ok();
                }
                'n' => {
                    state.address = Some(value.to_string());
                }
                'T' if value.starts_with("ST=") => {
                    state.status = Some(value[3..].to_string());
                }
                _ => {}
            }
        }

        if let Some(port) = state.try_create_port(protocol) {
            ports.push(port);
        }

        ports
    }
}

fn extract_port(address: &str) -> Option<u16> {
    let local_part = address.split("->").next()?;

    if local_part == "*:*" || !local_part.contains(':') {
        return None;
    }

    let port = if local_part.starts_with('[') {
        local_part
            .rfind("]:")
            .and_then(|pos| local_part[pos + 2..].parse::<u16>().ok())?
    } else {
        local_part
            .rfind(':')
            .and_then(|pos| local_part[pos + 1..].parse::<u16>().ok())?
    };

    Some(port)
}

fn extract_addresses(address: &str) -> (String, String) {
    if address.contains("->") {
        let parts: Vec<&str> = address.split("->").collect();
        if parts.len() == 2 {
            return (parts[0].to_string(), parts[1].to_string());
        }
    }

    (address.to_string(), String::new())
}
