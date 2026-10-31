# ChatGPT 与 MCP 接入

[English](../../how-to/mcp.md) · [Bridge HTTP API](../reference/bridge-api.md)

VidOtter 在现有桌面 Go 程序中增加**13 个可选 MCP 工具**。浏览器扩展、MCP 共用同一个认证 Bridge 和下载任务队列，不存在第二套下载器。

| 工具 | 用途 |
| --- | --- |
| `vidotter_status` | 检查 Bridge 和依赖状态（不会启动 Bridge） |
| `vidotter_download` | 提交公开 HTTP(S) 媒体或受支持的网页链接，可选字幕 |
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

下载接口立即返回 `task_id`，不代表已下载完成。用 `vidotter_get_job` 查询结果。

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

- 只接受公开 HTTP(S) 链接。MCP 在提交时拒绝私网地址、非标准端口和包含账号密码的 URL；公开网页解析可能依赖可选的 yt-dlp。
- 这一输入校验**不能彻底阻止 DNS 重绑定、HTTP 重定向及 yt-dlp 特定访问行为**。不要将私人 Tunnel 共享给不可信的调用者。
- 浏览器登录态和 Cookie 仍由扩展掌握；MCP 不支持替用户刷新浏览器会话。需要重新授权的任务必须回到扩展完成。
- MCP 的任务响应不会返回原始源 URL、绝对路径、Bridge Token；不提供任意文件读取、文件删除或全局配置修改。
- 请求字幕内容时，字幕文本会返回 ChatGPT 云端。此接口只允许读取已完成任务下、位于下载目录内部的关联字幕，单次最多 256 KiB。
- 不要把本地 Bridge 的 `127.0.0.1:17891` 直接公开映射到互联网；远程只转发受限 MCP。

官方说明：[Secure MCP Tunnels](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)。
