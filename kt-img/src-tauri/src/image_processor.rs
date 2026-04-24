use image::codecs::jpeg::JpegEncoder;
use image::codecs::png::PngEncoder;
use image::{ExtendedColorType, GenericImageView, ImageEncoder};
use std::fs::{self, OpenOptions};
use std::io::ErrorKind;
use std::io::Cursor;
use std::io::Write;
use std::path::PathBuf;

use fast_image_resize::{FilterType as FirFilterType, ResizeAlg, ResizeOptions, Resizer};

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

fn resize_if_needed(
    img: &image::DynamicImage,
    width: u32,
    height: u32,
) -> Option<image::DynamicImage> {
    let (current_width, current_height) = img.dimensions();

    if current_width == width && current_height == height {
        None
    } else {
        Some(
            resize_image_parallel(img, width, height).unwrap_or_else(|_| {
                img.resize(width, height, image::imageops::FilterType::CatmullRom)
            }),
        )
    }
}

fn resize_image_parallel(
    img: &image::DynamicImage,
    width: u32,
    height: u32,
) -> Result<image::DynamicImage, String> {
    let mut resized = image::DynamicImage::new(width, height, img.color());
    let options =
        ResizeOptions::new().resize_alg(ResizeAlg::Convolution(FirFilterType::CatmullRom));
    let mut resizer = Resizer::new();

    resizer
        .resize(img, &mut resized, Some(&options))
        .map_err(|e| format!("Failed to resize image: {}", e))?;

    let _ = resized.set_color_space(img.color_space());

    Ok(resized)
}

fn encode_webp_image(
    img: &image::DynamicImage,
    quality: u8,
    lossless: bool,
) -> Result<Vec<u8>, String> {
    let encode = |rgb: &[u8], width: u32, height: u32| {
        let encoder = webp::Encoder::from_rgb(rgb, width, height);

        if lossless {
            encoder.encode_lossless().to_vec()
        } else {
            encoder.encode(quality as f32).to_vec()
        }
    };

    match img {
        image::DynamicImage::ImageRgb8(rgb) => Ok(encode(rgb.as_raw(), rgb.width(), rgb.height())),
        image::DynamicImage::ImageRgba8(rgba) => {
            let encoder = webp::Encoder::from_rgba(rgba.as_raw(), rgba.width(), rgba.height());

            Ok(if lossless {
                encoder.encode_lossless().to_vec()
            } else {
                encoder.encode(quality as f32).to_vec()
            })
        }
        _ if img.color().has_alpha() => {
            let rgba = img.to_rgba8();
            let encoder = webp::Encoder::from_rgba(&rgba, rgba.width(), rgba.height());

            Ok(if lossless {
                encoder.encode_lossless().to_vec()
            } else {
                encoder.encode(quality as f32).to_vec()
            })
        }
        _ => {
            let rgb = img.to_rgb8();
            Ok(encode(rgb.as_raw(), rgb.width(), rgb.height()))
        }
    }
}

fn encode_png_image(
    img: &image::DynamicImage,
    quality: u8,
    buffer: &mut Cursor<Vec<u8>>,
) -> Result<(), String> {
    let compression_type = match quality {
        1..=40 => image::codecs::png::CompressionType::Fast,
        41..=89 => image::codecs::png::CompressionType::Default,
        _ => image::codecs::png::CompressionType::Best,
    };

    match img {
        image::DynamicImage::ImageRgb8(rgb) => {
            PngEncoder::new_with_quality(
                buffer,
                compression_type,
                image::codecs::png::FilterType::Adaptive,
            )
            .write_image(rgb, rgb.width(), rgb.height(), ExtendedColorType::Rgb8)
            .map_err(|e| format!("Failed to write PNG: {}", e))?;
        }
        image::DynamicImage::ImageRgba8(rgba) => {
            PngEncoder::new_with_quality(
                buffer,
                compression_type,
                image::codecs::png::FilterType::Adaptive,
            )
            .write_image(rgba, rgba.width(), rgba.height(), ExtendedColorType::Rgba8)
            .map_err(|e| format!("Failed to write PNG: {}", e))?;
        }
        _ if img.color().has_alpha() => {
            let rgba = img.to_rgba8();

            PngEncoder::new_with_quality(
                buffer,
                compression_type,
                image::codecs::png::FilterType::Adaptive,
            )
            .write_image(&rgba, rgba.width(), rgba.height(), ExtendedColorType::Rgba8)
            .map_err(|e| format!("Failed to write PNG: {}", e))?;
        }
        _ => {
            let rgb = img.to_rgb8();

            PngEncoder::new_with_quality(
                buffer,
                compression_type,
                image::codecs::png::FilterType::Adaptive,
            )
            .write_image(&rgb, rgb.width(), rgb.height(), ExtendedColorType::Rgb8)
            .map_err(|e| format!("Failed to write PNG: {}", e))?;
        }
    }

    Ok(())
}

fn encode_jpeg_image(
    img: &image::DynamicImage,
    quality: u8,
    buffer: &mut Cursor<Vec<u8>>,
) -> Result<(), String> {
    match img {
        image::DynamicImage::ImageRgb8(rgb) => {
            let mut encoder = JpegEncoder::new_with_quality(buffer, quality);

            encoder
                .encode(rgb, rgb.width(), rgb.height(), ExtendedColorType::Rgb8)
                .map_err(|e| format!("Failed to write JPEG: {}", e))?;
        }
        _ => {
            let rgb = img.to_rgb8();
            let mut encoder = JpegEncoder::new_with_quality(buffer, quality);

            encoder
                .encode(&rgb, rgb.width(), rgb.height(), ExtendedColorType::Rgb8)
                .map_err(|e| format!("Failed to write JPEG: {}", e))?;
        }
    }

    Ok(())
}

pub fn load_original_image(path: &str) -> Result<OriginalImageResult, String> {
    let path_buf = PathBuf::from(path);

    if !path_buf.exists() {
        return Err("File does not exist".to_string());
    }

    let original_size = std::fs::metadata(&path_buf)
        .map_err(|e| format!("Failed to get file size: {}", e))?
        .len();

    let image_data = fs::read(&path_buf).map_err(|e| format!("Failed to read file: {}", e))?;
    let (width, height) = if let Ok(size) = imagesize::size(&path_buf) {
        (size.width as u32, size.height as u32)
    } else {
        image::image_dimensions(&path_buf)
            .map_err(|e| format!("Failed to get image dimensions: {}", e))?
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
    webp_lossless: bool,
) -> Result<ProcessResult, String> {
    let (original_width, original_height) = img.dimensions();
    let quality = quality.clamp(1, 100);

    let (scaled_width, scaled_height) = scaled_dimensions(original_width, original_height, scale);

    let resized = resize_if_needed(img, scaled_width, scaled_height);
    let source = resized.as_ref().unwrap_or(img);

    let mut buffer = Cursor::new(Vec::new());

    match format {
        OutputFormat::WebP => {
            buffer = Cursor::new(encode_webp_image(source, quality, webp_lossless)?);
        }
        OutputFormat::Png => {
            encode_png_image(source, quality, &mut buffer)?;
        }
        OutputFormat::Jpg => {
            encode_jpeg_image(source, quality, &mut buffer)?;
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

    let (output_path, mut output_file) = reserve_unique_output_file(&output_dir_buf, file_stem, format)?;

    output_file
        .write_all(&result.data)
        .map_err(|e| format!("Failed to write file: {}", e))?;

    Ok(SavedImageResult {
        output_path: output_path.to_string_lossy().to_string(),
        width: result.width,
        height: result.height,
        processed_size: result.processed_size,
    })
}

fn reserve_unique_output_file(
    output_dir: &std::path::Path,
    file_stem: &str,
    format: OutputFormat,
) -> Result<(PathBuf, std::fs::File), String> {
    for index in 0.. {
        let suffix = if index == 0 {
            String::new()
        } else {
            format!("_{}", index + 1)
        };

        let output_file_name = format!("{}_compress{}.{}", file_stem, suffix, format.to_str());
        let output_path = output_dir.join(output_file_name);

        match OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&output_path)
        {
            Ok(file) => return Ok((output_path, file)),
            Err(error) if error.kind() == ErrorKind::AlreadyExists => continue,
            Err(error) => return Err(format!("Failed to create output file: {}", error)),
        }
    }

    Err("Failed to reserve unique output file".to_string())
}

#[cfg(test)]
mod tests {
    use super::*;
    use image::{DynamicImage, Rgba, RgbaImage};
    use std::time::{SystemTime, UNIX_EPOCH};

    fn sample_image() -> DynamicImage {
        let width = 192;
        let height = 128;
        let mut img = RgbaImage::new(width, height);

        for y in 0..height {
            for x in 0..width {
                let r = ((x * 13 + y * 7) % 256) as u8;
                let g = ((x * 5 + y * 11) % 256) as u8;
                let b = (((x * y) + x * 3 + y * 9) % 256) as u8;
                let a = if (x / 12 + y / 12) % 2 == 0 { 255 } else { 210 };
                img.put_pixel(x, y, Rgba([r, g, b, a]));
            }
        }

        DynamicImage::ImageRgba8(img)
    }

    #[test]
    fn scale_changes_output_dimensions() {
        let img = sample_image();

        let result = encode_image(&img, OutputFormat::Jpg, 80, 50, false).unwrap();

        assert_eq!(result.width, 96);
        assert_eq!(result.height, 64);
    }

    #[test]
    fn jpeg_quality_changes_output_size() {
        let img = sample_image();

        let low_quality = encode_image(&img, OutputFormat::Jpg, 25, 100, false).unwrap();
        let high_quality = encode_image(&img, OutputFormat::Jpg, 95, 100, false).unwrap();

        assert_ne!(low_quality.data, high_quality.data);
        assert!(low_quality.processed_size < high_quality.processed_size);
    }

    #[test]
    fn webp_quality_and_lossless_toggle_change_output() {
        let img = sample_image();

        let lossy_low = encode_image(&img, OutputFormat::WebP, 20, 100, false).unwrap();
        let lossy_high = encode_image(&img, OutputFormat::WebP, 90, 100, false).unwrap();
        let lossless = encode_image(&img, OutputFormat::WebP, 90, 100, true).unwrap();

        assert_ne!(lossy_low.data, lossy_high.data);
        assert_ne!(lossy_high.data, lossless.data);
        assert!(lossy_low.processed_size <= lossy_high.processed_size);
        assert!(lossy_high.processed_size < lossless.processed_size);
    }

    #[test]
    fn png_quality_is_effective_by_bucket() {
        let img = sample_image();

        let fast_10 = encode_image(&img, OutputFormat::Png, 10, 100, false).unwrap();
        let fast_30 = encode_image(&img, OutputFormat::Png, 30, 100, false).unwrap();
        let default_50 = encode_image(&img, OutputFormat::Png, 50, 100, false).unwrap();
        let default_80 = encode_image(&img, OutputFormat::Png, 80, 100, false).unwrap();
        let best_95 = encode_image(&img, OutputFormat::Png, 95, 100, false).unwrap();

        assert_eq!(fast_10.data, fast_30.data);
        assert_eq!(default_50.data, default_80.data);
        assert_ne!(fast_10.data, default_50.data);
        assert_ne!(default_50.data, best_95.data);
    }

    #[test]
    fn process_and_save_image_uses_unique_file_names() {
        let unique = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let base_dir = std::env::temp_dir().join(format!("kt-img-test-{}", unique));
        let input_a_dir = base_dir.join("a");
        let input_b_dir = base_dir.join("b");
        let output_dir = base_dir.join("out");

        fs::create_dir_all(&input_a_dir).unwrap();
        fs::create_dir_all(&input_b_dir).unwrap();
        fs::create_dir_all(&output_dir).unwrap();

        let source = sample_image();
        let first_path = input_a_dir.join("logo.png");
        let second_path = input_b_dir.join("logo.jpg");
        source.save(&first_path).unwrap();
        source.save(&second_path).unwrap();

        let first_result = process_and_save_image(
            first_path.to_str().unwrap(),
            output_dir.to_str().unwrap(),
            OutputFormat::WebP,
            80,
            100,
            false,
        )
        .unwrap();
        let second_result = process_and_save_image(
            second_path.to_str().unwrap(),
            output_dir.to_str().unwrap(),
            OutputFormat::WebP,
            80,
            100,
            false,
        )
        .unwrap();

        assert_ne!(first_result.output_path, second_result.output_path);
        assert!(PathBuf::from(&first_result.output_path).exists());
        assert!(PathBuf::from(&second_result.output_path).exists());

        let _ = fs::remove_dir_all(base_dir);
    }
}
