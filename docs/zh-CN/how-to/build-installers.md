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

## CI 候选安装包

[安装器 CI 工作流](../../../.github/workflows/installer-smoke.yml)会从已发布的
Release 下载正式归档，核验 SHA-256，然后在 Windows、macOS Intel、Linux amd64
构建离线安装器候选文件。CI 为了可复现使用 `aqtinstall` 安装固定版本
**Qt IFW 4.7**；正式的 Qt IFW 4.11 工具可通过 Qt Online Installer 安装，
还需要另行验收。所有 CI 产物只是短期 Actions 附件，**不会自动发布 Release**。
三个平台均增加了临时目录的命令行安装、卸载和 Data 保留验证；
其中 Linux 已通过，Windows/macOS 验收正在进行。

2026-11-01 已在 CI 中使用完整 v1.6.2 发行包成功构建 Windows x64、
macOS Intel、Linux x64 三个平台的安装器；Linux amd64 通过真实命令行
安装/卸载与数据保留测试。Windows x64、macOS Intel、Linux x64 已在 GitHub Actions
[运行 #41284068572](https://github.com/ThinkerQAQ/vidotter/actions/runs/41284068572)
全部通过原生平台命令行安装/卸载与 Data 保留测试。
图形安装界面及浏览器扩展首次启用仍未完成验证。

**离线升级限制：** 已实测 Qt IFW 的新版离线安装器不能直接安装进
已存在旧版本的目录（报 `TargetDirectoryInUse`）。在完成安全的更新/迁移
流程前，不得宣传一键升级；现阶段请继续使用手动升级文档中的方法。
Qt IFW Maintenance Tool 卸载组件时可以保留 Data，但这不等于一键升级。

## 发布前必须验收

- 三平台生成真正能打开的离线安装器，不以单元测试代替图形验收。
- 带空格路径安装、Bridge 自动启动、扩展连接及授权的公开 MP4 下载。
- 升级前在 `Data/` 放入测试配置和任务，升级后验证未被覆盖。
- 卸载后 Native Messaging 注册消失，`Data/` 原样保留。
- Windows、macOS Intel/Apple Silicon、Linux amd64/arm64 分别验证。
- 审查 Qt IFW GPLv3 及其例外条款、对应源码分发义务与原有第三方许可证；
  校验发布附件 SHA256，并处理需要的代码签名。

验收前，**现有压缩包 + 手动运行 `vidotterdev install` 仍是正式可用方式**。

参见[首次使用](../tutorial/first-run-on-new-machine.md)、
[升级与卸载](upgrade.md)。

## 一键升级的正式实现路线

**使用 Qt IFW Maintenance Tool 更新组件，不再运行新版离线安装器覆盖旧目录。**
最新 Go 构建工具支持 `update-repository`，可通过 Qt `repogen` 从正式版本
发行目录生成 `Updates.xml` 和版本化组件包。发布后，将更新仓库放到可靠的
HTTPS 静态地址，构建安装器时传入 `--update-url https://.../updates/linux/amd64`。
用户即可通过 Maintenance Tool 的“更新”操作完成原地升级：自动重新注册
Native Messaging、保留 `Data/` 和下载文件。无需手动覆盖 `bin/`。

**实际测试：** Linux 在隔离目录安装正式 v1.6.2，然后通过本地仓库升级
到临时构造的 **v1.6.3 测试包**，版本记录更新、Native Messaging 正常、
`Data/config.toml` 保留；还验证了内置 `file://` 更新仓库地址的版本可直接
检查和升级。测试版本不是可公开使用的新 Release。旧离线安装器可以通过
`Maintenance Tool --at file:///... update` 接入临时更新仓库，但不能当作
完整的一键远程更新。

**当前限制：** 稳定的远程更新仓库尚未正式部署、Windows/macOS 升级回归
及 GUI/签名验收尚未完成，因此不能把“自动升级已正式发布”写进产品说明。

MCP Server 已和 Bridge 一起打包，默认不开放远程连接；只有用户主动选择
ChatGPT 集成时才单独配置 Tunnel，详见 [MCP 使用说明](mcp.md)。

### 发布静态更新源（发布维护者执行）

新增 [GitHub Pages 手动发布工作流](../../../.github/workflows/publish-update-feeds.yml)。
只能从受保护的 `main` 分支手动触发，输入已公开、非预发布的 Release 标签，
工作流逐平台校验 Release SHA-256，再生成并部署 Qt IFW 更新仓库。
绝对不上传 CI 使用的临时测试版本。首次运行前需要为 **VidOtter 仓库**
启用 GitHub Pages 的 GitHub Actions 发布模式，并**实际验证**站点地址。
由于个人博客已有自定义域名，不能猜测 URL 或覆盖既有博客页面。

只有确认 HTTPS 站点的 `updates/windows/amd64/Updates.xml`、
`updates/macos/amd64/Updates.xml`、`updates/linux/amd64/Updates.xml`
及相应组件包均可访问后，才在发布打包时使用
`--update-base-url 已验证的地址`；未验证时 CI 默认生成离线安装器。

配置了更新源的版本在 Windows 开始菜单提供 **Update VidOtter**，
Linux 应用菜单提供 **VidOtter Updater**；macOS 可打开安装目录中的
**VidOtter-Maintenance.app**。进入维护工具检查/执行更新即可。
更新前必须先暂停或完成下载任务、关闭活动 Bridge，不自动强杀下载任务。
**旧版手动解压安装迁移：**完成/停止下载任务并备份 `Data/` 后，
可以在图形安装向导中**明确选择旧版所在的原目录**。安装器提示“目录非空”
并要求确认，随后只替换程序文件，不覆盖 `Data/`，并将现有目录纳入 Qt IFW
管理。Linux 已在隔离目录用原有配置哨兵测试通过，Windows/macOS
也已加入 CI 回归。不会静默寻找或覆盖旧目录；若改装到新目录，
则不会自动迁移原目录的用户数据。

### MCP 安装验收

安装器跨平台验收现在会直接启动**已经安装的** Bridge，使用 `--mcp-stdio`
完成 MCP 初始化并检查 13 个工具，升级后再检查一次。此测试不需要
Tunnel、运行密钥或云端连接。普通用户无需手动启动本地 MCP 服务，
只有主动开启 ChatGPT Tunnel 的用户需要配置独立 tunnel-client 和登录时自启。

### 首次安装引导（新版安装器候选）

安装器只有一个 VidOtter 产品，因此取消多余的组件选择页，保留许可、
安装目录和进度页面。**激活 Edge/Chrome 扩展是首次使用的必经步骤，不再
是一个可取消的复选框**。

图形安装成功后点击“完成”，安装器会默认打开本机安装目录中的
`docs/extension-install.html` **离线图文步骤**及 `extension/` 文件夹。
Windows 会同时打开已安装的 Edge（若没有则尝试 Chrome）扩展管理页。
在浏览器中打开开发者模式 → 点击“加载解压缩的扩展” → 选择包含
`manifest.json` 的目录并确认。这不需要扩展商店，也不会把扩展
自动发布到商店；浏览器确认必须由用户本人操作，无法无感绕过。

Windows 开始菜单保留 **VidOtter → Install Browser Extension**，
关闭安装器后也能重新打开该引导。ChatGPT MCP Tunnel 则继续在
**设置 → AI 集成 → ChatGPT MCP** 自行配置，不属于安装步骤。

Linux/macOS 也可在安装完成后打开扩展目录和安装教程；首次加载仍
须由浏览器用户确认。命令行/CI 安装不会启动浏览器或配置 Tunnel。
以上属于新的安装器候选版本，早先的 v1.6.2 ZIP/Actions 安装器
**没有这些新入口**。


### ChatGPT 可选连接由设置页管理（开发分支）

扩展通过 Native Messaging 连接本地 Bridge，用户在设置页输入 Tunnel ID 并点击“配置并启动 Tunnel”确认后，Bridge 才调用受信任的本地脚本，设置用户级 Tunnel。Tunnel ID 在设置页填写，运行密钥只在本机终端输入，不经过扩展。用户需另行安装官方 tunnel-client。设置页若显示“已配置”，只能证明本地有配置文件，不代表 Tunnel 正在运行。正式验收需 doctor、health 和 ChatGPT 调用。该界面需要同时更新 Bridge 与扩展，旧 v1.6.2 正式包尚不具备。

### Windows 卸载入口

新安装器会创建 **开始菜单 → VidOtter → Uninstall VidOtter**，指向同一套 Qt Maintenance Tool，不引入第二套卸载程序。打开后选择 **Remove all components** 并确认。卸载会清理 Bridge、扩展程序文件和 Native Messaging 注册，保留 Data/ 用户配置、任务和下载文件。旧测试安装包若没有快捷方式，可在安装目录运行 VidOtter-Maintenance.exe。

如果看到 VidOtter-Maintenance.exe.new，它可能是 Qt IFW 在 Windows 更新维护工具时暂存的文件，不是新的卸载程序。不要在更新进行时手动删除；如果更新失败，先查看安装目录的 InstallationLog.txt。
