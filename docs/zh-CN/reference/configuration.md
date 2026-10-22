# 桌面配置参考

Bridge 读取安装 `bin/` 同级的 `Data/config.toml`。优先通过界面修改，由程序验证后保存。直接改文件需要重启 Bridge；运行中的 Worker 可能保留任务提交时的配置快照，修改后用新任务验证。

## 字段与默认值

| 配置项 | 默认值 | 可用值或作用 |
| --- | --- | --- |
| `download.output_dir` | 安装目录 `Data/downloads/` | 非空输出路径，建议可写的绝对路径 |
| `download.concurrent` | `12` | 1–64 的整数，下载器并发 |
| `download.quality` | `""` | 默认选择、`best` 或 `720` / `720p` 等正整数高度 |
| `proxy.enabled` | `false` | 启用明确配置的 HTTP 代理 |
| `proxy.url` | `""` | HTTP 代理 URL，与主机、端口一起规范化 |
| `proxy.host`, `proxy.port` | `""`, `0` | 主机和端口，配置时端口范围 1–65535 |
| `tools.ffmpeg`, `tools.yt_dlp` | `""` | 程序覆盖路径，留空自动查找 |
| `tools.whisper`, `tools.llama` | `""` | whisper-cli、llama-server 路径 |
| `tools.whisper_model`, `tools.translation_model` | `""` | 本地模型文件路径 |
| `interface.open_mode` | `popup` | `popup` 或 `sidePanel` |
| `interface.language` | `zh-CN` | `zh-CN` 或 `en` |
| `interface.show_logs` | `true` | 显示日志导航，与文件日志独立 |
| `sniffer.hls`, `sniffer.dash`, `sniffer.mp4` | 均为 `true` | 至少保留一种检测类型 |
| `logging.enabled` | `true` | 写入结构化事件日志 |
| `logging.max_size_mb` | `10` | 单个活动组件日志上限，1–100 MB |

代理支持 HTTP，HTTPS 目标通过 CONNECT 访问；不支持 SOCKS 或携带账号密码的代理 URL。主机、端口需同时填写，主机字段不含协议和端口。URL 与主机、端口冲突时应通过界面重新保存。

## 完整示例

替换为自己的输出路径。Windows 可使用 `'C:\Users\YourName\Downloads\VidOtter'`，TOML 单引号字符串保留反斜杠。

```toml
[download]
output_dir = '/home/yourname/Downloads/VidOtter'
concurrent = 12
quality = ""

[proxy]
enabled = false
url = ""
host = ""
port = 0

[tools]
ffmpeg = ""
yt_dlp = ""
whisper = ""
whisper_model = ""
llama = ""
translation_model = ""

[interface]
open_mode = "popup"
language = "zh-CN"
show_logs = true

[sniffer]
hls = true
dash = true
mp4 = true

[logging]
enabled = true
max_size_mb = 10
```

不要用局部片段覆盖已有配置。加载器解码整份文件并验证；TOML 损坏或配置无效时使用默认值，但不覆盖错误文件。手写时遗漏字段也可能导致验证失败，编辑前先备份。

## 运行文件

| `Data/` 下的位置 | 用途 |
| --- | --- |
| `config.toml` | 用户设置 |
| `bridge.json` | 运行 PID、地址和认证 Token，不应分享 |
| `jobs.json` | 任务快照，包含 URL 和输出信息，不是脱敏导出 |
| `native-host/com.vidotter.bridge.json` | 生成的浏览器注册清单 |
| `models/`, `tools/` | 下载模型与可选组件 |
| `work/` | 下载断点及处理文件 |
| `logs/` | Bridge、Worker 结构化事件及轮转 |
| `downloads/` | 默认输出目录 |

结构化事件仅接受任务 ID、阶段、数量、耗时等固定字段，排除 URL、Cookie、Token、标题、路径、字幕和原始错误。其他运行文件及手动采集的终端输出仍可能包含敏感信息。日志轮转到 `.1`，界面读取有限的近期尾部，而非全部历史。

定义见 `bridge/bridge_config.go`、`bridge/runtime_layout.go`、`bridge/bridge_job_store.go`。操作见[工具](../how-to/configure-tools.md)及[升级](../how-to/upgrade.md)。
