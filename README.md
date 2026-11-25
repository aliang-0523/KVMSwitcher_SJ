# KVMSwitch

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/aliang-0523/KVMSwitcher_SJ)](https://github.com/aliang-0523/KVMSwitcher_SJ/releases/latest)
![Platforms](https://img.shields.io/badge/platform-Windows%20x64%20%7C%20macOS%20Apple%20silicon-lightgrey)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8)

**一个纯软件的 KVM：让一台 Windows 电脑和一台 Mac 共用一台显示器、一套键盘鼠标。**

按一下热键（默认 `Ctrl+F12`），显示器输入口、键盘、鼠标一起切到另一台设备，剪贴板里的文本、图片、文件也跟着过去。不需要 KVM 切换器，也不需要额外的硬件。

> English summary: [jump to the bottom](#english).

**下载最新稳定版：** [Windows x64 安装包（MSI）](https://github.com/aliang-0523/KVMSwitcher_SJ/releases/latest/download/KVMSwitch-Setup.msi) · [Mac 版（Apple 芯片，ZIP）](https://github.com/aliang-0523/KVMSwitcher_SJ/releases/latest/download/KVMSwitch-mac.zip) · [全部版本](https://github.com/aliang-0523/KVMSwitcher_SJ/releases)

## 特性

- **一键切换**：显示器通过 DDC/CI 指令换输入口，键鼠输入同时转到另一台设备。热键可自定义，另有紧急热键随时拿回控制。
- **对等设计，没有主从**：键鼠插在哪台都可以，两台上是同一个操作面板，在任意一台改设置都会自动同步到另一台。
- **剪贴板共享**：文本、图片、文件（含文件夹）。切换时在后台同步，就绪后正常粘贴即可；图片文件可以同时作为文件和图片粘贴。
- **手动发文件**：在面板里拖入文件直接发到另一台，逐文件校验 SHA-256，有进度、取消和重试。
- **键位映射**：PC 键盘的 `Ctrl` / `Alt` / `Win` 与 Mac 的 Control / Option / Command 对应关系可选，用 PC 键盘在 Mac 上也能按 `Ctrl+C` 复制。
- **媒体键**：音量加减、静音、播放/暂停、上一首/下一首双向转发。
- **加密通信**：仅在局域网内工作，所有流量用配对密钥派生的 AES-256-GCM 加密，不经过任何服务器。
- **应用内更新**：从 GitHub Releases 检查新版，校验 SHA-256 后安装。

## 工作原理

```
          ┌──────────── 同一台显示器 ────────────┐
          │  HDMI                          DP   │
   ┌──────┴──────┐   局域网（TCP，加密）  ┌──────┴──────┐
   │   Windows   │ ◀──────────────────▶ │     Mac     │
   │  KVMSwitch  │  键鼠 / 剪贴板 / 文件 │  KVMSwitch  │
   └──────┬──────┘                      └─────────────┘
          │
      键盘 + 鼠标（插在哪台都行）
```

1. 插着键鼠的那台设备在系统层截获本机键鼠输入（Windows 用低级键盘/鼠标钩子，Mac 用 CGEventTap）。
2. 切到另一台时，输入不再交给本机，而是通过局域网发给另一台，由那台用系统接口注入（`SendInput` / `CGEventPost`）。
3. 两台同时尝试用 DDC/CI 让显示器换到目标输入口，任意一台成功即可；都失败则撤销这次切换。
4. 切换时，发出方把剪贴板内容的摘要告诉接收方，接收方在后台拉取并写入本机剪贴板。

两端程序都用 Go 编写。Mac 端通过 [purego](https://github.com/ebitengine/purego) 直接调用系统框架，不需要 cgo，因此两端都可以在 Windows 上编译。

## 系统要求

| 项目 | 要求 |
|---|---|
| Windows | Windows 11 x64（Windows 10 需另装 WebView2 Runtime） |
| Mac | Apple 芯片（arm64）。Intel Mac 暂不支持 |
| 显示器 | 支持 DDC/CI 并已在显示器菜单中开启；有两个以上输入口 |
| 网络 | 两台在同一局域网，推荐有线；放行 TCP 47812 和 UDP 47813 |

## 快速开始

### 1. 接线

| 设备 | 接到显示器的哪个口 |
|---|---|
| Windows | HDMI |
| Mac mini | 显示器的 **DP 口**，Mac 这边用 **USB-C/雷雳口**（USB-C 转 DP 线） |

Mac 建议走 USB-C/雷雳口而不是自带的 HDMI 口：部分 Mac 的 HDMI 口不能发送切换显示器的指令。显示器菜单里如果有“DDC/CI”选项，请确认是开启的。

### 2. 安装

**Windows**：运行 MSI 安装（按当前用户安装，不需要管理员权限），也可以直接双击便携版 `kvmswitch.exe`。首次运行时防火墙会询问是否允许联网，选**允许（专用网络）**。

**Mac**：

1. 解压 `KVMSwitch-mac.zip`，把 `KVMSwitch.app` 拖进“应用程序”文件夹。
2. 双击打开。发布包使用临时签名（ad hoc），**没有 Apple Developer ID 签名和公证**，系统可能提示无法验证开发者；确认来源可信后，在“系统设置 › 隐私与安全”中选择“仍要打开”。如果提示“已损坏”，先检查签名完整性：

   ```bash
   codesign --verify --deep --strict --verbose=2 /Applications/KVMSwitch.app
   ```

   报告资源缺失或签名无效时，请从 Releases 重新下载。
3. 按提示授权：在 **系统设置 › 隐私与安全 › 设备控制和数据访问** 里开启 KVMSwitch（部分旧版 macOS 此项名为“辅助功能”）；弹出“查找本地网络上的设备”时选**允许**。
4. 建议开启自动登录：登录界面无法被另一台设备的键鼠控制。

Mac 端运行后在菜单栏显示 `KVM ○`（未连接）、`KVM ●`（键鼠在本机）、`KVM ◐`（键鼠在另一台设备），点它可以打开面板。

### 3. 配对

只需要做一次，方向随意：

1. 在任意一台的面板里打开**连接**页，点“本机密钥”旁的**复制**。
2. 在另一台的**连接**页，把它填进“另一台设备的密钥”，点**保存并连接**。

几秒后两边都会显示“已连接”。局域网自动发现不通时，在 Mac 面板“连接”页手动填写 Windows 的 IP。

### 4. 设置显示器

1. 如果机器接了多台显示器，先到 **显示器 › 本机共享屏** 选择共用的那一台（两台各自设置）。
2. 打开 **显示器 › 输入口识别向导**，程序会逐个试切输入口，你根据显示器画面选择 Windows、Mac 或无信号。两台都识别出来后才会保存。
3. 如果试切后画面没有自动切回，用显示器上的实体按键恢复。也可以手动选择输入口。

## 使用

| 操作 | 默认按键 |
|---|---|
| 在两台设备间切换 | `Ctrl+F12` |
| 紧急拿回 | `Ctrl+Alt+Shift+F12`：在哪台设备的键盘上按，键鼠和显示器就回到哪台 |

- 也可以点面板概览页的切换按钮，或托盘 / 菜单栏菜单里的“切换”。
- **修饰键**：默认 PC 键盘的 `Ctrl`↔Control、`Alt`↔Option、`Win`↔Command，所以用 PC 键盘在 Mac 上复制粘贴是 `Win+C` / `Win+V`。想让 `Ctrl+C` 两边通用，在 **键盘与鼠标 › 跨设备操作键位** 选择 **Windows 操作键位**。这个对应关系双向生效。
- **剪贴板**：切换时在后台同步新复制的内容，就绪后用快捷键或右键菜单粘贴。如果在接收过程中就按了粘贴，传完时只要焦点没变、期间没有其他输入，会自动补上这次粘贴。
- **文件**：剪贴板里的文件会在切换时传到接收端缓存，进度显示在接收端的“剪贴板”页。超过上限（默认 1 GB）会弹窗确认。跨机**剪切一律按复制处理**，源文件不会被移动或删除。
- **图片**：单个 PNG、JPEG、GIF、BMP、TIFF、WebP、HEIC/HEIF、AVIF、静态 SVG 等图片文件，可以同时作为原文件和图片粘贴。预览上限 64 MiB、约 2500 万像素。
- **手动发送文件**：在 **概览** 页拖入或多选文件，点“发送到…”。接收方无需确认，文件存到本机设置的接收目录（默认 `~/Downloads/KVMSwitch`），同名文件自动编号，不会覆盖。暂不支持直接发送文件夹，请先压缩。

### 操作面板

两台设备上的面板完全相同。

| 页面 | 内容 |
|---|---|
| 概览 | 键鼠在哪台、一键切换、连接/权限/显示器/剪贴板状态、手动发送文件 |
| 显示器 | 是否随切换换口、本机共享屏、输入口识别向导、测试切换、显示器信息 |
| 键盘与鼠标 | 录制热键、修饰键对应、指针和滚轮速度、媒体键测试 |
| 文件传输 | 本机接收目录、单批上限、暂停接收、最近任务 |
| 剪贴板 | 同步开关、大文件确认阈值、传输进度与重试 |
| 连接 | 本机密钥、另一台设备的密钥、连接状态 |
| 系统与日志 | 开机自启、软件更新、最近日志、退出程序 |

- 设置改完立即生效并同步到另一台。配对密钥、开机自启、共享屏绑定、文件接收目录等属于单台设备的设置不同步。
- 打开面板：Windows 单击托盘图标；Mac 点菜单栏的 `KVM`。再次启动程序也会打开面板。关闭面板窗口不会退出程序。

### 更新

启动后以及每 12 小时会检查 GitHub 上的最新稳定版，检查失败不影响使用。也可以在 **系统与日志 › 软件更新** 手动检查或重装。安装包会先校验大小和 SHA-256，配对和设置不受影响。

- Windows：通过 MSI 安装向导更新。便携版使用此功能会安装 MSI 版，不会替换原来的 EXE。
- Mac：需要应用所在目录可写；替换后会重新校验签名，失败则还原旧版。更新后系统可能要求重新确认键鼠和本地网络权限。

## 常见问题

**切过去之后屏幕回不来**：按紧急热键；还不行就用显示器上的按键手动切回，然后在“显示器”页用“测试切换”和“显示器信息”排查。Mac 无法让显示器换口时，可以安装 [m1ddc](https://github.com/waydabber/m1ddc) 或 BetterDisplay，在 **显示器 › 高级** 里填写切换命令，例如 `/opt/homebrew/bin/m1ddc set input {input}`。

**一直显示未连接**：确认两台都在运行、密钥一致、在同一网段，Windows 防火墙已放行。自动发现不通时，在 Mac 面板“连接”页填写 Windows 的 IP。

**切到 Mac 后键鼠没反应 / Mac 上热键无效**：检查 **系统设置 › 隐私与安全 › 设备控制和数据访问** 中的 KVMSwitch 开关。开关已开但仍提示未授权时，从菜单栏退出后重新打开。如果换过应用文件，请退出 KVMSwitch，从列表里移除旧条目，重新添加 `/Applications/KVMSwitch.app` 并开启。启动日志会显示正在运行的程序路径和授权状态，可以用来核对。

**Mac 上面板窗口打不开**：在“终端”里执行下面的命令，改用浏览器显示面板：

```bash
/Applications/KVMSwitch.app/Contents/MacOS/kvmswitch-mac --browser
```

**音量旋钮切到 Mac 后不起作用**：在 Windows 面板 **键盘与鼠标 › 测试媒体键** 点“监听 60 秒”，切到 Mac 后转动旋钮，再切回查看结果。显示“对端已收到”但音量不变时，可能是当前输出设备（例如部分 HDMI/DP 显示器）不支持软件调音。

**在 Windows 的管理员窗口或 UAC 弹窗里热键无效**：这是系统限制，切到普通窗口再按。

**日志位置**：两端都可以在面板“系统与日志”页查看。文件位于 Windows `%LOCALAPPDATA%\KVMSwitch\kvmswitch.log`，Mac `~/Library/Logs/KVMSwitch.log`。

## 已知限制

- 目前只支持一台 Windows + 一台 Mac（Apple 芯片）的组合。
- 两台设备都要开机并已登录。没有插键鼠的那台在另一台关机时无法操作，建议留一套备用键鼠。
- 控制另一台时，鼠标只在其主显示器范围内移动。
- 亮度键不转发；只调耳机硬件或映射为厂商私有命令的旋钮无法转发。
- Mac 版没有经过 Apple 公证，首次打开需要手动确认。
- 不同大版本之间协议可能不兼容，升级时请两端一起更新（1.3.1 起的协议与更早版本不兼容）。

## 安全说明

- 两台设备之间的所有通信都使用配对密钥派生（HKDF-SHA256）的 AES-256-GCM 加密。没有密钥的设备无法连接，也无法读取流量。
- 程序只在局域网内工作：TCP 47812 用于通信，UDP 47813 用于局域网自动发现。不连接任何第三方服务器，唯一的外网请求是向 GitHub 检查更新。
- 操作面板只监听本机回环地址，并要求每次启动随机生成的访问令牌。
- 持有配对密钥就能控制另一台设备的键鼠，请像保管密码一样保管它，不要发给别人。
- 如果你发现了安全问题，请不要公开提交 Issue，可以通过 GitHub 的 [Private vulnerability reporting](https://github.com/aliang-0523/KVMSwitcher_SJ/security/advisories/new) 私下报告。

## 从源码构建

需要在 **Windows** 上构建，两端程序都由同一台机器交叉编译：

- [Go](https://go.dev/dl/) 1.26 或更高
- [rcodesign](https://github.com/indygreg/apple-platform-rs/releases/tag/apple-codesign/0.29.0)（用于给 Mac 应用签名，已测试 0.29.0；加入 PATH，或用环境变量 `RCODESIGN` 指定路径）
- 生成 MSI 另需 [WiX Toolset 3.x](https://github.com/wixtoolset/wix3/releases)，并把其目录设为环境变量 `WIX_BIN`

```powershell
.\build.ps1             # 生成 dist\kvmswitch.exe 和 dist\KVMSwitch-mac.zip
.\build.ps1 -Installer  # 另外生成 dist\KVMSwitch-Setup.msi
go test ./...           # 运行测试
```

更换应用图标后，先重新生成 Windows 资源文件再编译：

```powershell
go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -ico packaging/KVMSwitch.ico -o cmd/kvmswitch-win/icon_windows_amd64.syso
```

### 代码结构

| 目录 | 内容 |
|---|---|
| `internal/core` | 两端共用的核心：设置与同步、热键、切换状态机、连接、剪贴板同步 |
| `internal/proto` | 加密帧协议 |
| `internal/xfer` | 剪贴板内容（文本、图片、文件）传输 |
| `internal/filedrop` | 手动发送文件 |
| `internal/clipimage` | 图片格式识别与预览生成 |
| `internal/panel` | 两端共用的操作面板（HTML/CSS/JS 与本地 HTTP 接口） |
| `internal/update` | 检查与安装更新 |
| `cmd/kvmswitch-win` | Windows：键鼠钩子与注入、DDC/CI、剪贴板、托盘、WebView2 窗口 |
| `cmd/kvmswitch-mac` | Mac：CGEventTap 与注入、IOAVService DDC、剪贴板、菜单栏、WKWebView 窗口 |
| `packaging`、`tools/pack` | Windows MSI 与 Mac 应用打包 |

## 参与贡献

欢迎提交 Issue 和 Pull Request。

- 报告问题时，请附上两端的版本号、系统版本、显示器型号和连接方式，以及相关日志片段（日志里不包含配对密钥，但请再检查一遍）。
- 提交代码前请运行 `go test ./...`，并尽量在真实的 Windows + Mac 环境里验证改动。
- 改动通信协议时请同时更新两端，并在说明里注明是否与旧版本兼容。

## 版本记录

完整的发布说明见 [Releases](https://github.com/aliang-0523/KVMSwitcher_SJ/releases)。

- **1.3.13**：修复部分 Windows 键盘的音量旋钮切到 Mac 后无效的问题，支持驱动合成的音量键；Mac 优先直接调节当前声音输出设备。请两端一起更新。
- **1.3.12**：修复从 Windows 切到 Mac 后，单按 Q、H 等字母可能误触系统快捷键的问题。
- **1.3.11**：新增独立的双向文件传输；剪贴板改为切换时立即开始同步。
- **1.3.10**：修复跨端粘贴有时需要按两次的问题。
- **1.3.9**：修复 Mac 复制 JPEG 后在 Windows 无法粘贴为图片；新增 SVG、WebP、HEIC/HEIF、AVIF 等图片预览。
- **1.3.8**：修复 Windows 复制 PNG 后在 Mac 应用中无法粘贴为图片。
- **1.3.7**：修复点击托盘后有时无法打开面板。
- **1.3.6**：安装或更新成功后自动打开；Mac 更新改由独立进程验签和替换。
- **1.3.5**：重构双端面板，适配深色模式和窄窗口。
- **1.3.4**：修复 Mac 应用包签名资源缺失，更新前校验完整性。
- **1.3.3**：自动检查稳定版并支持应用内安装。
- **1.3.2**：可选择 Mac 或 Windows 操作键位，支持自定义修饰键对调。
- **1.3.1**：跨机剪切保留原件；输入拥塞处理；传输取消与重试；按显示器绑定切换。协议升级，与旧版不兼容。
- **1.3**：按需剪贴板、显示器输入口向导、Windows WebView2 面板与 MSI 安装包、媒体键转发。
- **1.2**：两台设备对等；Mac 端改为带菜单栏图标和面板的应用；设置双向同步。
- **1.1**：Windows 端新增操作面板。
- **1.0**：热键切换显示器和键鼠，剪贴板与文件同步。

## 许可证

[MIT](LICENSE) © 2026 kejimip

---

<a id="english"></a>

## English

**KVMSwitch** is a software KVM for one Windows PC and one Apple-silicon Mac that share a single monitor, keyboard and mouse.

- Press a hotkey (default `Ctrl+F12`). The monitor switches its input through DDC/CI, and keyboard and mouse input moves to the other machine.
- The clipboard follows you: text, images and files.
- Both machines are equal peers. Plug the keyboard and mouse into either one. Both run the same control panel, and settings sync automatically.
- All traffic stays on your LAN and is encrypted with AES-256-GCM, using a key derived from a shared pairing key.
- Requirements: Windows 10/11 x64, an Apple-silicon Mac, and a monitor with DDC/CI enabled.
- Download it from [Releases](https://github.com/aliang-0523/KVMSwitcher_SJ/releases/latest). Build it from source on Windows with `.\build.ps1`, which needs Go 1.26+ and rcodesign.
- The UI is currently Chinese only. Issues and pull requests in English are welcome.
