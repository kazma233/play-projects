use super::types::{PortError, PortInfo, PortScanner};
use super::utils::run_command;
use std::collections::HashMap;
use std::process::Command;

pub struct LinuxScanner;

fn parse_port_from_addr(addr: &str) -> Option<u16> {
    parse_decimal_port(addr)
}

fn parse_decimal_port(addr: &str) -> Option<u16> {
    if addr.starts_with('[') {
        if let Some(bracket_end) = addr.find("]:") {
            let port_str = &addr[bracket_end + 2..];
            return port_str.parse().ok();
        }
    }
    if let Some(colon_pos) = addr.rfind(':') {
        let port_str = &addr[colon_pos + 1..];
        port_str.parse().ok()
    } else {
        None
    }
}

fn hex_to_ip(hex: &str) -> Option<String> {
    let parts: Vec<u8> = (0..hex.len())
        .step_by(2)
        .filter_map(|i| u8::from_str_radix(&hex[i..i + 2], 16).ok())
        .collect();

    if parts.len() == 4 {
        Some(format!(
            "{}.{}.{}.{}",
            parts[0], parts[1], parts[2], parts[3]
        ))
    } else if parts.len() == 16 {
        let mut result = String::new();
        for (i, chunk) in parts.chunks(16).enumerate() {
            if i > 0 {
                result.push(':');
            }
            for (j, byte) in chunk.chunks(2).enumerate() {
                if j > 0 || i > 0 {
                    result.push(':');
                }
                result.push_str(&format!("{:x}{:x}", byte[0], byte[1]));
            }
        }
        Some(result)
    } else {
        None
    }
}

fn parse_local_remote_addr(local: &str, remote: &str) -> (String, String) {
    let local_parsed = parse_hex_addr(local);
    let remote_parsed = parse_hex_addr(remote);
    (local_parsed, remote_parsed)
}

fn parse_hex_addr(addr: &str) -> String {
    if let Some(colon_pos) = addr.find(':') {
        let ip_hex = &addr[..colon_pos];
        let port_hex = &addr[colon_pos + 1..];

        if let (Some(ip), Ok(port)) = (hex_to_ip(ip_hex), u16::from_str_radix(port_hex, 16)) {
            return format!("{}:{}", ip, port);
        }
    }
    addr.to_string()
}

fn get_username_from_uid(uid: u32) -> String {
    if let Ok(output) = Command::new("id").arg("-nu").arg(uid.to_string()).output() {
        if output.status.success() {
            return String::from_utf8_lossy(&output.stdout).trim().to_string();
        }
    }
    if let Ok(passwd) = std::fs::read_to_string("/etc/passwd") {
        for line in passwd.lines() {
            let parts: Vec<&str> = line.split(':').collect();
            if parts.len() >= 3 && parts[2].parse::<u32>().unwrap_or(0) == uid {
                return parts[0].to_string();
            }
        }
    }
    format!("{}", uid)
}

impl PortScanner for LinuxScanner {
    fn scan(&self) -> Result<Vec<PortInfo>, PortError> {
        let mut ports = Vec::new();

        if let Ok(tcp_content) = std::fs::read_to_string("/proc/net/tcp") {
            ports.extend(parse_proc_net_tcp(&tcp_content, "tcp"));
        }

        if let Ok(tcp6_content) = std::fs::read_to_string("/proc/net/tcp6") {
            ports.extend(parse_proc_net_tcp(&tcp6_content, "tcp6"));
        }

        if let Ok(udp_content) = std::fs::read_to_string("/proc/net/udp") {
            ports.extend(parse_proc_net_udp(&udp_content, "udp"));
        }

        if let Ok(udp6_content) = std::fs::read_to_string("/proc/net/udp6") {
            ports.extend(parse_proc_net_udp(&udp6_content, "udp6"));
        }

        if !ports.is_empty() {
            enhance_ports_with_process_info(&mut ports)?;
        }

        Ok(ports)
    }
}

fn parse_proc_net_tcp(content: &str, protocol: &str) -> Vec<PortInfo> {
    let mut ports = Vec::new();

    for line in content.lines().skip(1) {
        let parts: Vec<&str> = line.split_whitespace().collect();
        if parts.len() < 12 {
            continue;
        }

        let local_addr = parts[1];
        let remote_addr = parts[2];

        let port = match parse_hex_port(local_addr) {
            Some(p) => p,
            None => continue,
        };

        let state = parts[3];
        let status = parse_tcp_state(state);

        if status.is_none() {
            continue;
        }

        let uid = parts[7].parse::<u32>().unwrap_or(0);
        let user = get_username_from_uid(uid);

        let (local_addr_str, remote_addr_str) = parse_local_remote_addr(local_addr, remote_addr);

        ports.push(PortInfo {
            port,
            protocol: protocol.to_string(),
            pid: 0,
            process_name: String::new(),
            status: status.unwrap(),
            process_name_unknown: false,
            local_addr: local_addr_str,
            remote_addr: remote_addr_str,
            user,
        });
    }

    ports
}

fn parse_proc_net_udp(content: &str, protocol: &str) -> Vec<PortInfo> {
    let mut ports = Vec::new();

    for line in content.lines().skip(1) {
        let parts: Vec<&str> = line.split_whitespace().collect();
        if parts.len() < 12 {
            continue;
        }

        let local_addr = parts[1];
        let remote_addr = parts[2];

        let port = match parse_hex_port(local_addr) {
            Some(p) => p,
            None => continue,
        };

        let uid = parts[7].parse::<u32>().unwrap_or(0);
        let user = get_username_from_uid(uid);

        let (local_addr_str, remote_addr_str) = parse_local_remote_addr(local_addr, remote_addr);

        ports.push(PortInfo {
            port,
            protocol: protocol.to_string(),
            pid: 0,
            process_name: String::new(),
            status: "-".to_string(),
            process_name_unknown: false,
            local_addr: local_addr_str,
            remote_addr: remote_addr_str,
            user,
        });
    }

    ports
}

fn parse_tcp_state(state: &str) -> Option<String> {
    match state {
        "01" => Some("ESTABLISHED".to_string()),
        "02" => Some("SYN_SENT".to_string()),
        "03" => Some("SYN_RECV".to_string()),
        "04" => Some("FIN_WAIT1".to_string()),
        "05" => Some("FIN_WAIT2".to_string()),
        "06" => Some("TIME_WAIT".to_string()),
        "07" => Some("CLOSE".to_string()),
        "08" => Some("CLOSE_WAIT".to_string()),
        "09" => Some("LAST_ACK".to_string()),
        "0A" => Some("LISTEN".to_string()),
        "0B" => Some("CLOSING".to_string()),
        "0C" => Some("NEW_SYN_RECV".to_string()),
        _ => None,
    }
}

fn parse_hex_port(addr: &str) -> Option<u16> {
    let colon_pos = addr.find(':')?;
    let port_hex = &addr[colon_pos + 1..];
    u16::from_str_radix(port_hex, 16).ok()
}

fn enhance_ports_with_process_info(ports: &mut [PortInfo]) -> Result<(), PortError> {
    let _ = try_ss_command(ports);

    for port in ports.iter_mut() {
        if port.process_name.is_empty() {
            port.process_name = "unknown".to_string();
            port.process_name_unknown = true;
        }
    }

    Ok(())
}

fn try_ss_command(ports: &mut [PortInfo]) -> Result<(), PortError> {
    let output = run_command("ss", &["-tulpn"])?;

    let mut port_info_map: HashMap<u16, (u32, String, String)> = HashMap::new();

    for line in output.lines().skip(1) {
        if !line.contains("users:") {
            continue;
        }

        let parts: Vec<&str> = line.split_whitespace().collect();
        if parts.len() < 7 {
            continue;
        }

        let local_addr = parts[4];
        let port = match parse_decimal_port(local_addr) {
            Some(p) => p,
            None => continue,
        };

        let process_col = match parts.get(6) {
            Some(s) => s,
            None => continue,
        };

        let pid = extract_pid_from_process_info(process_col).unwrap_or(0);
        let name = extract_name_from_process_info(process_col);
        let user = extract_user_from_process_info(process_col);

        if pid > 0 || !name.is_empty() {
            port_info_map.insert(port, (pid, name, user));
        }
    }

    for port in ports.iter_mut() {
        if let Some((pid, name, user)) = port_info_map.get(&port.port) {
            port.pid = *pid;
            if !name.is_empty() {
                port.process_name = name.clone();
            }
            if !user.is_empty() && port.user.is_empty() {
                port.user = user.clone();
            }
        }
    }

    Ok(())
}

fn extract_pid_from_process_info(info: &str) -> Option<u32> {
    if let Some(pid_start) = info.find("pid=") {
        let pid_str = &info[pid_start + 4..];
        return pid_str.split(',').next()?.parse().ok();
    }
    None
}

fn extract_name_from_process_info(info: &str) -> String {
    if let Some(start) = info.find("(\"") {
        let rest = &info[start + 2..];
        if let Some(end) = rest.find('"') {
            return rest[..end].to_string();
        }
    }
    String::new()
}

fn extract_user_from_process_info(info: &str) -> String {
    if let Some(uid_start) = info.find("uid=") {
        let uid_str = &info[uid_start + 4..];
        if let Some(end) = uid_str.find(',') {
            if let Ok(uid) = uid_str[..end].parse::<u32>() {
                return get_username_from_uid(uid);
            }
        }
    }
    String::new()
}
