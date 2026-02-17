# Linux Knob Controller

[![AUR](https://img.shields.io/aur/version/knob-control)](https://aur.archlinux.org/packages/knob-control)

---

## 功能

- **独占控制**：接管旋钮事件，消除与 GNOME/KDE 音量弹窗的逻辑冲突。
- **音频控制** (`Mode Audio`)
  - **单击**：切换静音/取消静音（多设备时为切换物理输出设备并取消静音）。
  - **旋转**：调整系统音量。
- **亮度控制** (`Mode Brightness`)
  - **双击**：切换到此模式（通知提示 `Mode: Brightness`）。
  - **单击**：切换控制的显示屏（多屏支持）。
  - **旋转**：调整显示屏亮度（支持 DDC/CI 和 笔记本背光）。
- **应用音量控制** (`Mode App Volume`)
  - **三击**：切换到此模式（通知提示 `Mode: App Volume`）。
  - **单击**：切换当前控制的应用程序。
  - **旋转**：调整选中应用程序的音量。

默认排除 HDMI/DP 音频，需要的话可以带`-hdmi`参数。

## 安装

### 1. Arch Linux (AUR)

**源码包**：
```bash
yay -S knob-control
```

### 2. 手动编译安装

需要 Go 环境：
```bash
git clone https://github.com/Aurora-Nasa-1/volume-control.git
cd volume-control
go build -o volume-knob-control .
sudo install -m 755 volume-knob-control /usr/local/bin/volume-knob-control
```

## 配置与使用

### 权限配置
需要将用户加入 input 用户组以读取输入设备：
```bash
sudo usermod -aG input $USER
# 重启或重新登录生效
```
若使用 DDC 控制亮度，可能还需要 i2c 权限：
```bash
sudo usermod -aG i2c $USER
```

### 运行参数
```bash
volume-knob-control -help
# -device string: 指定输入设备路径 (默认自动搜索 "Consumer Control")
# -step string: 调整步长 (默认 "2%")
# -hdmi: 包含 HDMI/DP 音频输出
# -verbose: 显示详细日志
```

### 开机自启
建议配置为 Systemd 用户服务或桌面环境自启动项。
