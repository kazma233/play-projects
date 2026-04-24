use super::types::PortError;
use std::process::Command;

pub fn run_command(cmd: &str, args: &[&str]) -> Result<String, PortError> {
    let output = Command::new(cmd)
        .args(args)
        .output()
        .map_err(|e| PortError::CommandFailed {
            cmd: cmd.to_string(),
            reason: e.to_string(),
        })?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
        return Err(PortError::CommandFailed {
            cmd: cmd.to_string(),
            reason: if stderr.is_empty() {
                "command returned non-zero exit code".to_string()
            } else {
                stderr
            },
        });
    }

    Ok(String::from_utf8_lossy(&output.stdout).to_string())
}

pub fn lookup_username_by_uid(uid: u32) -> Option<String> {
    let output = Command::new("id")
        .arg("-nu")
        .arg(uid.to_string())
        .output()
        .ok()?;

    if !output.status.success() {
        return None;
    }

    let username = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if username.is_empty() {
        None
    } else {
        Some(username)
    }
}

pub fn normalize_protocol_label(protocol: &str) -> String {
    let upper = protocol.trim().to_ascii_uppercase();
    if upper.starts_with("TCP") {
        "TCP".to_string()
    } else if upper.starts_with("UDP") {
        "UDP".to_string()
    } else {
        upper
    }
}

pub fn format_socket_addr(host: &str, port: u16) -> String {
    if host.contains(':') && !host.starts_with('[') {
        format!("[{}]:{}", host, port)
    } else {
        format!("{}:{}", host, port)
    }
}

pub fn normalize_socket_addr(addr: &str) -> String {
    let trimmed = addr.trim();
    if trimmed.is_empty() {
        return String::new();
    }

    if trimmed == "*:*" {
        return trimmed.to_string();
    }

    if let Some(rest) = trimmed.strip_prefix('[') {
        if let Some(position) = rest.rfind("]:") {
            let host = &rest[..position];
            let port = &rest[position + 2..];
            if let Ok(port) = port.parse::<u16>() {
                return format_socket_addr(&host.to_ascii_lowercase(), port);
            }
        }
    }

    if let Some((host, port)) = trimmed.rsplit_once(':') {
        if let Ok(port) = port.parse::<u16>() {
            let normalized_host = match host.trim() {
                "*" => "0.0.0.0".to_string(),
                other => other.to_ascii_lowercase(),
            };
            return format_socket_addr(&normalized_host, port);
        }
    }

    trimmed.to_ascii_lowercase()
}
