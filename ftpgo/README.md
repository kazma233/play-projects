# FTPGo - 简单文件管理器

一个基于 Go + Vue3 + Tailwind CSS 的轻量级文件管理器，支持 FTP 风格的文件浏览、上传、下载和管理功能。

## 特性

- 📁 **目录导航** - 面包屑导航，双击进入文件夹
- 📤 **文件上传** - 支持多文件上传、拖拽上传、文件夹上传
- 📋 **剪贴板上传** - 支持粘贴图片、文件和纯文本（自动转换为 .txt 文件）
- 👁️ **文件预览** - 支持图片和文本文件（代码、配置、Markdown等）预览，大文件智能截断
- 📥 **文件下载** - 单文件下载、多文件批量下载为 ZIP
- 🗂️ **文件管理** - 创建文件夹、重命名、删除文件/文件夹
- 🎨 **现代界面** - 使用 Vue3 + Tailwind CSS 构建的响应式界面
- 🚀 **高性能** - 基于 Fiber 框架，轻量快速
- 🐳 **简单部署** - 支持 Docker 和 Docker Compose

## 快速开始

### 使用 Docker Compose（推荐）

```bash
# 克隆项目后进入目录
cd ftpgo

# 可选：在 .env 中配置鉴权
# FTPGO_AUTH_USER=admin
# FTPGO_AUTH_PASS=your_secure_password

# 启动服务
docker-compose up -d

# 访问 http://localhost:7300
```

### 使用 Docker

```bash
# 构建镜像
docker build -t ftpgo .

# 运行容器
docker run -d \
  -p 7300:7300 \
  -v $(pwd)/data:/data \
  -e FTPGO_ROOT=/data \
  --name ftpgo \
  ftpgo
```

### 直接运行

```bash
# 安装依赖
go mod tidy

# 运行
go run .

# 或使用环境变量
FTPGO_ROOT=./data FTPGO_PORT=8080 go run .
```

### 构建二进制文件

```bash
# 构建
go build -o ftpgo .

# 运行
./ftpgo
```

## 配置

应用支持通过环境变量进行配置：

| 环境变量 | 说明 | 默认值 |
|---------|------|--------|
| `FTPGO_ROOT` | 文件存储根目录 | `./data` |
| `FTPGO_PORT` | 服务端口 | `7300` |
| `FTPGO_MAX_SIZE` | 最大文件大小（字节） | `1073741824` (1GB) |
| `FTPGO_CORS_ORIGINS` | 允许跨域来源（逗号分隔） | (空，默认同源) |
| `FTPGO_AUTH_USER` | Basic Auth 用户名 | (空，不启用鉴权) |
| `FTPGO_AUTH_PASS` | Basic Auth 密码 | (空，不启用鉴权) |

### 启用鉴权

```bash
# 设置用户名和密码
export FTPGO_AUTH_USER=admin
export FTPGO_AUTH_PASS=your_secure_password
./ftpgo
```

访问时会弹出浏览器 Basic Auth 鉴权窗口。

### 示例

```bash
export FTPGO_ROOT=/var/files
export FTPGO_PORT=3000
export FTPGO_MAX_SIZE=2147483648  # 2GB
export FTPGO_CORS_ORIGINS=https://files.example.com
export FTPGO_AUTH_USER=admin
export FTPGO_AUTH_PASS=admin123
./ftpgo
```

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/browse?path=/xxx` | 浏览目录内容 |
| POST | `/api/upload?path=/xxx` | 上传文件（支持多文件） |
| POST | `/api/mkdir` | 创建目录 |
| POST | `/api/rename` | 重命名文件/目录 |
| POST | `/api/delete` | 删除文件/目录 |
| GET | `/api/download?path=/xxx` | 下载文件 |
| GET | `/api/download-zip?paths=[...]` | 批量下载为 ZIP |

## 项目结构

```
ftpgo/
├── main.go           # 应用入口
├── config.go         # 配置管理
├── fs.go             # 文件系统操作
├── handlers.go       # HTTP 处理器
├── middleware.go     # 中间件
├── templates/
│   └── index.html    # 前端页面
├── static/           # 静态资源
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## 技术栈

- **后端**: Go 1.23 + Fiber v3
- **前端**: Vue 3 (CDN) + Tailwind CSS (CDN)
- **部署**: Docker / Docker Compose

## 截图

![界面预览](screenshot.png)

## 许可证

MIT License
