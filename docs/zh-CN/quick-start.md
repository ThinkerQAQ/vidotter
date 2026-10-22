# 快速入门

此路径使用桌面发行包。从源码运行请看独立的[开发教程](tutorial/development.md)。

1. 从 [Releases](https://github.com/ThinkerQAQ/vidotter/releases) 下载对应系统、CPU 的压缩包，完整解压到固定目录。
2. 在该目录运行安装命令：Windows PowerShell 使用 `.\bin\vidotterdev.exe install`，Linux/macOS 使用 `./bin/vidotterdev install`。Linux/macOS 还需将 FFmpeg 安装到 PATH，用于合并、封装。
3. 打开 `chrome://extensions` 或 `edge://extensions`，开启开发人员模式，选择加载解压缩的扩展，指向 `extension/`。
4. 打开 VidOtter，在设置中检查 Bridge 连接。浏览器会按需启动 Bridge，无需保持终端运行。
5. 播放视频，在资源嗅探中选择完整 MP4/HLS 资源，字幕保持“不下载字幕”，提交一次。在下载任务等待完成并检查实际文件。

安装或连接步骤不清楚时，按[首次使用](tutorial/first-run-on-new-machine.md)操作；不确定如何选资源时，看[首次下载](tutorial/first-download.md)。

下一步：[工具与模型](how-to/configure-tools.md) · [字幕](how-to/subtitles.md) · [工作台](how-to/use-web-ui.md) · [任务恢复](how-to/manage-downloads.md) · [故障排查](how-to/troubleshoot.md)。
