package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := strings.ToLower(os.Args[1])

	switch cmd {
	case "version", "-v", "--version":
		fmt.Printf("SniShaper-Linux v%s\n", version)
	case "start":
		cmdStart(os.Args[2:])
	case "stop":
		cmdStop()
	case "status":
		cmdStatus()
	case "web":
		cmdWeb(os.Args[2:])
	case "config":
		cmdConfig(os.Args[2:])
	case "cert":
		cmdCert(os.Args[2:])
	case "tun":
		cmdTUN(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`SniShaper-Linux - TLS Proxy with ECH/TLS-RF/QUIC Support

Usage:
  snishaper <command> [options]

Commands:
  start       Start proxy + web dashboard (auto on 127.0.0.1:5173)
  stop        Stop the proxy server (daemon mode)
  status      Show proxy and TUN status
  web         Start web dashboard only (default 127.0.0.1:5173)
  config      Manage configuration
  cert        Manage CA certificate
  tun         Manage TUN mode
  version     Show version
  help        Show this help

Examples:
  snishaper start                    Start proxy + web on 127.0.0.1:5173
  snishaper start --no-web           Start proxy only, no web dashboard
  snishaper start --port 8888        Start proxy on port 8888
  snishaper web                      Start web dashboard on 127.0.0.1:5173
  snishaper web --bind 0.0.0.0       Start web on all interfaces
  snishaper cert install             Install CA to system trust store
  snishaper tun start                Start TUN mode (requires root)
  snishaper config show              Show current configuration`)
}

func getConfigDir() string {
	execPath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(execPath)
}

func setupSignalHandler(cleanup func()) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cleanup()
		os.Exit(0)
	}()
}
