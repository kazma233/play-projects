# Kt工具箱

一个基于 Tauri + Vue 3 开发的跨平台工具集合，提供多种实用的工具。

## 功能特性

app icon 由AI生成

### 🔧 已实现功能
- **时间转换工具** - 时间戳与日期格式互转，支持多时区
- **Base64 编码/解码** - 支持标准和URL安全模式
- **URL 编码/解码** - URL参数编码解码
- **JSON 格式化** - JSON美化和压缩
- **MD5 编码** - 文本MD5哈希计算
- **SHA1 加密** - 文件SHA1哈希计算
- **文件名格式化** - 批量文件重命名工具
- **二维码生成** - 自定义样式的二维码生成
- **图片处理** - 图片压缩、转换和清洗
- **端口占用管理** - 查看和管理系统端口占用情况

## 快速开始

### 安装依赖
```bash
npm install
```

### 开发模式
```bash
npm run tauri dev
```

### 构建应用

#### 生成图标
```bash
npm run tauri icon /path/to/app-icon.png
```

#### 构建所有平台
```bash
npm run tauri build
```

## 项目结构

```
├── src/                    # Vue 前端源码
│   ├── components/         # 组件
│   ├── views/             # 页面
│   │   └── PortsKiller.vue # 端口管理页面
│   └── router/            # 路由配置
├── src-tauri/             # Rust 后端源码
│   ├── src/
│   │   └── lib.rs         # 主要逻辑，包含端口管理功能
│   └── Cargo.toml         # Rust 依赖配置
└── README.md              # 项目说明
```