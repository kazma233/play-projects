use image::codecs::png::PngEncoder;
use image::{GenericImageView, ImageEncoder, ImageFormat};
use std::fs;
use std::io::Cursor;
use std::path::PathBuf;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, serde::Deserialize)]
pub enum OutputFormat {
    #[default]
    Png,
    Jpg,
    WebP,
}

impl OutputFormat {
    pub fn to_str(&self) -> &str {
        match self {
            OutputFormat::Png => "png",
            OutputFormat::Jpg => "jpg",
            OutputFormat::WebP => "webp",
        }
    }
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ImageInfo {
    pub path: String,
    pub name: String,
    pub size: u64,
    pub width: u32,
    pub height: u32,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ProcessResult {
    pub data: Vec<u8>,
    pub width: u32,
    pub height: u32,
    pub original_size: u64,
    pub processed_size: usize,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct OriginalImageResult {
    pub image_data: Vec<u8>,
    pub original_size: u64,
    pub width: u32,
    pub height: u32,
}

pub fn load_original_image(path: &str) -> Result<OriginalImageResult, String> {
    let path_buf = PathBuf::from(path);

    if !path_buf.exists() {
        return Err("File does not exist".to_string());
    }

    let original_size = std::fs::metadata(&path_buf)
        .map_err(|e| format!("Failed to get file size: {}", e))?
        .len();

    // 读取原图数据
    let image_data = std::fs::read(&path_buf).map_err(|e| format!("Failed to read file: {}", e))?;

    // 获取图片尺寸
    let (width, height) = if let Ok(size) = imagesize::size(&path_buf) {
        (size.width as u32, size.height as u32)
    } else if let Ok(img) = image::open(&path_buf) {
        img.dimensions()
    } else {
        return Err("Failed to get image dimensions".to_string());
    };

    Ok(OriginalImageResult {
        image_data,
        original_size,
        width,
        height,
    })
}

fn encode_image(
    img: &image::DynamicImage,
    format: OutputFormat,
    quality: u8,
    scale: u32,
) -> Result<ProcessResult, String> {
    let (original_width, original_height) = img.dimensions();

    let scaled_width = ((original_width as f32 * scale as f32 / 100.0) as u32).max(1);
    let scaled_height = ((original_height as f32 * scale as f32 / 100.0) as u32).max(1);

    let resized = if scale != 100 {
        img.resize(
            scaled_width,
            scaled_height,
            image::imageops::FilterType::Lanczos3,
        )
    } else {
        img.clone()
    };

    let mut buffer = Cursor::new(Vec::new());

    match format {
        OutputFormat::WebP => {
            let rgba = resized.to_rgba8();
            rgba.write_to(&mut buffer, ImageFormat::WebP)
                .map_err(|e| format!("Failed to write WebP: {}", e))?;
        }
        OutputFormat::Png => {
            let compression_level = ((quality as f32 / 100.0) * 9.0) as u8;
            let rgba = resized.to_rgba8();

            let compression_type = match compression_level {
                0..=3 => image::codecs::png::CompressionType::Fast,
                4..=6 => image::codecs::png::CompressionType::Default,
                _ => image::codecs::png::CompressionType::Best,
            };

            let encoder = PngEncoder::new_with_quality(
                &mut buffer,
                compression_type,
                image::codecs::png::FilterType::Adaptive,
            );

            encoder
                .write_image(
                    &rgba,
                    rgba.width(),
                    rgba.height(),
                    image::ExtendedColorType::Rgba8,
                )
                .map_err(|e| format!("Failed to write PNG: {}", e))?;
        }
        OutputFormat::Jpg => {
            let rgb = resized.to_rgb8();
            rgb.write_to(&mut buffer, ImageFormat::Jpeg)
                .map_err(|e| format!("Failed to write JPEG: {}", e))?;
        }
    }

    let data = buffer.into_inner();
    let processed_size = data.len();

    Ok(ProcessResult {
        data,
        width: scaled_width,
        height: scaled_height,
        original_size: (original_width * original_height * 4) as u64,
        processed_size,
    })
}

pub fn get_image_info(path: &str) -> Result<ImageInfo, String> {
    let path_buf = PathBuf::from(path);

    if !path_buf.exists() {
        return Err("File does not exist".to_string());
    }

    let metadata =
        std::fs::metadata(&path_buf).map_err(|e| format!("Failed to get file metadata: {}", e))?;

    let name = path_buf
        .file_name()
        .and_then(|n| n.to_str())
        .unwrap_or("unknown")
        .to_string();

    let img = image::open(&path_buf).map_err(|e| format!("Failed to open image: {}", e))?;

    let (width, height) = img.dimensions();

    Ok(ImageInfo {
        path: path.to_string(),
        name,
        size: metadata.len(),
        width,
        height,
    })
}

pub fn process_image(
    path: &str,
    format: OutputFormat,
    quality: u8,
    scale: u32,
) -> Result<ProcessResult, String> {
    let path_buf = PathBuf::from(path);

    let original_size = std::fs::metadata(&path_buf)
        .map_err(|e| format!("Failed to get file size: {}", e))?
        .len();

    let img = image::open(&path_buf).map_err(|e| format!("Failed to open image: {}", e))?;

    let mut result = encode_image(&img, format, quality, scale)?;
    result.original_size = original_size;

    Ok(result)
}

pub fn process_and_save_image(
    input_path: &str,
    output_dir: &str,
    format: OutputFormat,
    quality: u8,
    scale: u32,
) -> Result<(String, Vec<u8>), String> {
    let input_path_buf = PathBuf::from(input_path);
    let output_dir_buf = PathBuf::from(output_dir);

    let img = image::open(&input_path_buf).map_err(|e| format!("Failed to open image: {}", e))?;

    let result = encode_image(&img, format, quality, scale)?;

    let file_stem = input_path_buf
        .file_stem()
        .and_then(|n| n.to_str())
        .unwrap_or("unknown");

    let output_file_name = format!("{}_compress.{}", file_stem, format.to_str());
    let output_path = output_dir_buf.join(&output_file_name);

    fs::write(&output_path, &result.data).map_err(|e| format!("Failed to write file: {}", e))?;

    Ok((output_path.to_string_lossy().to_string(), result.data))
}
