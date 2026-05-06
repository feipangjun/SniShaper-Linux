# SniShaper CLI

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux-green?style=flat-square&logo=linux)]()

**SniShaper CLI** 是 SniShaper 的 Linux 命令行版本，专为 Linux 和 WSL 环境设计。它集成了多种技术，包括 **ECH 注入**、**TLS-RF 分片**、**QUIC 重建连接** 以及 **Server 模式轻量中转**，旨在为用户提供稳定的访问体验。

---

## 特性

- **六模式全方位覆盖**：支持从轻量级的 `transparent` 到高级的 `server` 转发，满足不同需求。
- **灵活策略**：
  - **TLS-RF (TLS 分片)**：通过分片规避针对 SNI 的精准阻断。
  - **QUIC 重建**：利用 quic-go 的混淆特性绕过常规 SNI 检测。
  - **ECH 注入**：自动获取并注入 echconfig。
- **优选 IP 与 WARP**：集成 Cloudflare 优选 IP 池与 WARP Masque 隧道。
- **智能分流**：基于 GFWList 自动识别被屏蔽域名，大多数规则外网站无需手动配置即可连接。
- **WSL 兼容**：完美支持 Windows Subsystem for Linux，可在 Windows 主机上使用。

---

## 快速开始

### 1. 下载与构建

```bash
# 克隆仓库
git clone https://github.com/SniShaper/snishaper.git
cd snishaper

# 构建
go build -o dist/snishaper main.go

# 复制默认规则
cp -r rules dist/
```

### 2. 运行

```bash
./dist/snishaper
```

默认配置：
- 代理监听：`0.0.0.0:8080`
- API 服务：`0.0.0.0:5173`

### 3. 证书安装（首次运行需要）

首次启动后，CA 证书会生成在 `~/.snishaper/cert/ca.crt`。需要安装到系统：

```bash
# 复制证书到系统目录
sudo cp ~/.snishaper/cert/ca.crt /usr/local/share/ca-certificates/snishaper-ca.crt
sudo update-ca-certificates
```

或者手动导入浏览器。

---

## 命令行参数

```
Usage:
  snishaper [OPTIONS]

Options:
  -l, --listen string    监听地址 (默认 "0.0.0.0:8080")
  -api string            API 服务地址 (默认 "0.0.0.0:5173")
  -c, --config string    配置目录
  -r, --rules string     规则配置文件
  -s, --settings string  设置配置文件
  -d, --cert-dir string  证书目录
  -m, --mode string      代理模式: mitm, transparent, tls-rf, quic
  -v, --version          显示版本
  -h, --help             显示帮助

Examples:
  snishaper                           使用默认设置启动
  snishaper -l 127.0.0.1:8080        仅监听本地接口
  snishaper -m mitm                   以 MITM 模式启动
  snishaper --version                 显示版本信息
```

---

## API 端点

启动后可通过 HTTP API 访问：

| 端点 | 说明 |
|------|------|
| `/status` | 运行状态 |
| `/stats` | 流量统计 |
| `/logs` | 日志 |
| `/mode` | 当前模式 |
| `/reload` | 重载证书 |
| `/stop` | 停止服务 |

示例：
```bash
curl http://127.0.0.1:5173/status
```

---

## WSL 使用指南

### 在 WSL 中运行，Windows 主机使用

1. **WSL 中启动 SniShaper**：
   ```bash
   ./snishaper
   ```

2. **查看 WSL IP**：
   ```bash
   hostname -I
   ```

3. **Windows 浏览器配置代理**：
   - 代理地址：WSL IP（如 `192.168.1.6`）
   - 端口：`8080`

### 使用 Mirrored 网络模式（推荐）

在 `C:\Users\<用户名>\.wslconfig` 中设置：

```ini
[wsl2]
networkingMode=mirrored
```

重启 WSL 后，Windows 可以直接通过 `127.0.0.1` 访问 WSL 服务。

---

## 构建与开发

### 环境要求

- `Go 1.25+`
- Linux 或 WSL 环境

### 构建

```bash
# 开发构建
go build -o snishaper main.go

# 生产构建（优化体积）
go build -ldflags="-s -w" -o snishaper main.go

# 使用构建脚本
chmod +x build.sh
./build.sh
```

### 项目结构

```
.
├── main.go           # CLI 入口
├── cert/             # 证书管理
├── proxy/            # 代理核心
│   ├── proxy.go      # 主代理逻辑
│   ├── tun_flow.go   # TUN 流量处理
│   └── ...
├── utls/             # TLS 工具库
├── rules/            # 默认规则
├── dist/             # 构建输出
├── build.sh          # 构建脚本
├── LICENSE           # MIT 许可证
└── README.md         # 本文档
```

---

## 文档

想要了解更详细的技术原理、部署教程和自定义指南，请参阅 [**GitHub Wiki**](https://github.com/coolapijust/snishaper/wiki)：

- **[核心模式介绍](https://github.com/coolapijust/snishaper/wiki/Core-Proxy-Modes)**：了解 TLS-RF、QUIC 与 Server 模式的运行原理。
- **[规则自定义指南](https://github.com/coolapijust/snishaper/wiki/Custom-Rules-Guide)**：了解如何开发针对性的规则。
- **[服务端部署](https://github.com/coolapijust/snishaper/wiki/Server-Deployment)**：在 CF Workers 或 VPS 上架设你自己的 Server 节点。
- **[常见问题排除](https://github.com/coolapijust/snishaper/wiki/FAQ)**：解决证书警告、规则不生效等常见问题。

---

## 致谢

本项目受益于以下优秀开源项目的启发：

- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)
- [usque](https://github.com/Diniboy1123/usque)

---

## 许可

[MIT License](LICENSE) - Copyright (c) 2026 JetCPPTeam and SniShaperTeam
