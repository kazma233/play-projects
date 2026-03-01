# 构建阶段
FROM golang:1.25-alpine AS builder

WORKDIR /app

# 安装依赖
RUN apk add --no-cache git

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ftpgo .

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/ftpgo .

# 创建数据目录
RUN mkdir -p /data

# 暴露端口
EXPOSE 8080

# 设置环境变量
ENV FTPGO_ROOT=/data
ENV FTPGO_PORT=8080

# 运行
CMD ["./ftpgo"]