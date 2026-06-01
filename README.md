# SniShaper-Linux

SniShaper 的 Linux 跨平台版本 — 支持 ECH / TLS-RF / QUIC 的本地代理工具。

## 功能特性

- **HTTP/HTTPS 代理** — 支持 CONNECT 隧道和 MITM 模式
- **SOCKS5 代理** — 完整的 SOCKS5 支持
- **TLS Record Fragmentation (TLS-RF)** — 分片 ClientHello 以绕过 DPI 检测
- **ECH (Encrypted Client Hello)** — 自动获取并注入 echconfig
- **QUIC 模式** — 使用 quic-go 进行 QUIC 协议混淆
- **TUN 模式** — 通过 mihomo 实现系统级流量捕获（需要 root 权限）
- **Cloudflare IP 池** — 自动健康检查和优选 IP
- **自动路由** — 基于 GFWList 的智能路由
- **Web 管理面板** — 美观的 React + MUI 仪表盘
- **CLI 工具** — 完整的命令行管理

## 快速开始

### 编译

```bash
# 编译 Web 前端
cd web
npm install
npm run build
cd ..

# 编译 Go 程序
go build -o snishaper .
```

### 使用

```bash
# 启动代理（前台）
./snishaper start

# 启动代理（指定端口）
./snishaper start --port 8888

# 启动 Web 管理面板
./snishaper web

# 查看状态
./snishaper status

# 管理配置
./snishaper config show

# 管理证书
./snishaper cert status
sudo ./snishaper cert install

# TUN 模式（需要 root）
sudo ./snishaper tun start
```

## CLI 命令

| 命令 | 说明 |
|------|------|
| `snishaper start` | 启动代理服务器 |
| `snishaper stop` | 停止代理服务器 |
| `snishaper status` | 查看状态 |
| `snishaper web` | 启动 Web 管理面板 |
| `snishaper config show` | 显示配置 |
| `snishaper config import <file>` | 导入配置 |
| `snishaper cert status` | 证书状态 |
| `snishaper cert install` | 安装 CA 证书 |
| `snishaper cert uninstall` | 卸载 CA 证书 |
| `snishaper cert regenerate` | 重新生成 CA |
| `snishaper cert export` | 导出 CA 证书 |
| `snishaper tun start` | 启动 TUN 模式 |
| `snishaper tun stop` | 停止 TUN 模式 |
| `snishaper tun status` | TUN 状态 |
| `snishaper version` | 显示版本 |

## Web 管理面板

启动 Web 面板后，浏览器访问 `http://localhost:9090`：

```bash
./snishaper web --port 9090
```

功能：
- 实时代理状态监控
- 流量统计
- 证书管理（安装/卸载/重新生成）
- 实时日志查看
- 规则列表浏览

## TUN 模式

TUN 模式通过 mihomo 实现系统级透明代理：

```bash
# 需要 root 权限
sudo ./snishaper tun start

# 停止
sudo ./snishaper tun stop
```

需要预先安装 mihomo 二进制文件到 `core/mihomo/mihomo`。

## 证书管理

SniShaper 使用自签名 CA 证书进行 MITM 代理。首次使用需要安装 CA 证书：

```bash
# 查看证书状态
./snishaper cert status

# 安装到系统信任存储（需要 sudo）
sudo ./snishaper cert install

# 导出 CA 证书供其他设备使用
./snishaper cert export snishaper-ca.crt
```

## 配置文件

- `config/settings.json` — 应用设置
- `rules/config.json` — 代理规则

## 与 Windows 版的区别

| 特性 | Windows | Linux |
|------|---------|-------|
| GUI | Wails WebView | CLI + Web |
| 系统代理 | Registry | 手动配置 |
| 证书安装 | certutil/PowerShell | update-ca-certificates |
| 自启动 | Registry Run | systemd/XDG autostart |
| TUN | mihomo + WinDivert | mihomo + tun |

## License

MIT License - 详见 [LICENSE](LICENSE)
