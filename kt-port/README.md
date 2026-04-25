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

先安装 `cargo-packager`：

```bash
cargo install cargo-packager --locked
```

执行打包：

```bash
cargo packager --release

# or
cargo packager --release --formats appimage
```

打包配置在 `Packager.toml`，产物默认输出到 `dist/`。

## 说明

- 项目使用 `iced` 构建界面
- 端口扫描和终止逻辑在 `src/ports` 下
- 不同平台会走对应的扫描器实现

## 应用图标

使用 AI 生成

## License

MIT
