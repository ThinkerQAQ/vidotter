# 构建并验证源码

本教程验证源码并生成 Bridge 程序，不会注册浏览器主机，也不会替换已有安装。

## 环境要求

- Git、Go 1.24 或更高版本，与两个 Go 模块要求一致。
- Node.js，用于 JavaScript 语法与扩展测试。扩展使用普通 JavaScript，不需要 npm install。
- 首次缓存缺失时需联网下载 Go 依赖。
- 桌面验收还需要 Chrome/Edge，媒体合并需要 FFmpeg。

```sh
git clone https://github.com/ThinkerQAQ/vidotter.git
cd vidotter
go version
node --version
go -C tools/vidotterdev run . verify
go -C tools/vidotterdev run . build
```

`verify` 运行两个 Go 模块的测试、扩展 JavaScript 语法检查和扩展测试。通过这些检查不代表浏览器连接、媒体下载或 Android 构建已经验证。

`build` 默认使用当前系统、架构，输出 `.build/out/vidotter-<os>-<arch>[.exe]`。macOS 此处使用 Go 的系统名 `darwin`，发行包名称则使用 `macos`。

## 交叉编译

```sh
go -C tools/vidotterdev run . build --os windows --arch amd64
```

此命令只编译 Bridge，不包含第三方工具、浏览器注册或安装器程序。单个 Go 构建产物不等于完整发行包。

## 尝试开发安装

使用独立目标目录，避免替换日常安装：

```sh
go -C tools/vidotterdev run . deploy --target /absolute/path/to/vidotter-dev
```

命令构建并复制程序文件和维护程序，不准备随包媒体工具，也不自动注册。在目标操作系统上，从目标根目录运行安装后的维护程序 `install` 命令，再加载该目录的 `extension/`。Windows 应使用 Windows 程序，而不是 WSL 的 Linux 构建。

WSL 部署到 Windows 时，明确指定 `--os windows --arch amd64` 和 `/mnt/c/...` 目标。替换前停止目标 Bridge，已有 `Data/` 会保留。不同安装根目录的数据相互独立，但同名浏览器主机注册会指向最后注册的目录。

修改 JS、HTML、CSS、manifest 或语言文件后，重新加载浏览器扩展。在“设置 → 关于”核对扩展和 Bridge 版本，再下载一个有权保存的媒体并检查输出。使用独立开发目录后，必要时重新注册日常安装目录。

打包与原生依赖见[工程维护](../architecture/maintenance.md)，Android 见[开发说明](../../../mobile/android/README.md)。
