# SniShaper-Linux

Linux cross-platform version of SniShaper — a local proxy tool supporting ECH / TLS-RF / QUIC for censorship circumvention.
--

Windows version: [https://github.com/snishaper/snishaper](https://github.com/snishaper/snishaper)
## Features

- **HTTP/HTTPS Proxy** — CONNECT tunnel and MITM mode
- **SOCKS5 Proxy** — Full SOCKS5 support
- **TLS Record Fragmentation** — Fragments TLS ClientHello to bypass DPI
- **ECH (Encrypted Client Hello)** — Auto-fetch and inject echconfig
- **QUIC Mode** — QUIC protocol obfuscation via quic-go
- **TUN Mode** — System-level traffic capture via mihomo (requires root)
- **Cloudflare IP Pool** — Auto health-check and preferred IPs
- **Auto Routing** — Smart routing based on GFWList
- **Web Dashboard** — Beautiful React + MUI management panel
- **CLI Tool** — Full command-line management

## Quick Start

### Build

```bash
# Build web frontend
cd web
npm install
npm run build
cd ..

# Build Go binary
go build -o snishaper .
```

### Usage

```bash
# Start proxy (foreground)
./snishaper start

# Start with custom port
./snishaper start --port 8888

# Start web dashboard
./snishaper web

# Check status
./snishaper status

# Manage certificates
./snishaper cert status
sudo ./snishaper cert install

# TUN mode (requires root)
sudo ./snishaper tun start
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `snishaper start` | Start proxy server |
| `snishaper stop` | Stop proxy server |
| `snishaper status` | Show status |
| `snishaper web` | Start web dashboard |
| `snishaper config show` | Show configuration |
| `snishaper config import <file>` | Import configuration |
| `snishaper cert status` | Certificate status |
| `snishaper cert install` | Install CA certificate |
| `snishaper cert uninstall` | Uninstall CA certificate |
| `snishaper cert regenerate` | Regenerate CA |
| `snishaper cert export` | Export CA certificate |
| `snishaper tun start` | Start TUN mode |
| `snishaper tun stop` | Stop TUN mode |
| `snishaper tun status` | TUN status |
| `snishaper version` | Show version |

## Web Dashboard

After starting the web panel, open `http://localhost:9090`:

```bash
./snishaper web --port 9090
```

Features:
- Real-time proxy status monitoring
- Traffic statistics
- Certificate management (install/uninstall/regenerate)
- Live log viewer
- Rules browser

## License

MIT License - see [LICENSE](LICENSE)
