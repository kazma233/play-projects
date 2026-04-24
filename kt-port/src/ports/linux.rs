use super::types::{PortError, PortInfo, PortScanner};
use super::utils::{
    format_socket_addr, lookup_username_by_uid, normalize_protocol_label, normalize_socket_addr,
    run_command,
};
use std::collections::HashMap;
use std::net::{Ipv4Addr, Ipv6Addr};

pub struct LinuxScanner;

type UserCache = HashMap<u32, String>;
type SocketKey = (String, String, String);

fn decode_hex_bytes(hex: &str) -> Option<Vec<u8>> {
    if !hex.len().is_multiple_of(2) {
        return None;
    }

    (0..hex.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&hex[index..index + 2], 16).ok())
        .collect()
}

fn hex_to_ip(hex: &str) -> Option<String> {
    match hex.len() {
        8 => {
            let bytes = decode_hex_bytes(hex)?;
            let octets = [*bytes.get(3)?, *bytes.get(2)?, *bytes.get(1)?, *bytes.get(0)?];
            Some(Ipv4Addr::from(octets).to_string())
        }
        32 => {
            let bytes = decode_hex_bytes(hex)?;
            if bytes.len() != 16 {
                return None;
            }

            let mut normalized = [0_u8; 16];
            for (index, chunk) in bytes.chunks_exact(4).enumerate() {
                let start = index * 4;
                normalized[start] = chunk[3];
                normalized[start + 1] = chunk[2];
                normalized[start + 2] = chunk[1];
                normalized[start + 3] = chunk[0];
            }

            Some(Ipv6Addr::from(normalized).to_string())
        }
        _ => None,
    }
}

fn parse_local_remote_addr(local: &str, remote: &str) -> (String, String) {
    (parse_hex_addr(local), parse_hex_addr(remote))
}

fn parse_hex_addr(addr: &str) -> String {
    let Some((ip_hex, port_hex)) = addr.split_once(':') else {
        return addr.to_string();
    };

    let Some(ip) = hex_to_ip(ip_hex) else {
        return addr.to_string();
    };

    let Ok(port) = u16::from_str_radix(port_hex, 16) else {
        return addr.to_string();
    };

    format_socket_addr(&ip, port)
}

fn get_username_from_uid(uid: u32, cache: &mut UserCache) -> String {
    if let Some(username) = cache.get(&uid) {
        return username.clone();
    }

    let username = lookup_username_by_uid(uid).or_else(|| {
        let passwd = std::fs::read_to_string("/etc/passwd").ok()?;
        passwd.lines().find_map(|line| {
            let parts: Vec<&str> = line.split(':').collect();
            if parts.len() >= 3 && parts[2].parse::<u32>().ok()? == uid {
                Some(parts[0].to_string())
            } else {
                None
            }
        })
    });

    let username = username.unwrap_or_else(|| uid.to_string());
    cache.insert(uid, username.clone());
    username
}

impl PortScanner for LinuxScanner {
    fn scan(&self) -> Result<Vec<PortInfo>, PortError> {
        let mut ports = Vec::new();
        let mut user_cache = HashMap::new();

        if let Ok(tcp_content) = std::fs::read_to_string("/proc/net/tcp") {
            ports.extend(parse_proc_net_tcp(&tcp_content, "TCP", &mut user_cache));
        }

        if let Ok(tcp6_content) = std::fs::read_to_string("/proc/net/tcp6") {
            ports.extend(parse_proc_net_tcp(&tcp6_content, "TCP", &mut user_cache));
        }

        if let Ok(udp_content) = std::fs::read_to_string("/proc/net/udp") {
            ports.extend(parse_proc_net_udp(&udp_content, "UDP", &mut user_cache));
        }

        if let Ok(udp6_content) = std::fs::read_to_string("/proc/net/udp6") {
            ports.extend(parse_proc_net_udp(&udp6_content, "UDP", &mut user_cache));
        }

        if !ports.is_empty() {
            enhance_ports_with_process_info(&mut ports, &mut user_cache)?;
        }

        Ok(ports)
    }
}

fn parse_proc_net_tcp(content: &str, protocol: &str, user_cache: &mut UserCache) -> Vec<PortInfo> {
    let mut ports = Vec::new();

    for line in content.lines().skip(1) {
        let parts: Vec<&str> = line.split_whitespace().collect();
        if parts.len() < 12 {
            continue;
        }

        let local_addr = parts[1];
        let remote_addr = parts[2];

        let Some(port) = parse_hex_port(local_addr) else {
            continue;
        };

        let Some(status) = parse_tcp_state(parts[3]) else {
            continue;
        };

        let uid = parts[7].parse::<u32>().unwrap_or(0);
        let user = get_username_from_uid(uid, user_cache);
        let (local_addr, remote_addr) = parse_local_remote_addr(local_addr, remote_addr);

        ports.push(PortInfo {
            port,
            protocol: protocol.to_string(),
            pid: 0,
            process_name: String::new(),
            status,
            process_name_unknown: false,
            local_addr,
            remote_addr,
            user,
        });
    }

    ports
}

fn parse_proc_net_udp(content: &str, protocol: &str, user_cache: &mut UserCache) -> Vec<PortInfo> {
    let mut ports = Vec::new();

    for line in content.lines().skip(1) {
        let parts: Vec<&str> = line.split_whitespace().collect();
        if parts.len() < 12 {
            continue;
        }

        let local_addr = parts[1];
        let remote_addr = parts[2];

        let Some(port) = parse_hex_port(local_addr) else {
            continue;
        };

        let uid = parts[7].parse::<u32>().unwrap_or(0);
        let user = get_username_from_uid(uid, user_cache);
        let (local_addr, remote_addr) = parse_local_remote_addr(local_addr, remote_addr);

        ports.push(PortInfo {
            port,
            protocol: protocol.to_string(),
            pid: 0,
            process_name: String::new(),
            status: String::new(),
            process_name_unknown: false,
            local_addr,
            remote_addr,
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
    let port_hex = addr.split_once(':')?.1;
    u16::from_str_radix(port_hex, 16).ok()
}

fn normalize_remote_addr_for_match(addr: &str) -> String {
    match normalize_socket_addr(addr).as_str() {
        "*:*" | "0.0.0.0:0" | "0.0.0.0:*" | "[::]:0" | "[::]:*" => "*:*".to_string(),
        normalized => normalized.to_string(),
    }
}

fn make_socket_key(protocol: &str, local_addr: &str, remote_addr: &str) -> SocketKey {
    (
        normalize_protocol_label(protocol),
        normalize_socket_addr(local_addr),
        normalize_remote_addr_for_match(remote_addr),
    )
}

fn enhance_ports_with_process_info(
    ports: &mut [PortInfo],
    user_cache: &mut UserCache,
) -> Result<(), PortError> {
    let _ = try_ss_command(ports, user_cache);

    for port in ports.iter_mut() {
        if port.process_name.is_empty() {
            port.process_name_unknown = true;
        }
    }

    Ok(())
}

fn try_ss_command(ports: &mut [PortInfo], user_cache: &mut UserCache) -> Result<(), PortError> {
    let output = run_command("ss", &["-tunap"])?;
    let mut port_info_map: HashMap<SocketKey, (u32, String, String)> = HashMap::new();

    for line in output.lines().skip(1) {
        let trimmed = line.trim();
        if trimmed.is_empty() {
            continue;
        }

        let parts: Vec<&str> = trimmed.split_whitespace().collect();
        if parts.len() < 6 {
            continue;
        }

        let protocol = normalize_protocol_label(parts[0]);
        if protocol != "TCP" && protocol != "UDP" {
            continue;
        }

        let local_addr = parts[4];
        let remote_addr = parts[5];
        let process_info = if parts.len() > 6 {
            parts[6..].join(" ")
        } else {
            String::new()
        };

        let pid = extract_pid_from_process_info(&process_info).unwrap_or(0);
        let name = extract_name_from_process_info(&process_info);
        let user = extract_user_from_process_info(&process_info, user_cache);

        if pid == 0 && name.is_empty() {
            continue;
        }

        port_info_map.insert(
            make_socket_key(&protocol, local_addr, remote_addr),
            (pid, name, user),
        );
    }

    for port in ports.iter_mut() {
        if let Some((pid, name, user)) = port_info_map.get(&make_socket_key(
            &port.protocol,
            &port.local_addr,
            &port.remote_addr,
        )) {
            if *pid > 0 {
                port.pid = *pid;
            }
            if !name.is_empty() {
                port.process_name = name.clone();
            }
            if !user.is_empty() {
                port.user = user.clone();
            }
        }
    }

    Ok(())
}

fn extract_pid_from_process_info(info: &str) -> Option<u32> {
    let pid_start = info.find("pid=")?;
    let pid = &info[pid_start + 4..];
    pid.split([',', ')']).next()?.parse().ok()
}

fn extract_name_from_process_info(info: &str) -> String {
    let Some(start) = info.find("(\"") else {
        return String::new();
    };

    let rest = &info[start + 2..];
    let Some(end) = rest.find('"') else {
        return String::new();
    };

    rest[..end].to_string()
}

fn extract_user_from_process_info(info: &str, user_cache: &mut UserCache) -> String {
    let Some(uid_start) = info.find("uid=") else {
        return String::new();
    };

    let uid = &info[uid_start + 4..];
    let Some(uid) = uid
        .split([',', ')'])
        .next()
        .and_then(|value| value.parse::<u32>().ok())
    else {
        return String::new();
    };

    get_username_from_uid(uid, user_cache)
}
