# 组件与数据

VidOtter 有两条运行路径：桌面端的扩展与 Bridge，以及原生 Android App。安装桌面 Bridge 不会把其工具同步到 Android。

| 组件 | 职责 | 管理的数据 |
| --- | --- | --- |
| 浏览器扩展 | 发现页面媒体，收集对应会话，提交任务 | 临时页面检测结果与浏览器会话访问 |
| Go Bridge | 验证本机请求，管理任务和设置，启动 Worker | `Data/config.toml`、任务及运行状态 |
| Worker | 解析、下载、封装、处理字幕 | 工作检查点与输出文件 |
| FFmpeg / yt-dlp / whisper.cpp / llama.cpp | 提供所选流程需要的可选能力 | 工具程序和单独选择的模型 |
| Android App | 接收媒体链接，通过 Go 下载、Android API 封装 | App 状态及导出的媒体 |

## 桌面界面

Popup、Side Panel 和完整 Web UI 复用扩展界面模块。关闭弹窗不等于暂停 Bridge 任务。Web UI 地址为 `http://127.0.0.1:17891/console/`，需要通过扩展中继执行认证操作。页面能显示，不代表中继已经连通。

Native Messaging 主机名为 `com.vidotter.bridge`。生成的 JSON 清单告诉浏览器允许启动哪个程序、接受哪些扩展 ID。HTTP Token 由 Native Messaging 交给扩展，不是用户设置，也无需粘贴到网页。

## 持久化数据

程序位于 `bin/` 时，数据放在其父目录的 `Data/`；开发程序不在 `bin/` 时，则在可执行文件旁创建 `Data/`。数据位置不由终端当前目录决定。

任务快照保存 URL 和输出信息，但移除 Cookie 与代码识别的敏感授权头。Bridge 重启后，受保护任务可能需要重新获取原页面会话。见[任务恢复](../how-to/manage-downloads.md)和[配置](../reference/configuration.md)。
