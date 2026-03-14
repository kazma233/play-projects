use super::types::{PortError, PortInfo, PortScanner};
use super::utils::{
    lookup_username_by_uid, normalize_protocol_label, normalize_socket_addr, run_command,
};
use std::collections::HashMap;

pub struct MacOSScanner;

impl PortScanner for MacOSScanner {
    fn scan(&self) -> Result<Vec<PortInfo>, PortError> {
        let tcp_output = run_command("lsof", &["-P", "-n", "-iTCP", "-F", "pcfnTu"])?;
        let udp_output = run_command("lsof", &["-P", "-n", "-iUDP", "-F", "pcfnTu"])?;

        let mut user_cache = HashMap::new();
        let mut tcp_ports = self.parse_lsof_output(&tcp_output, "TCP", &mut user_cache);
        let mut udp_ports = self.parse_lsof_output(&udp_output, "UDP", &mut user_cache);

        tcp_ports.append(&mut udp_ports);

        Ok(tcp_ports)
    }
}

#[derive(Debug, Default)]
struct ParseState {
    pid: Option<u32>,
    process_name: Option<String>,
    user: Option<String>,
    fd: Option<String>,
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
        let port = extract_port(&local_addr)?;

        let status = self.status.take().unwrap_or_else(|| {
            if normalize_protocol_label(protocol) == "UDP" {
                String::new()
            } else if !remote_addr.is_empty() {
                "ESTABLISHED".to_string()
            } else {
                "LISTEN".to_string()
            }
        });

        self.fd = None;
        self.address = None;

        Some(PortInfo {
            port,
            protocol: normalize_protocol_label(protocol),
            pid,
            process_name,
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
        self.user = None;
        self.fd = None;
        self.address = None;
        self.status = None;
    }
}

impl MacOSScanner {
    fn parse_lsof_output(
        &self,
        output: &str,
        protocol: &str,
        user_cache: &mut HashMap<u32, String>,
    ) -> Vec<PortInfo> {
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
                }
                'c' => {
                    state.process_name = Some(value.to_string());
                }
                'u' => {
                    state.user = Some(resolve_user(value, user_cache));
                }
                'f' => {
                    if let Some(port) = state.try_create_port(protocol) {
                        ports.push(port);
                    }
                    state.fd = Some(value.to_string());
                    state.address = None;
                    state.status = None;
                }
                'n' => {
                    state.address = Some(value.to_string());
                }
                'T' if value.starts_with("ST=") => {
                    state.status = Some(value[3..].to_ascii_uppercase());
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

fn resolve_user(value: &str, user_cache: &mut HashMap<u32, String>) -> String {
    let Ok(uid) = value.parse::<u32>() else {
        return value.to_string();
    };

    if let Some(username) = user_cache.get(&uid) {
        return username.clone();
    }

    let username = lookup_username_by_uid(uid).unwrap_or_else(|| value.to_string());
    user_cache.insert(uid, username.clone());
    username
}

fn extract_port(address: &str) -> Option<u16> {
    if address == "*:*" {
        return None;
    }

    if let Some(rest) = address.strip_prefix('[') {
        let position = rest.rfind("]:")?;
        return rest.get(position + 2..)?.parse::<u16>().ok();
    }

    address.rsplit_once(':')?.1.parse::<u16>().ok()
}

fn extract_addresses(address: &str) -> (String, String) {
    if let Some((local_addr, remote_addr)) = address.split_once("->") {
        return (
            normalize_socket_addr(local_addr),
            normalize_socket_addr(remote_addr),
        );
    }

    (normalize_socket_addr(address), String::new())
}
