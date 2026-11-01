# Bridge HTTP API

这是扩展和 Go Worker 使用的桌面内部协议。根地址为 `http://127.0.0.1:17891`，JSON 操作使用 `Content-Type: application/json`。特权请求需要 `X-VidOtter-Token`，由 Native Messaging 交给扩展。ping 同样需要认证，直接访问返回 401 不代表 Bridge 没运行。

Web UI 操作通过扩展白名单中继，HTTP 页面不获取 Token。Worker 通过 `/v1/worker/{token}` 下的一次性独立 Token 领取任务，该入口不是浏览器集成接口。

## 路由

| 方法 / 路径 | 结果或操作 |
| --- | --- |
| `GET /v1/ping` | 认证健康检查：`{ok, pid}` |
| `GET /v1/environment` | Bridge 平台、版本和 FFmpeg/yt-dlp 状态 |
| `GET /v1/config` | 完整配置 |
| `PUT /v1/config` | 验证并保存完整配置 |
| `GET /v1/preferences` | 界面与检测偏好 |
| `PUT /v1/preferences` | 局部更新偏好 |
| `GET /v1/tools` | 工具注册表与模型状态 |
| `POST /v1/tools/yt-dlp/install` | 安装 yt-dlp |
| `POST /v1/tools/whisper/models/install` | 安装或选择识别模型 |
| `POST /v1/tools/llama/models/install` | 确认许可后安装或选择翻译模型 |
| `POST /v1/playlist/probe` | 探测支持的播放列表 |
| `POST /v1/tasks` | 提交，`202` 返回 `{ok, taskId}` |
| `GET /v1/jobs` | 任务快照 |
| `GET /v1/jobs/{id}/subtitle?index=0` | 读取完成任务下已登记的字幕（限制下载目录、256 KiB） |
| `POST /v1/jobs/clear` | 移除完成、失败记录 |
| `POST /v1/jobs/{id}/pause` | 暂停活动 Worker |
| `POST /v1/jobs/{id}/resume` | 继续，必要时刷新会话 |
| `POST /v1/jobs/{id}/retry` | 重试失败任务 |
| `POST /v1/jobs/{id}/delete` | 删除记录，`deleteFiles` 控制是否删除输出 |
| `POST /v1/jobs/{id}/open` | 打开输出 |
| `POST /v1/jobs/{id}/reveal` | 在文件管理器定位输出 |
| `POST /v1/open-output` | 打开配置的输出目录 |
| `GET /v1/logs?limit=100` | 近期事件，最多 1000 条 |
| `DELETE /v1/logs` | 清空指定运行日志 |
| `POST /v1/restart` | 支持且无活动任务时重启 |
| `GET /console/` | 跳转至共享界面资源 |

## 任务请求体

以下是公开媒体直链任务示例，`example.invalid` 是占位符，不能实际下载：

```json
{
  "url": "https://example.invalid/video.mp4",
  "title": "Example video",
  "quality": "best",
  "playlist": "single",
  "resolvePage": false,
  "subtitles": { "mode": "none" }
}
```

`bridgeTask` 还包含 `referer`、`origin`、`userAgent`、`pageUrl`、页面和帧 ID，以及分别限定于页面和媒体的请求头、Cookie。应通过现有扩展流程获取，不要把凭据放入示例或日志。

`subtitles.mode` 支持 `none`、`site`、`site-or-asr`、`asr`。其他字段为 `languages`、`includeAutomatic`、`format`（`best`、`srt`、`vtt`、`ass`）、`asrLanguage`、`targetLanguage`、`bilingual`。识别后翻译必须明确语音语言，双语输出必须指定目标语言。

完整配置 PUT 使用 `outputDir`、`ffmpegPath` 等 JSON camelCase 字段，不使用 TOML 分节。先读取配置，再保留完整对象；局部界面或检测修改使用 preferences PUT。

## 响应与内部上报

处理器通常通过 `{ok:false,error}` 返回错误：400 为输入或状态无效，401 为认证失败，404 为路由或任务不存在。部分方法错误没有 JSON 正文。重启时有活动任务会返回 409，不支持重启则返回 503。

提交不等于完成，应轮询任务并检查状态、阶段和文件，见[任务状态](../how-to/manage-downloads.md)。

`/v1/jobs/{id}/` 下的 `progress`、`output`、`files`、`state` 供 Worker 上报结果，不是用户控制。字段见 `bridge/bridge.go`、`bridge/bridge_jobs.go`、`bridge/subtitle.go`、`bridge/bridge_preferences.go`。中继契约见 `extension/background.js` 和 `extension/console-relay.js`，中继动作与 HTTP 路由不同。


### 浏览器媒体集成（仅本地认证 API）

网页下载接口在已捕获媒体时返回普通下载任务 ID；否则返回 `202 {matched:false, taskId, status:"waiting_for_browser"}`。等待任务存储在既有 `jobs.json`，以页面 SHA-256 哈希精确匹配，**不持久化带查询参数的完整页面 URL**；扩展通过原有媒体事件唤醒等待任务，并在内部向 `POST /v1/tasks` 附带 `resumeJobId`，复用原任务 ID 和原生下载队列。这不是新增公开 MCP 工具。等待期限为 20 分钟；本地通知尽力发送，网站验证由用户手动完成。

下列接口均需通过回环地址提供 `X-VidOtter-Token`，**不得直接暴露到公网**。浏览器索引仅在 Bridge 内存中保留十分钟。MCP 无法获取媒体签名 URL、Cookie 或鉴权请求头。

| HTTP 接口 | 用途 |
| --- | --- |
| `POST /v1/browser/media` | 扩展提交页面身份、标签页内资源键和媒体类型，仅包含索引元数据。 |
| `GET /v1/browser/media` | 返回短期不透明媒体 ID 与已去除查询参数的页面信息。 |
| `GET /v1/browser/media?refresh=1` | 通过原有 Native Messaging 通道请求扩展补报近期非敏感媒体索引，再返回列表。 |
| `POST /v1/browser/media/{id}/download` | 请求扩展按不透明媒体 ID 提交下载。 |
| `POST /v1/browser/media/by-page/download` | Bridge 内部按完整页面身份匹配；必要时定向补报索引，再通过扩展提交捕获的 HLS/MP4，不向 MCP 返回敏感信息。 |
| `GET /v1/browser/commands/next` | 原生消息进程长轮询下载和媒体索引补报命令。 |
| `POST /v1/browser/commands/result` | 扩展回复执行结果及下载任务 ID，不包含浏览器会话数据。 |
| `GET /v1/browser/automation` | 可选浏览器组件的安装、启用与交互授权状态。 |
| `POST /v1/browser/automation` | 本地组件安装与交互权限操作。 |
| `POST /v1/browser/automation/call` | 仅允许受限浏览器操作；点击须由本地显式授权。 |

可选浏览器组件安装的是固定版本、经过 SHA-256 校验的上游发行包，存放于私有 `Data` 目录。AI 接口不管理额外 Tunnel，不提供 Cookie 读取、任意网络请求或通用 JavaScript 执行。
