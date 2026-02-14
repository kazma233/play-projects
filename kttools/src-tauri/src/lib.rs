use base64;
use image::{ImageBuffer, ImageFormat, Rgb, Rgba, RgbaImage};
use md5;
use qrcode::{QrCode, render::svg};
use sha1::{Digest, Sha1};
use std::fs::File;
use std::io::{BufReader, Cursor, Read};
use tauri::image::Image;
use tauri_plugin_clipboard_manager::ClipboardExt;

mod datetime;
mod image_processor;
mod ports;

use image_processor::{OutputFormat, ProcessResult};

#[tauri::command]
fn base64_encode(input: &str, url_mode: bool) -> String {
    use base64::Engine as _;
    if url_mode {
        base64::engine::general_purpose::URL_SAFE.encode(input)
    } else {
        base64::engine::general_purpose::STANDARD.encode(input)
    }
}

#[tauri::command]
fn base64_decode(input: &str) -> Result<String, String> {
    use base64::Engine as _;
    String::from_utf8(
        base64::engine::general_purpose::STANDARD
            .decode(input)
            .unwrap_or_default(),
    )
    .map_err(|e| e.to_string())
}

#[tauri::command]
fn url_encode(input: &str) -> String {
    urlencoding::encode(input).into_owned()
}

#[tauri::command]
fn url_decode(input: &str) -> Result<String, String> {
    urlencoding::decode(input)
        .map(|s| s.into_owned())
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn md5_encode(input: &str) -> String {
    format!("{:x}", md5::compute(input))
}

#[tauri::command]
async fn sha1_encode(file_path: String) -> Result<String, String> {
    tauri::async_runtime::spawn_blocking(move || {
        let file = File::open(file_path).map_err(|e| e.to_string())?;
        let mut reader = BufReader::new(file);
        let mut hasher = Sha1::new();
        let mut buffer = [0; 8192]; // 8KB buffer

        loop {
            let bytes_read = reader.read(&mut buffer).map_err(|e| e.to_string())?;
            if bytes_read == 0 {
                break;
            }
            hasher.update(&buffer[..bytes_read]);
        }

        let result = hasher.finalize();
        Ok(format!("{:x}", result))
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
fn generate_qr_code(
    content: String,
    size: u32,
    dark_color: &str,
    light_color: &str,
) -> Result<String, String> {
    let code = QrCode::new(content.as_bytes())
        .map_err(|e| format!("Failed to generate QR code: {}", e))?;

    let image = code
        .render()
        .min_dimensions(size, size)
        .dark_color(svg::Color(dark_color))
        .light_color(svg::Color(light_color))
        .build();

    Ok(image)
}

// 解析颜色字符串 (支持 #RRGGBB 格式)
fn parse_hex_color(color: &str) -> Result<(u8, u8, u8), String> {
    let color = color.trim_start_matches('#');
    if color.len() < 6 {
        return Err("Invalid color format".to_string());
    }
    let r = u8::from_str_radix(&color[0..2], 16).map_err(|e| e.to_string())?;
    let g = u8::from_str_radix(&color[2..4], 16).map_err(|e| e.to_string())?;
    let b = u8::from_str_radix(&color[4..6], 16).map_err(|e| e.to_string())?;
    Ok((r, g, b))
}

// 生成 PNG 格式的二维码图片数据
#[tauri::command]
fn generate_qr_code_png(
    content: String,
    size: u32,
    dark_color: &str,
    light_color: &str,
) -> Result<Vec<u8>, String> {
    let code = QrCode::new(content.as_bytes())
        .map_err(|e| format!("Failed to generate QR code: {}", e))?;

    let (dark_r, dark_g, dark_b) = parse_hex_color(dark_color)?;
    let (light_r, light_g, light_b) = parse_hex_color(light_color)?;

    // 使用 qrcode 的 image 渲染器直接生成 RGB 图像
    let qr_image = code
        .render::<Rgb<u8>>()
        .min_dimensions(size, size)
        .max_dimensions(size, size)
        .dark_color(Rgb([dark_r, dark_g, dark_b]))
        .light_color(Rgb([light_r, light_g, light_b]))
        .build();

    // 添加 padding (20px)
    let padding = 20u32;
    let qr_width = qr_image.width();
    let qr_height = qr_image.height();
    let total_width = qr_width + padding * 2;
    let total_height = qr_height + padding * 2;

    // 创建带 padding 的 RGBA 图像
    let mut final_img: RgbaImage = ImageBuffer::from_pixel(
        total_width,
        total_height,
        Rgba([light_r, light_g, light_b, 255]),
    );

    // 将二维码复制到中心
    for y in 0..qr_height {
        for x in 0..qr_width {
            let pixel = qr_image.get_pixel(x, y);
            final_img.put_pixel(
                padding + x,
                padding + y,
                Rgba([pixel[0], pixel[1], pixel[2], 255]),
            );
        }
    }

    // 编码为 PNG
    let mut buffer = Cursor::new(Vec::new());
    final_img
        .write_to(&mut buffer, ImageFormat::Png)
        .map_err(|e| format!("Failed to encode PNG: {}", e))?;

    Ok(buffer.into_inner())
}

// 复制二维码到剪贴板 (后端实现，绕过前端图片加载问题)
#[tauri::command]
async fn copy_qr_code_to_clipboard(
    app: tauri::AppHandle,
    content: String,
    size: u32,
    dark_color: &str,
    light_color: &str,
) -> Result<(), String> {
    // 生成 PNG 数据
    let png_data = generate_qr_code_png(content, size, dark_color, light_color)?;

    // 加载为 Tauri Image
    let image = Image::from_bytes(&png_data).map_err(|e| e.to_string())?;

    // 写入剪贴板
    let clipboard = app.clipboard();
    clipboard.write_image(&image).map_err(|e| e.to_string())?;

    Ok(())
}

#[tauri::command]
fn get_port_list() -> Result<Vec<ports::PortInfo>, String> {
    ports::get_port_list().map_err(|e| e.to_string())
}

#[tauri::command]
async fn kill_process(port: u16) -> Result<String, String> {
    let ports_list = ports::get_port_list().map_err(|e| e.to_string())?;
    ports::kill_process(port, &ports_list)
        .await
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn get_image_info(path: String) -> Result<image_processor::ImageInfo, String> {
    image_processor::get_image_info(&path)
}

#[tauri::command]
async fn process_image(
    path: String,
    format: OutputFormat,
    quality: u8,
    scale: u32,
) -> Result<ProcessResult, String> {
    tauri::async_runtime::spawn_blocking(move || {
        image_processor::process_image(&path, format, quality, scale)
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn process_and_save_image(
    input_path: String,
    output_dir: String,
    format: OutputFormat,
    quality: u8,
    scale: u32,
) -> Result<(String, Vec<u8>), String> {
    tauri::async_runtime::spawn_blocking(move || {
        image_processor::process_and_save_image(&input_path, &output_dir, format, quality, scale)
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn load_original_image(path: String) -> Result<image_processor::OriginalImageResult, String> {
    tauri::async_runtime::spawn_blocking(move || {
        image_processor::load_original_image(&path)
    })
    .await
    .map_err(|e| e.to_string())?
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_fs::init())
        .plugin(tauri_plugin_clipboard_manager::init())
        .invoke_handler(tauri::generate_handler![
            datetime::exchange_date,
            datetime::calc_date,
            base64_encode,
            base64_decode,
            url_encode,
            url_decode,
            md5_encode,
            sha1_encode,
            generate_qr_code,
            generate_qr_code_png,
            copy_qr_code_to_clipboard,
            get_port_list,
            kill_process,
            get_image_info,
            process_image,
            process_and_save_image,
            load_original_image,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
