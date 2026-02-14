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
        return Err(PortError::CommandFailed {
            cmd: cmd.to_string(),
            reason: "command returned non-zero exit code".to_string(),
        });
    }

    Ok(String::from_utf8_lossy(&output.stdout).to_string())
}
