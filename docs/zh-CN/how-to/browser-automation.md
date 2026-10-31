# 可选浏览器自动化组件

[English](../../how-to/browser-automation.md) · [MCP 工具](mcp.md) · [第三方许可证](../../../THIRD_PARTY_NOTICES.md)

VidOtter 采用**一个产品、一个 MCP 入口、一条私人 Tunnel**。浏览器自动化依赖独立维护的 [Remote Browser MCP](https://github.com/ThinkerQAQ/remote-browser-mcp) 完整发行包，以本地 stdio MCP 子进程方式调用；不复制、搬运、合并或维护其源码分叉。

## 完整流程

1. 用户在 Edge/Chrome 打开并播放媒体（手动操作，或通过可选自动化组件新建后台标签页）。
2. 现有 VidOtter 扩展的 `chrome.webRequest` 监听器捕获播放时请求的 HLS / DASH / MP4 资源。
3. 扩展只将 **资源索引元数据** 发给本地 Bridge：标签页作用域的资源 key、媒体类型、页面标题、页面 URL。Bridge 去除 URL 查询参数和片段，生成随机资源 ID，**只在内存中保存 10 分钟**；不索引签名媒体 URL、Cookie 和认证请求头。
4. `vidotter_list_browser_media` 返回资源 ID、经清理的页面地址及媒体类型。下载前应核对目标页面、标题和剧集，不能盲目取最新一个。
5. `vidotter_download_browser_media` 通过持久 **Native Messaging** 连接向扩展发出请求。扩展从既有 `chrome.storage.session` 中找到对应资源，**在真正提交任务时**按需读取会话信息，继续执行原有任务构造、Bridge 入队与下载。MCP 只返回任务 ID，再用 `vidotter_get_job` 查询进度。

如果网页没发出媒体网络请求，不能从普通 HTML 凭空推断 m3u8。项目不开发视频网站专用解析器，不绕过 DRM、登录或访问控制。媒体权利及合法使用由用户自行负责，参见 README 的法律声明。

## 已验证：单 VidOtter MCP 完整链路（Windows）

2026 年 10 月 31 日，**仅连接 VidOtter MCP** 完成了 Windows 真实端到端测试，无需另行调用 Remote Browser Tool 插件：

1. `vidotter_status` 确认 Bridge `1.5.0` 可用，`vidotter_browser_status` 确认可选 `v0.3.0` 组件已安装并启用。
2. `vidotter_browser_open` 为公开的 [MDN CC0 flower.mp4](https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4) 创建独立后台标签页。另一个公开示例页面还完成了 `vidotter_browser_snapshot` 和 `vidotter_browser_close` 验证。
3. `vidotter_list_browser_media` 返回 `extension_connected=true`、与页面匹配的 MP4 媒体类型及短期 `resource_id`。
4. `vidotter_download_browser_media` 使用该资源 ID 和 `quality=best` 提交任务；`vidotter_get_job` 返回 **completed、100%、1,128,375 字节、1 个输出文件**。
5. `vidotter_browser_close` 关闭测试标签页，未选择或修改其他用户标签页。

**已验证范围：** 浏览器打开网页、读取页面结构、关闭自己创建的标签页、扩展发现媒体、提交任务以及下载 MP4。**未验证：** 自动点击播放、HLS/DASH 全量下载，以及 Linux/macOS 的实际浏览器自动化体验。三平台构建和单元测试通过不等同于三平台真实浏览器验收。

新用户只需要安装 VidOtter 扩展和 Bridge、授权本机浏览器调试，并按需安装 Browser Automation 组件；不用另外安装 ChatGPT 浏览器插件。组件默认关闭，自动点击需要另行手动启用。**资源已经被扩展捕获时，直接查询资源 ID 下载即可，不需要重新打开网页或点击播放器。**

### 补充验收：浏览器优先 HLS 下载（Windows，2026-10-31）

升级到 v1.5.1 后，使用公开的 [hls.js 演示页](https://hlsjs.video-dev.org/demo/) 验证：Edge 扩展发现多条 HLS 资源，直接调用 `vidotter_download(url="https://hlsjs.video-dev.org/demo/", quality="480")` 返回 `route=browser`，任务 `77ce068babe5` 随后达到 `completed`、100%、1 个输出文件，**未调用 yt-dlp 网页解析器**。另以 W3Schools 公开 HTML Video 页验证网页匹配 MP4 后通过原生下载器完成。此结果仅代表所列样例通过，不保证所有站点或编码兼容。

## 已连接 Remote Browser Tool 时

如果 ChatGPT 已经连接并正常使用 Remote Browser Tool MCP，**直接复用现有插件**打开后台标签页、查看页面和操作媒体播放，再用 VidOtter 的 `vidotter_list_browser_media` 和 `vidotter_download_browser_media` 完成媒体发现与下载。VidOtter 扩展无需区分页面是由哪个工具打开的。

**这种情况下不要同时启用或调用 VidOtter 内置的 Browser Automation 子组件**，否则会额外建立一个 CDP 客户端，Edge/Chrome 可能再次要求授权。双 MCP 协作无需额外安装浏览器自动化组件，也不会复制 Remote Browser Tool 的代码。

对于没有独立 Remote Browser Tool 的用户，下方的可选组件仍可提供单产品安装、单 VidOtter MCP 的能力，但首次连接仍须遵守浏览器自身的授权要求。

## 用户只需安装 VidOtter

1. 正常安装 VidOtter，并加载浏览器扩展。基础下载无需 Node.js，也不依赖 Remote Browser MCP。
2. 在扩展 **设置 → 组件** 找到 **Browser Automation → 安装并启用**。VidOtter 下载固定的 **Remote Browser MCP v0.3.0** 发行包并验证 SHA-256；使用已安装的 Node.js 22+，否则从 nodejs.org 下载并验证 **Node.js v24.22.2 LTS** 的私有运行环境。npm 使用上游锁文件进行完整性验证，禁用安装脚本；所有文件位于本地 `Data/components/`，不需管理员权限。
3. 在 Edge/Chrome 为本机 **127.0.0.1:9222** 开启调试功能，必要时由本人确认浏览器授权。安装器不能自动取得该权限，更不会绕过浏览器安全提示。
4. ChatGPT **只需连接 VidOtter MCP**。浏览器自动化不需要另外配置 Tunnel 或安装另一个 ChatGPT 插件。
5. 确需 AI 自动点击播放器时，再在本机组件管理中显式开启**允许自动点击**。默认关闭，且必须通过本地确认框。

VidOtter 不复用你单独安装的 Remote Browser Tool 的配置、Tunnel、密钥和浏览器用户目录，而是为内置可选组件生成自己受限的配置。底层限制已有标签页访问和任意 JavaScript，VidOtter 再通过白名单缩减对外 MCP 工具。

## 默认权限

| 操作 | 默认策略 |
| --- | --- |
| 查看组件与连接状态 | 允许 |
| 新建公开网页的后台标签页 | 启用组件后允许 |
| 读取和关闭组件创建的标签页 | 启用组件后允许 |
| 点击自己新建的页面 | **需要单独在本地授权** |
| 访问/控制用户原有标签页 | **禁止** |
| 任意 JavaScript、浏览器存储/Cookie 读取、文件上传 | **禁止** |
| AI 直接查看网络原始请求和认证头 | **禁止** |
| 向 GitHub 提交用户数据、签名 URL、Cookie、私有配置 | **禁止** |

即使只能点击自己创建的页面，点击动作仍可能触发提交表单等副作用；不能仅凭控件文字断言安全。不得用自动点击执行登录、购买、账户管理等操作。浏览器调试权限和媒体使用均须遵守适用权限与法律。

## 内部依赖关系

```text
ChatGPT
  |
  +-- VidOtter MCP （唯一的私人 Tunnel）
        |
        +-- Go Bridge ----- 现有媒体下载引擎
        |      ^
        |      | 本地 Native Messaging + 认证 HTTP
        |      |
        +-- VidOtter Extension ---- 浏览器会话媒体索引
        |
        +-- 可选 Browser Adapter （固定白名单）
               |
               +-- Remote Browser MCP v0.3.0 （独立发行版，stdio）
                      |
                      +-- Chrome DevTools MCP / Edge / Chrome
```

两个开源项目继续独立迭代。升级/停用浏览器组件不能覆盖 `Data/config.toml`、用户下载目录或其他软件的浏览器配置。

## 常见问题

- **没有媒体资源**：确认扩展已更新、已开始实际播放；页面只有 HTML 不代表存在可下载 m3u8。
- **`extension_connected=false`**：检查 Native Messaging 状态，重新加载扩展或确保 Bridge 正在运行。
- **没有浏览器自动化组件**：在设置的组件列表安装；手动播放与普通下载不受影响。
- **浏览器调试不可用**：在 Edge/Chrome 本机开启并授权 9222 调试端口。
- **自动点击被拒绝**：到 VidOtter 本地设置中开启互动权限，AI 不能代替用户开启。
- **浏览器自动化请求超时**：适配器不会自动重启 CDP 子进程或重试，失败会进入 30 秒重连冷却期，以免反复触发浏览器授权。再次尝试前先检查已有的 Remote Browser Tool 连接与浏览器授权。
- **网页 URL 被 yt-dlp 拒绝**：应让扩展捕获播放时真正的媒体请求，再按资源 ID 下载，而非把网页 HTML 当成 m3u8。
