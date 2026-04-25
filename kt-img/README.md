# kt-img

`kt-img` 是一个基于 Tauri + Vue 3 + Rust 的本地图片压缩与转码工具，专门处理图片预览、压缩、缩放和批量导出。

## 功能特性

- 本地处理：图片处理在本机完成，不依赖远程服务
- 原图/处理后双栏预览：支持拖动分栏，便于直接对比效果
- 自动刷新预览：修改格式、质量/压缩强度、WebP 模式或缩放比例后会自动重新生成处理预览
- 格式转换：支持导出为 `JPG`、`PNG`、`WebP`
- WebP 模式切换：支持有损和无损两种编码方式
- 缩放导出：支持 `1%` 到 `100%` 的缩放比例
- 单张保存：处理后可自定义保存路径与文件名
- 批量导出：统一参数批量处理多张图片并输出到指定目录
- 唯一输出命名：默认使用 `原文件名_compress.ext`，冲突时自动追加序号

## 支持格式

输入格式：

- `jpg`
- `jpeg`
- `png`
- `webp`
- `bmp`

输出格式：

- `jpg`
- `png`
- `webp`

## 使用方式

1. 点击“添加图片”选择一张或多张本地图片。
2. 在右侧设置输出格式、质量/压缩强度、缩放比例，以及 WebP 是否无损。
3. 预览区会自动生成处理后的对比图。
4. 点击“保存单张”导出当前图片，或点击“批量保存”将当前配置应用到全部图片。

说明：

- `JPG` 和有损 `WebP` 使用质量参数，数值越低通常体积越小。
- `无损 WebP` 会固定使用无损编码，并禁用质量滑块。
- `PNG` 使用压缩强度参数，数值越高通常压缩更充分，但处理可能更慢。
- 输出图片会重新编码，不保留原始图片元数据。

## 快速开始

安装依赖：

```bash
npm install
```

启动开发环境：

```bash
npm run tauri dev
```

仅构建前端资源：

```bash
npm run build
```

构建桌面应用：

```bash
npm run tauri build

# or
npm run tauri build -- --bundles rpm
```

## 技术栈

- 前端：Vue 3 + Vite
- 桌面容器：Tauri 2
- 图片处理：Rust + `image` + `fast_image_resize` + `webp`

## 项目结构

```text
├── src/                      # Vue 前端
│   ├── components/           # 页面组件
│   │   └── ImageTools.vue    # 图片处理主界面
│   ├── assets/               # 全局样式
│   └── App.vue               # 应用入口
├── src-tauri/                # Tauri / Rust 后端
│   ├── src/
│   │   ├── lib.rs            # Tauri 命令入口
│   │   └── image_processor.rs  # 图片处理核心逻辑
│   └── tauri.conf.json       # 应用配置
└── README.md
```

## 输出规则

- 单张保存时，默认建议文件名为 `原文件名_compress.ext`
- 批量保存时，输出目录中的文件名也遵循同样规则
- 如果目标文件已存在，会自动生成 `原文件名_compress_2.ext`、`原文件名_compress_3.ext` 这类唯一名称

## 应用图标

使用 AI 生成

## License

MIT
