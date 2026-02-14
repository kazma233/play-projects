use chrono::SecondsFormat;
use chrono::{DateTime, Duration, Months, NaiveDateTime, TimeZone};
use serde::Serialize;

#[derive(Serialize)]
struct DateTimeResult {
    timestamp: i64, // 毫秒
    readable: String,
    rfc3339: String,
    match_format: String,
}

// Helper function to create a standardized response
fn create_datetime_response(dt: DateTime<chrono_tz::Tz>, format: &str) -> Result<String, String> {
    let result = DateTimeResult {
        timestamp: dt.timestamp_millis(),
        rfc3339: dt.to_rfc3339_opts(SecondsFormat::AutoSi, true),
        readable: dt.format("%Y-%m-%d %H:%M:%S%.f %:z").to_string(),
        match_format: String::from(format),
    };
    serde_json::to_string(&result).map_err(|e| e.to_string())
}

#[tauri::command]
pub fn exchange_date(input: &str, timezone: &str) -> Result<String, String> {
    let tz: chrono_tz::Tz = timezone.parse().unwrap_or(chrono_tz::Tz::UTC);

    // 尝试解析时间戳 (秒或毫秒)
    if let Ok(timestamp) = input.parse::<i64>() {
        // 首先检查是否是毫秒级时间戳 (通常是13位)
        if timestamp > 1_000_000_000_000 {
            // 毫秒级时间戳
            if let Some(datetime) = DateTime::from_timestamp(timestamp / 1000, 0) {
                let naive_utc = datetime.naive_utc();
                let target_dt = tz.from_utc_datetime(&naive_utc);
                return create_datetime_response(target_dt, "ms");
            }
        }
        // 尝试秒级时间戳
        if let Some(datetime) = DateTime::from_timestamp(timestamp, 0) {
            let naive_utc = datetime.naive_utc();
            let target_dt = tz.from_utc_datetime(&naive_utc);
            return create_datetime_response(target_dt, "s");
        }
    }

    // 常见日期时间格式
    let formats = [
        "%Y-%m-%d %H:%M:%S",
        "%Y-%m-%d %H:%M:%SZ",
        "%Y-%m-%d %H:%M:%S%.f",
        "%Y-%m-%d %H:%M:%S%.fZ",
        "%Y/%m/%d %H:%M:%S",
        "%Y%m%d %H%M%S",
        "%Y-%m-%d",
        "%Y/%m/%d",
        "%Y%m%d",
        "%Y年%m月%d日 %H时%M分%S秒",
        "%Y年%m月%d日",
        "%Y.%m.%d %H:%M:%S",
        "%d.%m.%Y %H:%M:%S",
        "%m/%d/%Y %H:%M:%S",
        "%d-%b-%Y %H:%M:%S",
        "%a %b %e %T %Y", // ctime format
    ];

    // 尝试解析各种格式
    for &format in &formats {
        if let Ok(datetime) = DateTime::parse_from_str(input, format) {
            let target_dt = datetime.with_timezone(&tz);
            return create_datetime_response(target_dt, &("DateTime: ".to_owned() + format));
        }
        if let Ok(naive_dt) = NaiveDateTime::parse_from_str(input, format) {
            let target_dt = tz.from_utc_datetime(&naive_dt);
            return create_datetime_response(target_dt, &("NaiveDateTime: ".to_owned() + format));
        }
        if let Ok(naive_date) = chrono::NaiveDate::parse_from_str(input, format) {
            let target_dt = naive_date
                .and_hms_opt(0, 0, 0)
                .unwrap()
                .and_local_timezone(tz)
                .unwrap();
            return create_datetime_response(target_dt, &("NaiveDate: ".to_owned() + format));
        }
    }

    // 尝试 RFC 2822 格式: Wed, 18 Feb 2015 23:16:09 GMT
    if let Ok(datetime) = DateTime::parse_from_rfc2822(input) {
        let target_dt = datetime.with_timezone(&tz);
        return create_datetime_response(target_dt, "rfc2822");
    }

    // 尝试 RFC 3339 / ISO 8601 格式 1996-12-19T16:39:57-08:00
    if let Ok(datetime) = DateTime::parse_from_rfc3339(input) {
        let target_dt = datetime.with_timezone(&tz);
        return create_datetime_response(target_dt, "rfc3339");
    }

    // 如果所有解析都失败，返回错误信息
    Err(format!("无法解析日期时间: {}", input))
}

#[tauri::command]
pub fn calc_date(
    rfc3339: &str,
    time_value: i64,
    time_unit: &str,
    timezone: &str,
) -> Result<String, String> {
    let tz: chrono_tz::Tz = timezone.parse().unwrap_or(chrono_tz::Tz::UTC);
    // 1. 解析并保持 timezone
    let dt_utc = DateTime::parse_from_rfc3339(rfc3339)
        .map_err(|e| format!("无法解析RFC 3339时间: {}", e))?
        .with_timezone(&tz);

    // 2. 统一处理：先尝试 i64 → i32，溢出直接报错
    let value = i32::try_from(time_value).map_err(|_| "数值过大，无法处理".to_string())?;

    // 3. 单分支计算
    let new_dt_utc = match time_unit {
        "seconds" => dt_utc + Duration::seconds(value as i64),
        "minutes" => dt_utc + Duration::minutes(value as i64),
        "hours" => dt_utc + Duration::hours(value as i64),
        "days" => dt_utc + Duration::days(value as i64),
        "weeks" => dt_utc + Duration::weeks(value as i64),
        // chrono 已内置月份/年份加减，自动处理闰年、月底越界
        "months" => {
            let months = Months::new(value.unsigned_abs());
            if value >= 0 {
                dt_utc.checked_add_months(months)
            } else {
                dt_utc.checked_sub_months(months)
            }
            .ok_or("月份运算溢出")?
        }
        "years" => {
            let months = Months::new(value.unsigned_abs() * 12);
            if value >= 0 {
                dt_utc.checked_add_months(months)
            } else {
                dt_utc.checked_sub_months(months)
            }
            .ok_or("年份运算溢出")?
        }

        _ => return Err(format!("未知的时间单位: {}", time_unit)),
    };

    // 4. 转回 chrono_tz::Tz，满足 create_datetime_response 的签名
    let new_dt_tz = new_dt_utc.with_timezone(&tz);

    create_datetime_response(new_dt_tz, "calculated")
}
