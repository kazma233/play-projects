mod image_processor;

use image_processor::{ImageMetadataResult, OutputFormat, ProcessImageResult, SavedImageResult};

#[tauri::command]
async fn process_image_preview(
    path: String,
    format: OutputFormat,
    quality: u8,
    scale: u32,
    webp_lossless: Option<bool>,
) -> Result<ProcessImageResult, String> {
    tauri::async_runtime::spawn_blocking(move || {
        image_processor::process_image_preview(
            &path,
            format,
            quality,
            scale,
            webp_lossless.unwrap_or(false),
        )
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
    webp_lossless: Option<bool>,
) -> Result<SavedImageResult, String> {
    tauri::async_runtime::spawn_blocking(move || {
        image_processor::process_and_save_image(
            &input_path,
            &output_dir,
            format,
            quality,
            scale,
            webp_lossless.unwrap_or(false),
        )
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn process_and_write_image(
    input_path: String,
    output_path: String,
    format: OutputFormat,
    quality: u8,
    scale: u32,
    webp_lossless: Option<bool>,
) -> Result<SavedImageResult, String> {
    tauri::async_runtime::spawn_blocking(move || {
        image_processor::process_and_write_image(
            &input_path,
            &output_path,
            format,
            quality,
            scale,
            webp_lossless.unwrap_or(false),
        )
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn load_original_preview(
    path: String,
) -> Result<image_processor::OriginalImageResult, String> {
    tauri::async_runtime::spawn_blocking(move || image_processor::load_original_preview(&path))
        .await
        .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn load_image_metadata(path: String) -> Result<ImageMetadataResult, String> {
    tauri::async_runtime::spawn_blocking(move || image_processor::load_image_metadata(&path))
        .await
        .map_err(|e| e.to_string())?
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_fs::init())
        .invoke_handler(tauri::generate_handler![
            process_image_preview,
            process_and_save_image,
            process_and_write_image,
            load_original_preview,
            load_image_metadata,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
