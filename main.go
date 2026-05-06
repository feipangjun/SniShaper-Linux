package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"snishaper/cert"
	"snishaper/proxy"
)

var (
	Version      = "1.0.0"
	listenAddr   string
	configDir    string
	rulesFile    string
	settingsFile string
	certDir      string
	mode         string
	apiAddr      string
	showVersion  bool
	showHelp     bool
)

func init() {
	flag.StringVar(&listenAddr, "i", "", "listen address (short: -i)")
	flag.StringVar(&listenAddr, "input", "", "listen address")
	flag.StringVar(&listenAddr, "l", "0.0.0.0:8080", "listen address")
	flag.StringVar(&listenAddr, "listen", "0.0.0.0:8080", "listen address")
	flag.StringVar(&configDir, "c", "", "config directory (short: -c)")
	flag.StringVar(&configDir, "config", "", "config directory")
	flag.StringVar(&rulesFile, "r", "", "rules config file (short: -r)")
	flag.StringVar(&rulesFile, "rules", "", "rules config file")
	flag.StringVar(&settingsFile, "s", "", "settings config file (short: -s)")
	flag.StringVar(&settingsFile, "settings", "", "settings config file")
	flag.StringVar(&certDir, "d", "", "certificate directory (short: -d)")
	flag.StringVar(&certDir, "cert-dir", "", "certificate directory")
	flag.StringVar(&mode, "m", "", "proxy mode: mitm, transparent, tls-rf, quic (short: -m)")
	flag.StringVar(&mode, "mode", "", "proxy mode: mitm, transparent, tls-rf, quic")
	flag.StringVar(&apiAddr, "api", "", "API server address (short: -api)")
	flag.BoolVar(&showVersion, "v", false, "show version (short: -v)")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.BoolVar(&showHelp, "h", false, "show help (short: -h)")
	flag.BoolVar(&showHelp, "help", false, "show help")

	flag.Usage = func() {
		fmt.Print(`SniShaper CLI - Cloudflare IP Shaper for Linux

Usage:
  snishaper [OPTIONS]

Options:
`)
		flag.PrintDefaults()
		fmt.Print(`
Examples:
  snishaper                           Start with default settings
  snishaper -l 0.0.0.0:8080          Listen on all interfaces
  snishaper -c ~/.snishaper           Use custom config directory
  snishaper -m mitm                   Start in MITM mode
  snishaper --version                 Show version

Report issues to: https://github.com/nic-ences/SniShaper
`)
	}
}

func normalizePath(p string) string {
	if p == "" {
		return p
	}
	p = os.ExpandEnv(p)
	p = strings.ReplaceAll(p, "\\", "/")

	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if strings.HasPrefix(p, "/mnt/") || strings.HasPrefix(p, "/host/") {
			if converted, err := windowsToLinuxPath(p); err == nil {
				return converted
			}
		}
	}

	return filepath.Clean(p)
}

func windowsToLinuxPath(p string) (string, error) {
	p = strings.ReplaceAll(p, "\\", "/")
	if strings.HasPrefix(p, "/mnt/") {
		parts := strings.SplitN(p[5:], "/", 2)
		if len(parts) >= 1 {
			letter := strings.ToLower(parts[0])
			if len(letter) == 1 && letter[0] >= 'a' && letter[0] <= 'z' {
				if len(parts) == 2 {
					return "/" + letter + "/" + parts[1], nil
				}
				return "/" + letter, nil
			}
		}
	}
	if strings.HasPrefix(p, "/host/") {
		return p[5:], nil
	}
	return p, fmt.Errorf("not a Windows path")
}

func getDefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/etc"
	}
	return filepath.Join(home, ".snishaper")
}

func ensureConfigPaths() (string, string, string) {
	cfgDir := normalizePath(configDir)
	if cfgDir == "" {
		cfgDir = getDefaultConfigDir()
	}

	rulesPath := normalizePath(rulesFile)
	if rulesPath == "" {
		rulesPath = filepath.Join(cfgDir, "rules", "config.json")
	}

	settingsPath := normalizePath(settingsFile)
	if settingsPath == "" {
		settingsPath = filepath.Join(cfgDir, "config", "settings.json")
	}

	certPath := normalizePath(certDir)
	if certPath == "" {
		certPath = filepath.Join(cfgDir, "cert")
	}

	if err := os.MkdirAll(filepath.Dir(rulesPath), 0755); err != nil {
		log.Printf("[WARN] failed to create rules dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		log.Printf("[WARN] failed to create settings dir: %v", err)
	}
	if err := os.MkdirAll(certPath, 0755); err != nil {
		log.Printf("[WARN] failed to create cert dir: %v", err)
	}

	// Copy default rules if not exists
	copyDefaultRules(rulesPath)

	return rulesPath, settingsPath, certPath
}

func copyDefaultRules(rulesPath string) {
	// Check if rules file already exists
	if _, err := os.Stat(rulesPath); err == nil {
		return // Rules already exist
	}

	// Get executable directory
	execDir := getExecutableDir()
	defaultRulesPath := filepath.Join(execDir, "rules", "config.json")

	// Check if default rules exist in executable directory
	if _, err := os.Stat(defaultRulesPath); os.IsNotExist(err) {
		// Try dist/rules for development
		defaultRulesPath = filepath.Join(execDir, "dist", "rules", "config.json")
		if _, err := os.Stat(defaultRulesPath); os.IsNotExist(err) {
			log.Printf("[WARN] Default rules not found at %s", defaultRulesPath)
			return
		}
	}

	// Copy default rules
	data, err := os.ReadFile(defaultRulesPath)
	if err != nil {
		log.Printf("[WARN] Failed to read default rules: %v", err)
		return
	}

	if err := os.WriteFile(rulesPath, data, 0644); err != nil {
		log.Printf("[WARN] Failed to copy default rules: %v", err)
		return
	}

	log.Printf("[INFO] Copied default rules to %s", rulesPath)
}

func getExecutableDir() string {
	execPath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(execPath)
}

type CLIApp struct {
	proxyServer  *proxy.ProxyServer
	certManager  *cert.CertManager
	ruleManager  *proxy.RuleManager
	listenAddr   string
	logBuffer    *ringLogWriter
	logCaptureMu sync.RWMutex
}

type ringLogWriter struct {
	mu      sync.Mutex
	lines   []string
	pending string
	max     int
}

func newRingLogWriter(max int) *ringLogWriter {
	if max <= 0 {
		max = 1000
	}
	return &ringLogWriter{
		lines: make([]string, 0, max),
		max:   max,
	}
}

func (w *ringLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	text := w.pending + strings.ReplaceAll(string(p), "\r\n", "\n")
	parts := strings.Split(text, "\n")
	if len(parts) == 0 {
		return len(p), nil
	}
	w.pending = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		if line == "" {
			continue
		}
		w.lines = append(w.lines, line)
		if len(w.lines) > w.max {
			if cap(w.lines) > w.max*2 {
				newLines := make([]string, w.max)
				copy(newLines, w.lines[len(w.lines)-w.max:])
				w.lines = newLines
			} else {
				w.lines = w.lines[len(w.lines)-w.max:]
			}
		}
	}
	return len(p), nil
}

func (w *ringLogWriter) Snapshot(limit int) []string {
	if limit <= 0 {
		limit = 200
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	total := len(w.lines)
	if total == 0 {
		if w.pending != "" {
			return []string{w.pending}
		}
		return []string{}
	}
	if limit > total {
		limit = total
	}
	start := total - limit
	out := make([]string, limit)
	copy(out, w.lines[start:])
	return out
}

func (w *ringLogWriter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lines = w.lines[:0]
	w.pending = ""
}

func (w *ringLogWriter) AppendLine(line string) {
	if line == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lines = append(w.lines, line)
	if len(w.lines) > w.max {
		w.lines = w.lines[len(w.lines)-w.max:]
	}
}

func NewCLIApp(rulesPath, settingsPath, certPath, listen string) (*CLIApp, error) {
	app := &CLIApp{
		listenAddr: listen,
		logBuffer:  newRingLogWriter(5000),
	}

	app.ruleManager = proxy.NewRuleManager(settingsPath, rulesPath)
	if err := app.ruleManager.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	port := app.ruleManager.GetListenPort()
	if port != "" && listen == "127.0.0.1:8080" {
		listen = "127.0.0.1:" + port
		app.listenAddr = listen
	}

	var err error
	app.certManager, err = cert.InitCertManager(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to init cert manager: %w", err)
	}

	app.proxyServer = proxy.NewProxyServer(listen)
	app.proxyServer.SetRuleManager(app.ruleManager)
	app.proxyServer.UpdateCloudflareConfig(app.ruleManager.GetCloudflareConfig())
	app.proxyServer.SetCertGenerator(app.certManager)
	app.proxyServer.SetLogCallback(app.appendLog)

	cf := app.ruleManager.GetCloudflareConfig()
	if len(cf.PreferredIPs) > 0 {
		app.proxyServer.UpdateCloudflareIPPool(cf.PreferredIPs)
		go func() {
			time.Sleep(1 * time.Second)
			app.proxyServer.TriggerCFHealthCheck()
		}()
	}

	return app, nil
}

func (a *CLIApp) appendLog(message string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}
	fmt.Println(trimmed)
	a.logBuffer.AppendLine(trimmed)
}

func (a *CLIApp) Start() error {
	return a.proxyServer.Start()
}

func (a *CLIApp) Stop() error {
	return a.proxyServer.Stop()
}

func (a *CLIApp) IsRunning() bool {
	return a.proxyServer.IsRunning()
}

func (a *CLIApp) GetStats() (int64, int64) {
	down, up, _ := a.proxyServer.GetStats()
	return down, up
}

func (a *CLIApp) ReloadConfig() error {
	if err := a.ruleManager.LoadConfig(); err != nil {
		return err
	}
	a.proxyServer.SetRuleManager(a.ruleManager)
	a.proxyServer.UpdateCloudflareConfig(a.ruleManager.GetCloudflareConfig())
	return nil
}

func (a *CLIApp) ReloadCertificate() error {
	cm, err := cert.InitCertManager(a.certManager.GetCAInstallStatus().CertPath)
	if err != nil {
		return err
	}
	a.certManager = cm
	a.proxyServer.SetCertGenerator(a.certManager)
	a.proxyServer.ClearCertCache()
	return nil
}

func (a *CLIApp) GetMode() string {
	return a.proxyServer.GetMode()
}

func (a *CLIApp) SetMode(newMode string) error {
	return a.proxyServer.SetMode(newMode)
}

func (a *CLIApp) GetLogs(limit int) []string {
	return a.logBuffer.Snapshot(limit)
}

func (a *CLIApp) ClearLogs() {
	a.logBuffer.Clear()
}

func printBanner() {
	fmt.Println(`
╔═══════════════════════════════════════════════════╗
║           SniShaper CLI v` + Version + `                    ║
║        Cloudflare IP Shaper - Linux Edition       ║
╚═══════════════════════════════════════════════════╝`)
}

type statsDisplay struct {
	ticker   *time.Ticker
	done     chan struct{}
	lastIn   int64
	lastOut  int64
	lastTick time.Time
}

func newStatsDisplay() *statsDisplay {
	return &statsDisplay{
		ticker:   time.NewTicker(1 * time.Second),
		done:     make(chan struct{}),
		lastIn:   0,
		lastOut:  0,
		lastTick: time.Now(),
	}
}

func (sd *statsDisplay) start(app *CLIApp) {
	go func() {
		for {
			select {
			case <-sd.ticker.C:
				if app.IsRunning() {
					currentIn, currentOut := app.GetStats()
					now := time.Now()
					duration := now.Sub(sd.lastTick).Seconds()

					var downSpeed, upSpeed float64
					if duration > 0 {
						downSpeed = float64(currentIn-sd.lastIn) / duration
						upSpeed = float64(currentOut-sd.lastOut) / duration
					}

					if downSpeed < 0 {
						downSpeed = 0
					}
					if upSpeed < 0 {
						upSpeed = 0
					}

					fmt.Printf("\r[Stats] ↓ %s/s  ↑ %s/s", formatBytes(int64(downSpeed)), formatBytes(int64(upSpeed)))

					sd.lastIn = currentIn
					sd.lastOut = currentOut
					sd.lastTick = now
				}
			case <-sd.done:
				return
			}
		}
	}()
}

func (sd *statsDisplay) stop() {
	sd.ticker.Stop()
	close(sd.done)
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n >= unit {
		n /= div
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n), "KMGTPE"[exp])
}

type APIServer struct {
	app    *CLIApp
	server *http.Server
}

func NewAPIServer(addr string, app *CLIApp) *APIServer {
	mux := http.NewServeMux()

	s := &APIServer{app: app}
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/logs", s.handleLogs)
	mux.HandleFunc("/mode", s.handleMode)
	mux.HandleFunc("/reload", s.handleReload)
	mux.HandleFunc("/stop", s.handleStop)

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

func (s *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"running":    s.app.IsRunning(),
		"mode":       s.app.GetMode(),
		"listenAddr": s.app.listenAddr,
		"version":    Version,
	})
}

func (s *APIServer) handleStats(w http.ResponseWriter, r *http.Request) {
	down, up := s.app.GetStats()
	json.NewEncoder(w).Encode(map[string]int64{"download": down, "upload": up})
}

func (s *APIServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	fmt.Fscanf(r.Body, "%d", &limit)
	logs := s.app.GetLogs(limit)
	json.NewEncoder(w).Encode(map[string][]string{"logs": logs})
}

func (s *APIServer) handleMode(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		fmt.Fprintf(w, s.app.GetMode())
	case http.MethodPost:
		m := r.URL.Query().Get("m")
		if m != "" {
			if err := s.app.SetMode(m); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			fmt.Fprintf(w, "Mode set to %s", m)
		}
	}
}

func (s *APIServer) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if err := s.app.ReloadConfig(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	fmt.Fprintf(w, "Config reloaded")
}

func (s *APIServer) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	fmt.Fprintf(w, "Shutting down...")
	go func() {
		s.app.Stop()
		os.Exit(0)
	}()
}

func (s *APIServer) Start() error {
	log.Printf("[Debug] API server starting on %s", s.server.Addr)
	err := s.server.ListenAndServe()
	log.Printf("[Debug] API server stopped: %v", err)
	return err
}

func (s *APIServer) Stop() error {
	return s.server.Shutdown(context.Background())
}

func waitForSignal(app *CLIApp) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	fmt.Printf("\n[Info] Received signal: %v\n", sig)
	fmt.Println("[Info] Shutting down...")

	if err := app.Stop(); err != nil {
		log.Printf("[Error] Stop proxy failed: %v", err)
	}

	fmt.Println("[Info] Goodbye!")
}

func main() {
	flag.Parse()

	if showVersion {
		fmt.Printf("SniShaper CLI v%s\n", Version)
		fmt.Printf("Go %s (%s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}

	if showHelp {
		printBanner()
		flag.Usage()
		return
	}

	printBanner()

	rulesPath, settingsPath, certPath := ensureConfigPaths()

	fmt.Printf("[Info] Config dir: %s\n", filepath.Dir(settingsPath))
	fmt.Printf("[Info] Rules file: %s\n", rulesPath)
	fmt.Printf("[Info] Settings file: %s\n", settingsPath)
	fmt.Printf("[Info] Cert dir: %s\n", certPath)

	actualListen := listenAddr
	if actualListen == "" {
		actualListen = "127.0.0.1:8080"
	}
	fmt.Printf("[Info] Listen address: %s\n", actualListen)

	app, err := NewCLIApp(rulesPath, settingsPath, certPath, actualListen)
	if err != nil {
		log.Fatalf("[Error] Failed to create app: %v", err)
	}

	if mode != "" {
		if err := app.SetMode(mode); err != nil {
			log.Printf("[Warn] Failed to set mode %s: %v", mode, err)
		} else {
			fmt.Printf("[Info] Proxy mode set to: %s\n", mode)
		}
	}

	fmt.Println("[Info] Starting proxy server...")
	if err := app.Start(); err != nil {
		log.Fatalf("[Error] Failed to start proxy: %v", err)
	}

	fmt.Printf("[Info] Proxy running in %s mode\n", app.GetMode())
	fmt.Println("[Info] Press Ctrl+C to stop")
	fmt.Println()
	actualAPIAddr := apiAddr
	if actualAPIAddr == "" {
		actualAPIAddr = "0.0.0.0:5173"
	}
	fmt.Printf("[Info] API server listening on %s\n", actualAPIAddr)
	fmt.Printf("[Info] Endpoints: /status, /stats, /logs, /mode, /reload, /stop\n")
	fmt.Println()

	apiServer := NewAPIServer(actualAPIAddr, app)
	go func() {
		if err := apiServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Error] API server error: %v", err)
		}
	}()

	stats := newStatsDisplay()
	stats.start(app)
	defer stats.stop()

	waitForSignal(app)
}
