# ChatGPT 与 MCP 接入

[English](../../how-to/mcp.md) · [Bridge HTTP API](../reference/bridge-api.md)

VidOtter 在现有桌面 Go 程序中增加**13 个可选 MCP 工具**。浏览器扩展、MCP 共用同一个认证 Bridge 和下载任务队列，不存在第二套下载器。

| 工具 | 用途 |
| --- | --- |
| `vidotter_status` | 检查 Bridge 和依赖状态（不会启动 Bridge） |
| `vidotter_download` | MP4/M3U8 直链走原生下载；网页默认使用浏览器捕获的 HLS/MP4；`strategy=page` 才显式使用 yt-dlp 网页解析 |
| `vidotter_list_jobs` | 查询最近的任务状态 |
| `vidotter_get_job` | 查询单个任务的进度 |
| `vidotter_control_job` | 暂停、恢复、重试，不提供浏览器凭据 |
| `vidotter_read_subtitle` | 读取已完成任务的字幕文件，最大 256 KiB |

| `vidotter_list_browser_media` | 读取扩展抓到的媒体索引（短期资源 ID，不含签名链接和 Cookie） |
| `vidotter_download_browser_media` | 请扩展按照资源 ID 与当前授权会话提交下载 |
| `vidotter_browser_status` | 查询可选浏览器自动化的安装与权限状态 |
| `vidotter_browser_open` | 经可选组件创建公开网页的后台标签页 |
| `vidotter_browser_snapshot` | 读取组件创建的后台页面结构 |
| `vidotter_browser_click` | 仅本地显式启用自动点击后才能操作已有控件 |
| `vidotter_browser_close` | 关闭组件自己创建的后台标签页 |

**普通网页默认走浏览器优先。** 在 Edge/Chrome 后台标签页打开网页、正常播放后，调用 `vidotter_download(url=...)`。VidOtter 在 Bridge 内按**完整页面 URL** 匹配短期媒体索引，匹配到 HLS/MP4 后，交由原生下载队列处理；浏览器 Cookie 和请求头仅在提交下载时由扩展读取，不会通过 MCP 返回。未捕获到匹配媒体时，立即返回 `status=waiting_for_browser`、原任务 `task_id` 及 `requires_user_action=true`。用户在浏览器完成必要的人工操作并点击播放后，扩展通过原有媒体事件唤醒等待任务，**沿用同一个任务 ID 和原生下载流程**，无需再次提交 URL。扩展在线时尝试发送一次 Edge/Chrome 本地通知；不自动处理真人验证。最长等待 20 分钟，超时后需要重新提交。**不会自动回退 yt-dlp**。普通 MP4/M3U8 直链仍可直接下载；只有明确选择 `strategy=page` 才尝试 yt-dlp 解析兼容的公开网页。`strategy=browser` 表示严格要求浏览器捕获。

Bridge 重启或内存索引丢失时，扩展在重新连接及 MCP 请求时**补报十分钟内捕获的非敏感媒体索引**。MCP 列表仍去除页面查询参数；Bridge 内部使用完整页面身份做精确匹配，避免同路径不同视频误选。若同一页面同时有 HLS 和 MP4，优先选择 HLS；只有 DASH/分离轨道时，仍需用户显式选择适用的通用页面解析方式。

下载接口立即返回 `task_id`，不代表已下载完成。用 `vidotter_get_job` 查询结果。

## MCP 与桌面安装的关系

**安装 VidOtter 就已经安装 MCP Server。** MCP 与下载引擎共用同一个 Go
可执行文件，不需要另装独立的 MCP 服务。浏览器扩展通过 Native Messaging
按需启动本地 Bridge；支持本地 stdio 的 AI 客户端可按需运行
`bin/vidotter-系统-架构 --mcp-stdio`，无须常驻后台下载服务。

**ChatGPT 云端访问需用户明确选择后再开启。** 仅在选择“连接 ChatGPT”时，
才需要个人 Tunnel ID、权限受限的运行密钥和官方 tunnel-client。
发行包附带的可选脚本在用户确认后设置登录时自动启动的 Tunnel；
平时不必手动启动，但不能在普通安装时默认开启远程访问。
Tunnel 相关私有信息应留在系统用户目录，不能写入公开仓库和发行包。

## 配置向导（可选）

先安装官方 tunnel-client。Windows 在发行包目录执行 `powershell -NoProfile -File scripts/Setup-VidOtter-MCP.ps1`；Linux/macOS 执行 `bash scripts/setup-mcp.sh`。向导询问 Tunnel ID 和受限运行密钥，保存到当前用户私有目录（Windows 使用 DPAPI，Unix 使用仅本人可读的文件），运行 doctor，并尽可能设置登录后常驻启动。重复执行时保留已有私有配置，同一个 Tunnel 只应运行一个客户端进程。向导不自动下载 tunnel-client，也不能代替用户在 ChatGPT 中注册和授权。

新版 MCP 2026-07-28 使用 `server/discover` 进行无状态工具发现；旧客户端仍可用 `initialize` 握手，两者共用同一 stdio 连接。

## 可选媒体抓包与浏览器自动化

**只需 VidOtter 一个 MCP 和一个私人 Tunnel。** 通过[浏览器自动化组件](browser-automation.md)按需安装 Remote Browser MCP v0.3.0 原版发行包作为本地 stdio 依赖，无需复制源码或创建第二个插件。VidOtter 只暴露受限白名单：默认不操作用户已有标签页、不执行任意 JavaScript、不访问浏览器存储/Cookie；自动点击必须在本地单独确认后才会开启。

扩展将非敏感媒体索引写入 Bridge 的十分钟内存缓存；MCP 使用资源 ID 请求下载时，通过持久 Native Messaging 让扩展按需读取 Cookie 并按原有流程提交，且会验证 MCP 触发任务的媒体目标是否指向公开网络。

### Windows 单 MCP 下载实测流程

在扩展 **设置 → 组件** 启用 Browser Automation 后，只使用 VidOtter MCP：

```text
vidotter_browser_status
vidotter_browser_open(url="https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4")
vidotter_list_browser_media
vidotter_download_browser_media(resource_id=<列表返回且匹配 MP4 的资源 ID>, quality="best")
vidotter_get_job(id=<上一步返回的任务 ID>)
vidotter_browser_close(page_id=<open 返回的标签页 ID>)
```

所有 ID 都是临时的，必须从前一个工具响应中获取，不能写死。扩展已经捕获媒体时，可直接查询资源和提交下载，不必再次打开或点击播放器。本例已在 Windows 对公开 MP4 完整实测，并不代表所有网站或 HLS/DASH 都已验证；详见[验收记录](browser-automation.md#已验证单-vidotter-mcp-完整链路windows)。

## 本地 stdio

桌面可执行文件支持 `--mcp-stdio`，例如：

```powershell
.\bin\vidotter-windows-amd64.exe --mcp-stdio
```

Linux/macOS 使用对应的 `bin/vidotter-linux-...` 或 `bin/vidotter-macos-...`。MCP 客户端负责启动此命令。多个顺序初始化会话可复用常驻进程；stdout 只发送 MCP 协议。

## 连接 ChatGPT（可选）

1. 安装官方 [OpenAI tunnel-client](https://github.com/openai/tunnel-client)，创建只有 **Tunnels Read + Use** 权限的运行密钥。
2. 在 [OpenAI Platform — Tunnels](https://platform.openai.com/settings/organization/tunnels) 创建本人专属的 Tunnel；运行密钥在 [API Keys](https://platform.openai.com/settings/organization/api-keys) 创建。
3. 在与 VidOtter Bridge **同一系统环境**配置私有 profile（把占位符替换为自己的 Tunnel ID 和程序路径）：

   ```sh
   tunnel-client init --sample sample_mcp_stdio_local --profile vidotter \
     --tunnel-id tunnel_... \
     --mcp-command "/absolute/path/to/bin/vidotter-linux-amd64 --mcp-stdio" \
     --health-listen-addr 127.0.0.1:0
   ```

4. 通过受保护的本地环境或密钥存储提供 `CONTROL_PLANE_API_KEY`，执行 `tunnel-client doctor --profile vidotter`，随后执行 `tunnel-client run --profile vidotter` 保持连接。
5. 在 ChatGPT 中选择 **Plugins → Add custom MCP server → Tunnel**（界面名称可能变化），选择自己的 Tunnel 并授权。刷新工具发现，连续两次调用 `vidotter_status` 验收。

安装目录的 `Data/`、下载文件和 Tunnel 密钥都不得提交 GitHub。Tunnel 健康、本地 MCP 正常和 ChatGPT 实际调用成功是三个不同验收条件。

## 安全边界

- 只接受公开 HTTP(S) 链接。MCP 在提交时拒绝私网地址、非标准端口和包含账号密码的 URL。网页默认使用浏览器捕获的 HLS/MP4；只有显式 `strategy=page` 才使用可选的 yt-dlp。
- 这一输入校验**不能彻底阻止 DNS 重绑定、HTTP 重定向及 yt-dlp 特定访问行为**。不要将私人 Tunnel 共享给不可信的调用者。
- 浏览器登录态和 Cookie 仍由扩展掌握；MCP 不支持替用户刷新浏览器会话。需要重新授权的任务必须回到扩展完成。
- MCP 的任务响应不会返回原始源 URL、绝对路径、Bridge Token；不提供任意文件读取、文件删除或全局配置修改。
- 请求字幕内容时，字幕文本会返回 ChatGPT 云端。此接口只允许读取已完成任务下、位于下载目录内部的关联字幕，单次最多 256 KiB。
- 不要把本地 Bridge 的 `127.0.0.1:17891` 直接公开映射到互联网；远程只转发受限 MCP。

官方说明：[Secure MCP Tunnels](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)。

### 在 VidOtter 设置页配置并启动 Tunnel（新版）

1. 在 [OpenAI Platform Tunnels](https://platform.openai.com/settings/organization/tunnels) 创建或查看私人 Tunnel，复制 Tunnel ID。
2. 在 [OpenAI Platform API Keys](https://platform.openai.com/settings/organization/api-keys) 创建 Restricted Runtime API Key，权限只需 Tunnels Read + Use。Tunnel ID 不是身份凭证，单靠它无法启动服务。
3. 在本机安装官方 [tunnel-client](https://github.com/openai/tunnel-client/releases/latest)，将程序放入 PATH 或用户 .local/bin 目录。
4. 打开 VidOtter 浏览器扩展 → 设置 → AI 集成 → ChatGPT MCP；在该页面内填写 Tunnel ID，点击「配置并启动 Tunnel」。这个按钮不会跳转到外部网页。旁边的「查看 VidOtter 配置文档」才会主动打开本教程。
5. VidOtter 验证 Tunnel ID 和客户端后，在本机系统终端运行绑定的配置脚本；首次使用时在那里输入 Runtime API Key。密钥不会交给浏览器扩展，也不会通过命令行参数传递。
6. 脚本初始化和诊断 Tunnel，设置用户级登录自动启动：Windows Startup 快捷方式、macOS LaunchAgent，或 Linux systemd --user。最后在 ChatGPT Plugins 中创建采用同一个 Tunnel 的自定义 MCP 插件，并执行一次真实工具调用验证。

说明：本地 MCP Server 随 VidOtter 安装，无须另装。配置文件存在不等于 Tunnel 已连通，也不等于 ChatGPT 已授权。设置页会如实显示“连接待验证”；请使用 doctor/health 和 ChatGPT 实际调用确认。

如果缺少官方 Tunnel Client，设置页会展示提示和文档链接，不会自动跳转或下载第三方程序；已经配置不同 Tunnel ID 时，脚本会拒绝静默覆盖。

## 新版一站式接入（开发分支，推荐）

**不用预制插件 ZIP。** ChatGPT 可直接创建私有 Tunnel 插件并自动发现 VidOtter 的 13 个 MCP 工具。上传 ZIP 并不能代替每位用户自己的 Tunnel 与授权；Secure MCP Tunnel 目前也不支持作为公开插件提交/分发。

用户只需要：首次在 OpenAI Platform 的 Tunnels 页面创建自己的 Tunnel ID → 在 VidOtter 设置页按需安装官方 Tunnel Client（来自 openai/tunnel-client 正式 GitHub Release，SHA-256 校验）→ 填入 Tunnel ID，点击「配置并启动 Tunnel」。本机脚本复用同一用户已保存的 VidOtter 运行凭据或显式提供的 CONTROL_PLANE_API_KEY。只有首次缺少凭据时，才在本机终端输入 Runtime API Key。不会扫描其他项目的私有密钥。

配置成功后，用户可以在 VidOtter 设置中「启动 / 停止 / 重启」本机 Tunnel 服务；通过官方 health 命令要求至少一次控制平面轮询成功，才能判定服务 Ready。状态为 Ready 不意味着 ChatGPT 插件已创建。

随后点击「复制 Tunnel ID」及「前往 ChatGPT 创建插件」，选择 Plugins → + → Add custom MCP server → Tunnel，选择或粘贴该 ID，按实际情况选择应用层认证（不需要应用层认证可选 None），确认风险提示，创建私有插件。MCP 工具自动发现；不需要上传 ZIP 或手工复制 Schema。最后在 ChatGPT 中调用 vidotter_status 验收。

注意：Tunnel ID 不是运行凭证；云端插件的首次创建与授权仍由用户本人完成。下载客户端不会自动开启远程访问。发行版和正式更新源尚未发布，本功能必须更新扩展和本地 Bridge。
