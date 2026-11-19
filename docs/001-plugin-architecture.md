# 001 · 插件架构（第一版草案）

## 已确定的方向

- Lua 脚本运行在 Go 进程内嵌的 `gopher-lua` VM 中，通过 Host API 调用系统能力。
- 插件 UI 由插件自带 web 资源提供，核心用 iframe 加载，`postMessage` 通信。
- Shell 控制台样本采用持久 shell 会话（`cd`、环境变量在命令间保留）。
- 不使用 `init()` 这种含糊命名；生命周期表达的是**时机**而非初始化过程。

## 插件目录

```
plugins/<id>/
├── plugin.lua       # 元数据 + 生命周期 + 事件处理
└── web/             # 插件自带的前端资源
    ├── index.html
    ├── style.css
    └── app.js
```

## 生命周期（工作草案，可改名）

| 名称 | 时机 |
| --- | --- |
| `on_install()` | 插件首次被安装，每个插件仅一次（标记在 `tmp/plugin-state/`） |
| `on_load()` | 插件被载入 Lua VM，每次服务启动都会触发 |
| `on_first_run()` | 用户第一次打开该插件界面 |
| `on_ui_show(session)` | 窗口显示 / 从最小化恢复 |
| `on_ui_hide(session)` | 窗口隐藏 / 最小化 |
| `on_close(session)` | 窗口关闭，会话结束 |

## Lua 侧职责

1. 元数据：`id / name / version / description / icon / menu / dock / web / window / shell`
   - `web` 是 `web/` 目录下的入口文件名（如 `index.html`），URL 前缀 `/api/plugins/{id}/web/` 由路由层统一拼接
   - `shell` 声明会话使用的解释器：`powershell`（优先 pwsh，回退 powershell.exe）、`cmd`、留空为系统默认
   - 第二个插件 `plugins/powershell/` 仅靠元数据 + Lua + web 资源接入，未改动核心逻辑，用于验证契约可复用
2. 生命周期钩子
3. 事件处理：`on_command(session, cmd)`
4. 调用 Host API

## Host API（第一版）

- `host.log(level, msg)`
- `host.exec(cmd)` → `{stdout, stderr, code, error}`，一次性执行，30s 超时
- `host.emit(session_id, chunk)` → 向会话输出流推送文本
- `host.shell_start(session_id)` / `host.shell_write(session_id, input)` / `host.shell_close(session_id)`
- `host.get_config(key)` / `host.set_config(key, value)`（当前仅内存）

## HTTP 接口

- `GET /api/plugins` → 插件元数据列表（顶部菜单栏 / Dock 从这里生成）
- `GET /api/plugins/{id}/web/...` → 插件静态资源
- `POST /api/plugins/{id}/lifecycle` → `{hook, session}`
- `POST /api/plugins/{id}/call` → `{method, session, args}`
- `GET /api/plugins/{id}/events?session=...` → SSE，流式输出

## 已知限制（后续迭代）

- 配置未持久化，重启丢失。
- 输出为纯文本并剥离 ANSI，没有真正的终端仿真（如 xterm.js）。
- Lua VM 生命周期与进程绑定，尚未支持运行时热加载 / 卸载。
- 没有权限模型；shell 插件本身可执行任意命令，仅靠 30s 超时约束 `host.exec`。
