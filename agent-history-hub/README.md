# Agent Session Hub

本地桌面工具，用于浏览 Codex、Claude Code、OpenCode 的历史会话，并导入到另一个工具。

## 功能

- 自动发现本机历史会话
- 浏览消息、事件和工具调用
- 导入到其他工具的新会话
- 导入前展示兼容性和目标路径
- OpenCode 子会话自动聚合
- 滚动到底部自动加载更多内容

### 导入说明

- 同程序导入选项在界面中隐藏
- 导入只会创建新会话，不会覆盖原始数据
- Codex 导入写出 transcript JSONL
- Claude Code 导入写入项目目录 JSONL
- OpenCode 导入通过官方 CLI 完成

## 开发

```bash
pnpm install
pnpm check
pnpm tauri dev
cd src-tauri && cargo test

# 端到端测试（需要真实环境和对应的 CLI）
OPENCODE_SESSION_ID=ses_xxx cargo test imports_real_opencode_session_into_codex_and_resumes -- --ignored --nocapture
```

## 构建

```bash
pnpm tauri build
# or pnpm tauri build --bundles (rpm|dmg)
```

## 运行约定

- 详情页只展示跨程序导入目标
- 消息和事件列表滚动到底部会自动加载更多
- OpenCode 读取会聚合 root session 与子会话

## 应用图标

图标使用声明：

作者：[Enze Fu](https://www.artstation.com/fuenze)

作品：[Aoi's Morning, Putting on Tights](https://www.artstation.com/artwork/dywQkA)

作为爱好开发使用，如有不妥随时移除

## License

MIT
