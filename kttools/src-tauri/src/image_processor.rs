use image::codecs::jpeg::JpegEncoder;
use image::codecs::png::PngEncoder;
use image::codecs::webp::WebPEncoder;
use image::{ExtendedColorType, GenericImageView, ImageEncoder};
use std::fs;
use std::io::Cursor;
use std::path::PathBuf;

const PREVIEW_MAX_EDGE: u32 = 1600;
const PREVIEW_JPEG_QUALITY: u8 = 80;

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
    pub preview_data: Vec<u8>,
    pub preview_mime_type: String,
    pub original_size: u64,
    pub width: u32,
    pub height: u32,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct SavedImageResult {
    pub output_path: String,
    pub width: u32,
    pub height: u32,
    pub processed_size: usize,
}

fn scaled_dimensions(width: u32, height: u32, scale: u32) -> (u32, u32) {
    (
        ((width as f32 * scale as f32 / 100.0) as u32).max(1),
        ((height as f32 * scale as f32 / 100.0) as u32).max(1),
    )
}

fn fit_dimensions(width: u32, height: u32, max_edge: u32) -> (u32, u32) {
    let longest_edge = width.max(height);

    if longest_edge <= max_edge {
        return (width, height);
    }

    let ratio = max_edge as f32 / longest_edge as f32;
    (
        ((width as f32 * ratio).round() as u32).max(1),
        ((height as f32 * ratio).round() as u32).max(1),
    )
}

fn resize_if_needed(
    img: &image::DynamicImage,
    width: u32,
    height: u32,
    filter: image::imageops::FilterType,
) -> Option<image::DynamicImage> {
    let (current_width, current_height) = img.dimensions();

    if current_width == width && current_height == height {
        None
    } else {
        Some(img.resize(width, height, filter))
    }
}

fn encode_preview_image(img: &image::DynamicImage) -> Result<(Vec<u8>, String), String> {
    let (preview_width, preview_height) =
        fit_dimensions(img.width(), img.height(), PREVIEW_MAX_EDGE);
    let resized = resize_if_needed(
        img,
        preview_width,
        preview_height,
        image::imageops::FilterType::Triangle,
    );
    let source = resized.as_ref().unwrap_or(img);
    let mut buffer = Cursor::new(Vec::new());

    if source.color().has_alpha() {
        let rgba = source.to_rgba8();

        WebPEncoder::new_lossless(&mut buffer)
            .encode(&rgba, rgba.width(), rgba.height(), ExtendedColorType::Rgba8)
            .map_err(|e| format!("Failed to write preview WebP: {}", e))?;

        Ok((buffer.into_inner(), "image/webp".to_string()))
    } else {
        let rgb = source.to_rgb8();
        let mut encoder = JpegEncoder::new_with_quality(&mut buffer, PREVIEW_JPEG_QUALITY);

        encoder
            .encode(&rgb, rgb.width(), rgb.height(), ExtendedColorType::Rgb8)
            .map_err(|e| format!("Failed to write preview JPEG: {}", e))?;

        Ok((buffer.into_inner(), "image/jpeg".to_string()))
    }
}

fn encode_webp_image(
    img: &image::DynamicImage,
    quality: u8,
    lossless: bool,
) -> Result<Vec<u8>, String> {
    if img.color().has_alpha() {
        let rgba = img.to_rgba8();
        let encoder = webp::Encoder::from_rgba(&rgba, rgba.width(), rgba.height());

        Ok(if lossless {
            encoder.encode_lossless().to_vec()
        } else {
            encoder.encode(quality as f32).to_vec()
        })
    } else {
        let rgb = img.to_rgb8();
        let encoder = webp::Encoder::from_rgb(&rgb, rgb.width(), rgb.height());

        Ok(if lossless {
            encoder.encode_lossless().to_vec()
        } else {
            encoder.encode(quality as f32).to_vec()
        })
    }
}

pub fn load_original_image(path: &str) -> Result<OriginalImageResult, String> {
    let path_buf = PathBuf::from(path);

    if !path_buf.exists() {
        return Err("File does not exist".to_string());
    }

    let original_size = std::fs::metadata(&path_buf)
        .map_err(|e| format!("Failed to get file size: {}", e))?
        .len();

    let img = image::open(&path_buf).map_err(|e| format!("Failed to open image: {}", e))?;
    let (width, height) = if let Ok(size) = imagesize::size(&path_buf) {
        (size.width as u32, size.height as u32)
    } else {
        img.dimensions()
    };
    let (preview_data, preview_mime_type) = encode_preview_image(&img)?;

    Ok(OriginalImageResult {
        preview_data,
        preview_mime_type,
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
    webp_lossless: bool,
) -> Result<ProcessResult, String> {
    let (original_width, original_height) = img.dimensions();
    let quality = quality.clamp(1, 100);

    let (scaled_width, scaled_height) = scaled_dimensions(original_width, original_height, scale);

    let resized = resize_if_needed(
        img,
        scaled_width,
        scaled_height,
        image::imageops::FilterType::CatmullRom,
    );
    let source = resized.as_ref().unwrap_or(img);

    let mut buffer = Cursor::new(Vec::new());

    match format {
        OutputFormat::WebP => {
            buffer = Cursor::new(encode_webp_image(source, quality, webp_lossless)?);
        }
        OutputFormat::Png => {
            let compression_type = match quality {
                1..=40 => image::codecs::png::CompressionType::Fast,
                41..=89 => image::codecs::png::CompressionType::Default,
                _ => image::codecs::png::CompressionType::Best,
            };

            if source.color().has_alpha() {
                let rgba = source.to_rgba8();

                PngEncoder::new_with_quality(
                    &mut buffer,
                    compression_type,
                    image::codecs::png::FilterType::Adaptive,
                )
                .write_image(&rgba, rgba.width(), rgba.height(), ExtendedColorType::Rgba8)
                .map_err(|e| format!("Failed to write PNG: {}", e))?;
            } else {
                let rgb = source.to_rgb8();

                PngEncoder::new_with_quality(
                    &mut buffer,
                    compression_type,
                    image::codecs::png::FilterType::Adaptive,
                )
                .write_image(&rgb, rgb.width(), rgb.height(), ExtendedColorType::Rgb8)
                .map_err(|e| format!("Failed to write PNG: {}", e))?;
            }
        }
        OutputFormat::Jpg => {
            let rgb = source.to_rgb8();
            let mut encoder = JpegEncoder::new_with_quality(&mut buffer, quality);

            encoder
                .encode(&rgb, rgb.width(), rgb.height(), ExtendedColorType::Rgb8)
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
    webp_lossless: bool,
) -> Result<ProcessResult, String> {
    let path_buf = PathBuf::from(path);

    let original_size = std::fs::metadata(&path_buf)
        .map_err(|e| format!("Failed to get file size: {}", e))?
        .len();

    let img = image::open(&path_buf).map_err(|e| format!("Failed to open image: {}", e))?;

    let mut result = encode_image(&img, format, quality, scale, webp_lossless)?;
    result.original_size = original_size;

    Ok(result)
}

pub fn process_and_save_image(
    input_path: &str,
    output_dir: &str,
    format: OutputFormat,
    quality: u8,
    scale: u32,
    webp_lossless: bool,
) -> Result<SavedImageResult, String> {
    let input_path_buf = PathBuf::from(input_path);
    let output_dir_buf = PathBuf::from(output_dir);

    let img = image::open(&input_path_buf).map_err(|e| format!("Failed to open image: {}", e))?;

    let result = encode_image(&img, format, quality, scale, webp_lossless)?;

    let file_stem = input_path_buf
        .file_stem()
        .and_then(|n| n.to_str())
        .unwrap_or("unknown");

    let output_file_name = format!("{}_compress.{}", file_stem, format.to_str());
    let output_path = output_dir_buf.join(&output_file_name);

    fs::write(&output_path, &result.data).map_err(|e| format!("Failed to write file: {}", e))?;

    Ok(SavedImageResult {
        output_path: output_path.to_string_lossy().to_string(),
        width: result.width,
        height: result.height,
        processed_size: result.processed_size,
    })
}
