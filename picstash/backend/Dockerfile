FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk --no-cache add gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o server ./cmd/server

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/server .
COPY config.yaml .

RUN mkdir -p data logs

EXPOSE 6100

CMD ["./server"]
