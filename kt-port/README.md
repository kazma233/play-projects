# kt-port

`kt-port` 是一个端口查看和进程终止工具，支持按协议、状态、关键字筛选，查看端口详情，并对选中的 PID 执行终止操作。

## 功能

- 查看当前系统端口列表
- 按协议、状态、关键字过滤
- 默认只看 `监听中`，并默认勾选 `有PID`
- 只展示有 PID 的记录
- 按列排序
- 点击行查看下方详情
- 选中多行后执行 `Kill Selected`
- 对没有 PID 的记录禁止勾选
- 终止前会弹窗确认

## 运行

```bash
cargo run
```

## 构建 Release

```bash
cargo build --release --locked
```

构建产物在：

```bash
target/release/kt-port
```

## 打包

项目现在使用 `cargo-bundle` 打包。

先安装工具：

```bash
cargo install cargo-bundle --locked
```

执行构建：

```bash
cargo bundle --release

# or explicitly build deb on Linux
cargo bundle --release --format deb
```

构建后会得到：

- `target/release/bundle/deb/` 下的 `.deb` 安装包

打包配置在 `Cargo.toml` 的 `[package.metadata.bundle]`。

## Linux 安装

### Debian / Ubuntu

```bash
sudo apt install ./target/release/bundle/deb/*.deb
```

安装后会把二进制、图标和 `.desktop` 文件放进系统目录，应用菜单里会像普通 Linux 程序一样显示。

## 说明

- `cargo-bundle` 当前官方文档主路径的 Linux 包类型是 `.deb`
- 如果还需要 `.rpm`，需要继续保留另一条打包链路

- 项目使用 `iced` 构建界面
- 端口扫描和终止逻辑在 `src/ports` 下
- 不同平台会走对应的扫描器实现

## 应用图标

使用 AI 生成

## License

MIT
