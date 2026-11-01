# 三平台统一图形安装器（Qt Installer Framework）

本页是**构建与发布说明**，不是已经上架的一键安装包。在 Windows、
macOS、Linux 真机完成首次安装、升级、卸载的验收并发布相应 Release
附件之前，不应声称“一键安装已支持”。

## 实现思路

三个平台复用同一套 Qt Installer Framework 离线安装器定义。
图形安装器只负责部署发行包，然后调用**现有 Go 安装核心**
`vidotterdev install` 注册 Native Messaging；卸载时调用
`vidotterdev uninstall`。不再创建第二套安装业务逻辑。

| 平台 | 计划发布的文件 | 使用方式 |
| --- | --- | --- |
| Windows | `VidOtter-Setup-v版本-windows-amd64.exe` | 双击运行 |
| macOS | `VidOtter-Setup-v版本-macos-架构.dmg` | 打开磁盘映像中的安装器 |
| Linux | `VidOtter-Setup-v版本-linux-架构.run` | 授予执行权限后在图形会话中运行 |

使用同一套配置，但产物仍然是**不同操作系统的安装文件**。
macOS 公开分发通常还需要考虑签名、公证；未签名的 Windows 程序可能
出现安全提醒，Linux 图形安装要求系统具有兼容的 Qt 平台依赖。

## 打包方式

1. 在对应系统的构建机通过 Qt Online Installer 的 **Qt → Build tools**
   安装 **Qt Installer Framework 4.11.0**，确认
   `binarycreator` 可调用。它只用于开发打包，VidOtter 最终用户无需安装 Qt。
   发布前检查打包后的 Qt 许可和第三方声明。
2. 使用项目现有的 `go -C tools/vidotterdev run . package`
   生成完整发行包，正式发布不得使用 `--allow-missing-tools`。
3. 选择匹配系统与 CPU 的已解包发行目录，执行，例如 Linux amd64：

```bash
go -C tools/vidotterdev run . installer \
  --stage .build/out/release-stage/vidotter-v1.6.2-linux-amd64 \
  --os linux --arch amd64 \
  --output .build/artifacts/installers/VidOtter-Setup-v1.6.2-linux-amd64.run
```

Windows 使用 `--os windows --arch amd64` 并输出 `.exe`；
macOS 使用 `--os darwin --arch amd64/arm64` 并输出 `.dmg`。
可通过 `--binarycreator /绝对路径/binarycreator` 指定构建工具。
**Qt IFW 安装器必须在对应操作系统上构建**，不能在 Linux 上直接
交叉生成 Windows/macOS 安装器。安装源必须包含
`bin/vidotterdev`、匹配系统的 Bridge 和 `extension/`。

打包时明确**排除 Data/**，安装器不主动删除安装目录，Go 卸载器保留
配置、任务、模型、下载文件。升级仍需真机测试：特别是 Windows
正在运行的 Bridge 不能直接覆盖，以及安装目录和扩展 ID 是否保持不变。

## 浏览器扩展首次启用

安装器不能绕过 Chrome/Edge 对非商店扩展的安装限制。用户仍须打开
`edge://extensions` 或 `chrome://extensions`，启用开发者模式，
选择“加载解压缩的扩展”，指向安装位置的 `extension/`。
首次授权后，按当前机制由扩展通过 Native Messaging 按需启动 Bridge。

## 发布前必须验收

- 三平台生成真正能打开的离线安装器，不以单元测试代替图形验收。
- 带空格路径安装、Bridge 自动启动、扩展连接及授权的公开 MP4 下载。
- 升级前在 `Data/` 放入测试配置和任务，升级后验证未被覆盖。
- 卸载后 Native Messaging 注册消失，`Data/` 原样保留。
- Windows、macOS Intel/Apple Silicon、Linux amd64/arm64 分别验证。
- 审查 Qt IFW 和原有第三方许可证，校验发布附件 SHA256，并处理需要的代码签名。

验收前，**现有压缩包 + 手动运行 `vidotterdev install` 仍是正式可用方式**。

参见[首次使用](../tutorial/first-run-on-new-machine.md)、
[升级与卸载](upgrade.md)。
