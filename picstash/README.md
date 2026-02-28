# Picstash - 图床 + 图片展览系统

基于 Git 的图床存储方案，配合瀑布流图片展览。

## 功能特性

- 📸 **Git图床存储**: 基于 GitHub REST API，支持自定义路径
- 🎨 **瀑布流展示**: 使用 Tailwind CSS 实现响应式布局
- 🖼️ **智能缩略图**: 自动生成 1080P 缩略图
- 🏷️ **图片标签**: 支持为图片添加和管理标签
- 📧 **邮箱验证码登录**: 使用 SMTP 发送验证码
- 🔐 **JWT 认证**: 安全的 Token 认证机制

## 技术栈

### 后端
- **框架**: Go 1.25 + Fiber v3
- **数据库**: SQLite3 (modernc.org/sqlite，无CGO)
- **存储**: 支持 GitHub REST API (go-github/v58) 或本地文件系统
- **认证**: JWT (golang-jwt/jwt/v5) + 邮箱验证码
- **配置**: Viper
- **日志**: Go 标准库 log/slog
- **图片处理**: go-image + imaging

### 前端
- **框架**: Vue 3 + TypeScript + Vite
- **UI**: Tailwind CSS v4
- **瀑布流**: 响应式 Grid 布局
- **状态管理**: Pinia
- **HTTP**: Axios

## 快速开始

### 1. 克隆项目

clone project

```bash
cd picstash
```

### 2. 配置后端
```bash
cd backend
cp config.yaml.example config.yaml
# 编辑 config.yaml，填入 GitHub Token、SMTP 等配置
```

### 3. 启动服务
```bash
# 使用 Docker Compose（推荐）
docker-compose up -d

# 或分别启动
cd backend && go run ./cmd/server
cd web && npm run dev
```

### 4. 访问
- 前端（开发）: http://localhost:3000
- 前端（生产）: http://localhost:6200
- 后端: http://localhost:6100
- 健康检查: http://localhost:6100/health

## 配置说明

### 服务器配置
```yaml
server:
  port: 6100                  # 服务端口
  mode: debug                 # 运行模式: debug, release
  max_body_size: 100MB        # 最大上传大小 (如: 10MB, 100MB, 1GB)
```

### 数据库配置
```yaml
database:
  path: ./data/picstash.db    # SQLite 数据库路径
```

### JWT 配置
```yaml
jwt:
  secret: your-jwt-secret     # JWT 密钥，生产环境请使用强随机字符串
  expires_in: 24h             # Token 过期时间
```

### 存储配置

支持两种存储后端：GitHub 或本地文件系统。

#### GitHub 存储（默认）
1. 创建 GitHub Personal Access Token
   - 访问: https://github.com/settings/tokens
   - 选择 `repo` 权限
   - 复制 Token

2. 创建 GitHub 仓库
   - 用于存储图片的仓库

3. 配置 backend/config.yaml
   ```yaml
   storage:
     type: github
      path_prefix: images
     github:
       token: your-github-token
       owner: your-github-username
       repo: your-image-repo
       branch: main
   ```

#### 本地文件系统存储
适用于自建服务器或开发环境，无需 GitHub Token。

```yaml
storage:
  type: local
  path_prefix: images
  local:
    base_path: ./data/files    # 文件存储根目录
    url_path: /files          # URL路径前缀
    server_addr: http://localhost:6100  # 后端服务地址（可选）
```

- `base_path`: 文件实际存储的本地路径
- `url_path`: URL路径前缀（如 `/files`）
- `server_addr`: 后端服务地址，用于拼接完整URL（可选，默认自动推断）

**URL 处理流程：**
- 数据库存储: `/files/images/xxx.jpg`（相对路径）
- API 返回: `http://localhost:6100/files/images/xxx.jpg`（通过 `storage.GetPublicURL()` 获取）
- GitHub 存储返回: `https://raw.githubusercontent.com/owner/repo/branch/images/xxx.jpg`

### SMTP 配置
```yaml
smtp:
  host: smtp.gmail.com
  port: "587"
  username: your-email@gmail.com
  password: your-app-password
  from: noreply@picstash.app
  from_name: Picstash
```

### 认证配置
```yaml
auth:
  allowed_emails:
    - admin@example.com
    - another@example.com
```
允许登录的邮箱地址列表（支持通配符，如 `*@example.com`）

### 上传配置
```yaml
upload:
  thumbnail_width: 1920      # 缩略图宽度（默认1920，即1080P）
  thumbnail_quality: 80      # 缩略图质量 1-100
  thumbnail_format: jpeg     # 缩略图格式: jpeg, png, webp
```

### 日志配置
```yaml
log:
  level: debug               # 日志级别: debug, info, warn, error
  format: json               # 日志格式: json, text
  path: ./logs               # 日志文件路径
```

## API 文档

### 公开接口
```
GET  /api/images              # 获取图片列表（分页）
GET  /api/images/:id          # 获取单张图片详情
GET  /api/tags                # 获取所有标签
GET  /api/tags/:id/images     # 按标签ID筛选图片
GET  /api/sync/logs           # 获取同步日志列表（分页）
GET  /api/sync/logs/:id       # 获取同步日志详情
GET  /api/sync/logs/:id/files # 获取同步日志文件列表
GET  /api/config/public       # 获取公开配置
```

### 认证接口
```
POST /api/auth/send-code      # 发送验证码
POST /api/auth/verify         # 验证码登录
```

### 管理接口（需JWT）
```
POST   /api/images/upload     # 批量上传
POST   /api/images/sync       # 从存储同步图片
DELETE /api/images/:id        # 删除图片
PUT    /api/images/:id/tags   # 更新图片标签
POST   /api/tags              # 创建标签
PUT    /api/tags/:id          # 更新标签
DELETE /api/tags/:id          # 删除标签
GET    /api/config            # 获取完整配置
PUT    /api/config            # 更新配置
```

## 项目结构

```
picstash/
├── backend/               # Go 后端
│   ├── cmd/server/        # 主程序入口
│   ├── internal/          # 内部业务逻辑
│   │   ├── api/          # API 层
│   │   │   ├── handler/  # 处理器
│   │   │   ├── middleware/# 中间件
│   │   │   └── request/  # 请求 DTO
│   │   ├── auth/         # 认证服务
│   │   ├── config/       # 配置管理
│   │   ├── database/     # 数据库层
│   │   ├── model/        # 数据模型
│   │   ├── repository/   # 数据访问层
│   │   ├── service/      # 业务逻辑层
│   │   └── storage/      # 存储抽象层
│   ├── migrations/       # 数据库迁移
│   ├── templates/        # 邮件模板
│   ├── config.yaml       # 配置文件
│   └── Dockerfile
├── web/                   # Vue 前端
│   ├── src/
│   │   ├── api/          # API 封装
│   │   ├── components/   # Vue 组件
│   │   ├── pages/        # 页面
│   │   ├── router/       # 路由
│   │   ├── stores/       # Pinia 状态管理
│   │   ├── types/        # TypeScript 类型
│   │   └── App.vue       # 根组件
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── Dockerfile
└── docker-compose.yml     # Docker 编排配置
```

## 开发指南

### 后端开发
```bash
cd backend
go run ./cmd/server
```

### 前端开发
```bash
cd web
npm install
npm run dev
```

## 部署

### Docker 部署
```bash
docker-compose up -d
```

### 生产环境配置
1. 修改 `backend/config.yaml` 中的日志级别为 `info`
2. 确保 SMTP 配置正确
3. 使用强密码作为 JWT secret

## 许可证

MIT License
