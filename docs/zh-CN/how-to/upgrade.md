# 升级、移动与卸载

## 升级桌面安装

1. 完成或暂停活动任务，替换程序前停止旧 Bridge。Windows 上运行中的可执行文件会被锁定。
2. 备份安装目录中的 `Data/`，以及需要保留的外部媒体文件。
3. 将新版完整解压到单独目录。配套替换 `bin/`、`extension/` 和随包 `tools/`，保留原 `Data/`，不要用另一份空数据覆盖它。
4. 在最终安装根目录按[首次使用](../tutorial/first-run-on-new-machine.md)运行对应系统的新版 `vidotterdev install`。
5. 到浏览器扩展页重新加载扩展。如果目录改变，应加载新目录下的 `extension/`，避免继续使用旧副本。
6. 打开 VidOtter，在“设置 → 关于”核对扩展与 Bridge 版本，检查工具和下载目录，测试一个下载。

安装器负责写入 Native Messaging 注册，不会替你迁移任意旧数据目录、重命名历史配置、复制模型，或更新浏览器已加载的扩展。

## 移动安装目录

整体移动程序和 `Data/` 后，在新根目录重新安装。检查设置中的绝对路径：自定义下载目录、工具覆盖路径、模型路径可能仍指向旧位置，需要重新选择。安装目录以外的媒体不会自动移动。

## 卸载

在安装根目录运行 Windows 的 `.\bin\vidotterdev.exe uninstall`，或 Linux/macOS 的 `./bin/vidotterdev uninstall`，再从浏览器移除扩展。命令注销 Native Messaging；Windows 还会删除 VidOtter URL 协议注册。下载文件和 `Data/` 会保留，需要的文件另存后，再手动删除安装目录。
