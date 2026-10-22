# Android 精简 FFmpeg

这里生成供 JNI 封装器使用的 libav `.so`，不是 Android 命令行 ffmpeg。当前 App 仍使用系统 MediaExtractor/MediaMuxer；构建这些库不会自动把 FFmpeg 接入 App。

## 依赖与命令

在 Linux、macOS 或 WSL 中准备 FFmpeg 源码、make、JDK、Android SDK/NDK。维护程序会检查 Android 环境；从仓库根目录运行：

```sh
export ANDROID_SDK_ROOT=/path/to/android-sdk
export ANDROID_NDK_HOME=/path/to/android-sdk/ndk/version
go -C tools/vidotterdev run . ffmpeg-android --source /path/to/FFmpeg
```

默认 ABI 为 `arm64-v8a`，编译 API 为 24。指定多个 ABI 时使用逗号分隔：

```sh
ANDROID_ABIS=arm64-v8a,armeabi-v7a,x86_64 go -C tools/vidotterdev run . ffmpeg-android --source /path/to/FFmpeg
```

产物位于 `.build/artifacts/android/ffmpeg/jniLibs/<abi>/`；`--output /absolute/output/path` 可修改输出根目录。库的编译 API 不等于 App 的最低系统版本（当前 App 为 API 26）。

## 集成要求

复制原生库到 Android 模块对应 `src/main/jniLibs/<abi>/`，实现 JNI wrapper，再接入 Go 的 `PlatformMuxer`；这些是后续集成工作，不是当前命令已完成的功能。按每个分发 ABI 准备库并在设备上测试。

配置关闭网络和转码，Go 负责下载与解密，FFmpeg 只读取本地清单和分片进行 stream copy。发布时同时提供许可证、版本、构建配置和对应源码；改变启用的组件后重新核对分发许可。见[维护指南](../../docs/zh-CN/architecture/maintenance.md)。
