# AGENTS.md

SSH 隧道管理器：Wails v2 + Vue 3 + TypeScript 桌面应用（Windows 优先），Go 后端提供 SSH 端口转发。

## 构建与测试

- **日常改动无需构建**：代码改完即交回，不要主动运行 `wails build` / 完整打包，除非用户明确要求构建或需要验证打包产物。
- `wails build` — 完整构建（生成前端绑定 → pnpm 构建 frontend/dist → 编译 exe 到 `build/bin/`）。前端依赖安装用 pnpm（`wails.json` 的 `frontend:install` 为 `pnpm install`）。发布构建用 `.\build.ps1`（校验 pnpm 可用，产物与 config.yaml/password.txt 均留在 `build\bin`，不输出到 dist）。
- `wails dev` — 前端热重载开发。
- `go test ./...` — 运行 Go 测试（config/tunnel 包有测试）。
- `go build ./...` — 仅编译 Go（需 `frontend/dist` 至少有一个文件，见下）。

## 架构

- `main.go`（仓库根，Wails 入口）：解析 `-config` 参数，加载配置，启动 `wails.Run` 并绑定 `internal/app.App`。
- `internal/app/`：Wails 绑定层。`App` 的导出方法即前端 API（前端通过 `frontend/wailsjs/go/app/App` 调用，该目录由 wails 生成、勿手改）；日志/状态通过 `runtime.EventsEmit` 推送（事件名 `log`、`status`）。
- `internal/tunnel/`：核心转发逻辑（`Manager` 管理 SSH 连接、keepalive、重连；`Forward` 单条隧道；`TrafficStats` 流量统计），与 UI 完全解耦，通过回调上报。
- `internal/config/`：YAML 配置读写（yaml tag 蛇形；JSON 序列化（Wails 绑定）`TunnelConfig` 用蛇形 json tag，与前端 `types.ts` 字段名一致，勿改动）。
- `frontend/`：Vue 3 + Vite + TS 界面；轮询 `GetStats`（1s），订阅事件拿日志。

## 项目特有注意事项

- **embed 路径相对 main 包解析**：`main.go` 必须留在仓库根（曾因放在 `cmd/ssh-tunnel` 导致 `//go:embed all:frontend/dist` 找不到文件）。`frontend/dist` 被 gitignore，纯 `go build` 前需保证其中有文件。
- **`wails build` 会在临时目录执行主程序生成绑定**：因此 config.yaml 缺失时不得 `os.Exit(1)`——用 `config.Default()` 兜底启动，错误经 `App` 转发到 UI 日志。
- 应用图标：`build/appicon.png`（由根目录 `transfor.ico` 内嵌 PNG 提取而来）。Wails 会从它自动生成 Windows 资源（syso）——**不要**再额外放置 go-winres 生成的 syso，否则链接时报 "too many .rsrc sections"。
- 已移除系统托盘/最小化到托盘功能，不要重新引入 systray 依赖；关窗即退出，`App.Shutdown` 负责停隧道。
- 密码/密钥口令临时文件写入 `secrets/`（0700），用完即删；运行日志写 `logs/ssh-tunnel-YYYY-MM-DD.log`，这些目录相对于 exe 工作目录。
- 前端只做会话级状态（如隧道 checkbox 启用态），持久化一律经 Go 侧写回 `config.yaml`（`Config.Save`）。

## 维护规则

当项目结构、构建/测试命令、架构边界、开发约定或本文件记录的其他事实发生变化时，必须在同一次改动中同步更新本文件。
