# 使用 Android App

Android 是独立原生客户端，不是桌面 Bridge 的远程界面。当前构建面向 Android 8.0/API 26 及以上的 arm64 设备。APK 是否提供以发行资产为准，桌面 ZIP 不能安装到 Android。

1. 在兼容设备安装本项目构建的 APK。
2. 在 App 内粘贴 MP4 或 HLS 直链，也可通过 Android 分享菜单分享文本链接。分享只预填链接，确认后才开始下载。
3. 查看前台下载通知及完成结果。目前没有桌面端式任务列表。
4. Android 10 及以上通过 MediaStore 输出到公共 `Downloads/VidOtter`；较旧的支持系统保存在应用专属目录。

普通网页地址不等于媒体直链。兼容的浏览器扩展可发现媒体后通过 `vidotter://download?...` 交给 App；Android 浏览器不走桌面 Native Messaging。桌面 Cookie、工具和任务列表不会自动同步到 App。

Android 复用 Go 下载器，使用系统 MediaExtractor/MediaMuxer 封装。不支持的 HLS 格式或时间戳可能导致封装失败，工作目录会保留。目前未集成 yt-dlp、本地字幕模型、分离音视频交接协议或 FFmpeg JNI 封装器；桌面端的 YouTube 能力不能直接套用到移动端。

构建要求与命令见 [Android 开发说明](../../../mobile/android/README.md)。
