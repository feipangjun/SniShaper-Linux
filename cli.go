package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"snishaper/cert"
	"snishaper/proxy"
)

func cmdStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	port := fs.String("port", "", "Listen port (default: from config or 8080)")
	socks5Port := fs.String("socks5-port", "", "SOCKS5 port (default: from config or 8081)")
	configDir := fs.String("config-dir", "", "Config directory (default: executable dir)")
	noWeb := fs.Bool("no-web", false, "Do not start web dashboard")
	foreground := fs.Bool("foreground", true, "Run in foreground (always true for Linux CLI)")
	_ = foreground
	fs.Parse(args)

	execDir := *configDir
	if execDir == "" {
		execDir = getConfigDir()
	}

	rt, err := newCoreRuntimeForDir(execDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	if *port != "" {
		rt.proxyServer.SetListenAddr("127.0.0.1:" + *port)
	}
	if *socks5Port != "" {
		rt.proxyServer.SetSocks5Addr("127.0.0.1:" + *socks5Port)
	}

	if err := rt.startProxy(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start proxy: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("SniShaper proxy started on %s\n", rt.proxyServer.GetListenAddr())
	if rt.proxyServer.IsSocks5Enabled() {
		fmt.Printf("SOCKS5 proxy on %s\n", rt.proxyServer.GetSocks5Addr())
	}

	var ws *webServer
	if !*noWeb {
		ws = startWebServerWithRuntime(rt)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nStopping...")
	rt.shutdown()
	fmt.Println("Stopped.")
	_ = ws
}

func cmdStop() {
	fmt.Println("Stop is only available in daemon mode.")
	fmt.Println("Use Ctrl+C to stop a foreground process.")
}

func cmdStatus() {
	execDir := getConfigDir()
	settingsPath := filepath.Join(execDir, "config", "settings.json")
	rulesPath := filepath.Join(execDir, "rules", "config.json")

	ruleManager := proxy.NewRuleManager(settingsPath, rulesPath)
	_ = ruleManager.LoadConfig()

	port := ruleManager.GetListenPort()
	if port == "" {
		port = "8080"
	}

	socks5Port := ruleManager.GetSocks5Port()
	if socks5Port == "" {
		socks5Port = "8081"
	}

	fmt.Println("SniShaper-Linux Status")
	fmt.Println("======================")
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("HTTP Proxy: 127.0.0.1:%s\n", port)
	fmt.Printf("SOCKS5:     127.0.0.1:%s\n", socks5Port)

	certPath := filepath.Join(execDir, "cert")
	cm, err := cert.InitCertManager(certPath)
	if err == nil {
		status := cm.GetCAInstallStatus()
		if status.Installed {
			fmt.Printf("CA Certificate: Installed (%s)\n", status.Platform)
		} else {
			fmt.Printf("CA Certificate: Not installed (%s)\n", status.Platform)
		}
	}

	siteGroups := ruleManager.GetSiteGroups()
	enabled := 0
	for _, sg := range siteGroups {
		if sg.Enabled {
			enabled++
		}
	}
	fmt.Printf("Rules: %d site groups (%d enabled)\n", len(siteGroups), enabled)
}

func cmdConfig(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: snishaper config <show|set|export|import> [args]")
		return
	}

	sub := strings.ToLower(args[0])
	execDir := getConfigDir()
	settingsPath := filepath.Join(execDir, "config", "settings.json")
	rulesPath := filepath.Join(execDir, "rules", "config.json")

	switch sub {
	case "show":
		ruleManager := proxy.NewRuleManager(settingsPath, rulesPath)
		if err := ruleManager.LoadConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Listen Port: %s\n", ruleManager.GetListenPort())
		fmt.Printf("SOCKS5 Port: %s\n", ruleManager.GetSocks5Port())
		fmt.Printf("SOCKS5 Enabled: %v\n", ruleManager.GetSocks5Enabled())

		tunCfg := ruleManager.GetTUNConfig()
		fmt.Printf("TUN Enabled: %v\n", tunCfg.Enabled)
		fmt.Printf("TUN MTU: %d\n", tunCfg.MTU)

		autoRouting := ruleManager.GetAutoRoutingConfig()
		fmt.Printf("Auto Routing Mode: %s\n", autoRouting.Mode)

		cfCfg := ruleManager.GetCloudflareConfig()
		fmt.Printf("CF Preferred IPs: %d\n", len(cfCfg.PreferredIPs))
		fmt.Printf("CF Auto Update: %v\n", cfCfg.AutoUpdate)

		siteGroups := ruleManager.GetSiteGroups()
		fmt.Printf("Site Groups: %d\n", len(siteGroups))
		for i, sg := range siteGroups {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(siteGroups)-10)
				break
			}
			status := "disabled"
			if sg.Enabled {
				status = "enabled"
			}
			fmt.Printf("  [%s] %s (%s) - %d domains\n", status, sg.Name, sg.Mode, len(sg.Domains))
		}

	case "export":
		ruleManager := proxy.NewRuleManager(settingsPath, rulesPath)
		if err := ruleManager.LoadConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
		_ = ruleManager
		fmt.Println("Export functionality: use the web dashboard for full import/export")

	case "import":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: snishaper config import <file>\n")
			os.Exit(1)
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read file: %v\n", err)
			os.Exit(1)
		}
		ruleManager := proxy.NewRuleManager(settingsPath, rulesPath)
		summary, err := ruleManager.ImportConfigWithSummary(string(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Import failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Import completed:\n")
		fmt.Printf("  Total: %d, Added: %d, Overwritten: %d, Skipped: %d\n",
			summary.Total, summary.Added, summary.Overwritten, summary.Skipped)

	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", sub)
		fmt.Println("Available: show, export, import")
	}
}

func cmdCert(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: snishaper cert <install|uninstall|status|export|regenerate> [--password <sudo_password>]")
		return
	}

	fs := flag.NewFlagSet("cert", flag.ExitOnError)
	password := fs.String("password", "", "sudo password for privileged operations")
	fs.Parse(args)

	sub := fs.Arg(0)
	if sub == "" {
		fmt.Println("Usage: snishaper cert <install|uninstall|status|export|regenerate> [--password <sudo_password>]")
		return
	}

	execDir := getConfigDir()
	certPath := filepath.Join(execDir, "cert")

	cm, err := cert.InitCertManager(certPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init cert manager: %v\n", err)
		os.Exit(1)
	}

	switch strings.ToLower(sub) {
	case "status":
		status := cm.GetCAInstallStatus()
		fmt.Printf("CA Certificate Status\n")
		fmt.Printf("====================\n")
		fmt.Printf("Platform:    %s\n", status.Platform)
		fmt.Printf("Installed:   %v\n", status.Installed)
		fmt.Printf("Cert Path:   %s\n", status.CertPath)
		if status.InstallHelp != "" {
			fmt.Printf("Help:        %s\n", status.InstallHelp)
		}

	case "install":
		if err := cm.InstallCA(*password); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install CA: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("CA certificate installed successfully")

	case "uninstall":
		certs, err := cm.GetInstalledCertificates()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list installed certs: %v\n", err)
			os.Exit(1)
		}
		for _, c := range certs {
			fmt.Printf("Removing cert: %s (%s)\n", c.Subject, c.Thumbprint)
			if err := cm.UninstallCertificate(c.Token, *password); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to remove: %v\n", err)
			}
		}
		if len(certs) == 0 {
			fmt.Println("No SniShaper certificates found in system store")
		} else {
			fmt.Println("All SniShaper certificates removed")
		}

	case "export":
		pemData := cm.GetCACertPEM()
		if pemData == "" {
			fmt.Fprintf(os.Stderr, "No CA certificate available\n")
			os.Exit(1)
		}
		exportPath := fs.Arg(1)
		if exportPath != "" {
			if err := os.WriteFile(exportPath, []byte(pemData), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("CA certificate exported to %s\n", exportPath)
		} else {
			fmt.Print(pemData)
		}

	case "regenerate":
		if err := cm.RegenerateCA(*password); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to regenerate CA: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("CA certificate regenerated and installed successfully")

	default:
		fmt.Fprintf(os.Stderr, "Unknown cert subcommand: %s\n", sub)
		fmt.Println("Available: status, install, uninstall, export, regenerate")
	}
}

func cmdTUN(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: snishaper tun <start|stop|status>")
		return
	}

	sub := strings.ToLower(args[0])
	execDir := getConfigDir()
	settingsPath := filepath.Join(execDir, "config", "settings.json")
	rulesPath := filepath.Join(execDir, "rules", "config.json")

	ruleManager := proxy.NewRuleManager(settingsPath, rulesPath)
	if err := ruleManager.LoadConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	tunCfg := ruleManager.GetTUNConfig()

	switch sub {
	case "status":
		tunManager := newExternalMihomoManager()
		status := tunManager.Status(tunCfg)
		fmt.Printf("TUN Mode Status\n")
		fmt.Printf("===============\n")
		fmt.Printf("Supported: %v\n", status.Supported)
		fmt.Printf("Running:   %v\n", status.Running)
		fmt.Printf("Driver:    %s\n", status.Driver)
		fmt.Printf("Message:   %s\n", status.Message)

	case "start":
		if os.Getuid() != 0 {
			fmt.Fprintf(os.Stderr, "TUN mode requires root privileges. Use: sudo snishaper tun start\n")
			os.Exit(1)
		}

		port := ruleManager.GetListenPort()
		if port == "" {
			port = "8080"
		}

		tunManager := newExternalMihomoManager()
		if err := tunManager.Start(tunCfg, port, func(msg string) {
			fmt.Println(msg)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start TUN: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("TUN mode started")

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nStopping TUN...")
		_ = tunManager.Stop(nil)
		fmt.Println("TUN stopped.")

	case "stop":
		tunManager := newExternalMihomoManager()
		if err := tunManager.Stop(func(msg string) {
			fmt.Println(msg)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop TUN: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("TUN mode stopped")

	default:
		fmt.Fprintf(os.Stderr, "Unknown tun subcommand: %s\n", sub)
		fmt.Println("Available: start, stop, status")
	}
}
