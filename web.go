package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"snishaper/cert"
	"snishaper/proxy"
)

//go:embed web/dist/*
var webAssets embed.FS

type webServer struct {
	runtime     *coreRuntime
	mux         *http.ServeMux
	logSubs     map[chan string]struct{}
	logSubsMu   sync.Mutex
	webPort     int
}

func cmdWeb(args []string) {
	port := 5173
	bind := "127.0.0.1"
	configDir := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port", "-p":
			if i+1 < len(args) {
				if p, err := strconv.Atoi(args[i+1]); err == nil {
					port = p
				}
				i++
			}
		case "--bind", "-b":
			if i+1 < len(args) {
				bind = args[i+1]
				i++
			}
		case "--config-dir":
			if i+1 < len(args) {
				configDir = args[i+1]
				i++
			}
		}
	}

	if configDir == "" {
		configDir = getConfigDir()
	}

	rt, err := newCoreRuntimeForDir(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize core runtime: %v\n", err)
		os.Exit(1)
	}
	defer rt.shutdown()

	ws := &webServer{
		runtime: rt,
		mux:     http.NewServeMux(),
		logSubs: make(map[chan string]struct{}),
		webPort: port,
	}

	ws.setupRoutes()

	addr := fmt.Sprintf("%s:%d", bind, port)
	fmt.Printf("SniShaper Web Dashboard: http://%s\n", addr)

	if err := http.ListenAndServe(addr, ws.mux); err != nil {
		fmt.Fprintf(os.Stderr, "Web server error: %v\n", err)
		os.Exit(1)
	}
}

func startWebServerAsync(configDir string) *webServer {
	rt, err := newCoreRuntimeForDir(configDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize core runtime: %v\n", err)
		return nil
	}

	return startWebServerWithRuntime(rt)
}

func startWebServerWithRuntime(rt *coreRuntime) *webServer {
	ws := &webServer{
		runtime: rt,
		mux:     http.NewServeMux(),
		logSubs: make(map[chan string]struct{}),
		webPort: 5173,
	}

	ws.setupRoutes()

	addr := "127.0.0.1:5173"
	go func() {
		fmt.Printf("Web dashboard started at http://%s\n", addr)
		if err := http.ListenAndServe(addr, ws.mux); err != nil {
			fmt.Fprintf(os.Stderr, "Web server error: %v\n", err)
		}
	}()

	return ws
}

func (ws *webServer) setupRoutes() {
	ws.mux.HandleFunc("/api/status", ws.handleStatus)
	ws.mux.HandleFunc("/api/proxy/start", ws.handleProxyStart)
	ws.mux.HandleFunc("/api/proxy/stop", ws.handleProxyStop)
	ws.mux.HandleFunc("/api/proxy/stats", ws.handleProxyStats)
	ws.mux.HandleFunc("/api/config", ws.handleConfig)
	ws.mux.HandleFunc("/api/cert/status", ws.handleCertStatus)
	ws.mux.HandleFunc("/api/cert/install", ws.handleCertInstall)
	ws.mux.HandleFunc("/api/cert/uninstall", ws.handleCertUninstall)
	ws.mux.HandleFunc("/api/cert/regenerate", ws.handleCertRegenerate)
	ws.mux.HandleFunc("/api/cert/export", ws.handleCertExport)
	ws.mux.HandleFunc("/api/rules", ws.handleRules)
	ws.mux.HandleFunc("/api/tun/status", ws.handleTUNStatus)
	ws.mux.HandleFunc("/api/tun/start", ws.handleTUNStart)
	ws.mux.HandleFunc("/api/tun/stop", ws.handleTUNStop)
	ws.mux.HandleFunc("/api/upstreams", ws.handleUpstreams)
	ws.mux.HandleFunc("/api/proxy/diagnostics", ws.handleProxyDiagnostics)
	ws.mux.HandleFunc("/api/proxy/self-check", ws.handleProxySelfCheck)
	ws.mux.HandleFunc("/api/dns/priority", ws.handleDNSPriority)
	ws.mux.HandleFunc("/api/log/capture", ws.handleLogCapture)
	ws.mux.HandleFunc("/api/rules/hits", ws.handleRuleHits)
	ws.mux.HandleFunc("/api/logs", ws.handleLogs)
	ws.mux.HandleFunc("/api/logs/clear", ws.handleLogsClear)
	ws.mux.HandleFunc("/api/route-events", ws.handleRouteEvents)
	ws.mux.HandleFunc("/api/routing/config", ws.handleRoutingConfig)
	ws.mux.HandleFunc("/api/routing/gfwlist/refresh", ws.handleGFWListRefresh)
	ws.mux.HandleFunc("/api/routing/status", ws.handleRoutingStatus)
	ws.mux.HandleFunc("/api/server", ws.handleServerConfig)
	ws.mux.HandleFunc("/api/dns/nodes", ws.handleDNSNodes)
	ws.mux.HandleFunc("/api/dns/test", ws.handleDNSTest)
	ws.mux.HandleFunc("/api/ech/profiles", ws.handleECHProfiles)
	ws.mux.HandleFunc("/api/ech/fetch", ws.handleECHFetch)
	ws.mux.HandleFunc("/api/cf/config", ws.handleCFConfig)
	ws.mux.HandleFunc("/api/cf/fetch", ws.handleCFFetch)
	ws.mux.HandleFunc("/api/cf/health-check", ws.handleCFHealthCheck)
	ws.mux.HandleFunc("/api/cf/stats", ws.handleCFStats)
	ws.mux.HandleFunc("/api/config/export", ws.handleConfigExport)
	ws.mux.HandleFunc("/api/config/import", ws.handleConfigImport)
	ws.mux.HandleFunc("/ws/logs", ws.handleWSLogs)

	distSub, err := fs.Sub(webAssets, "web/dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(distSub))
		ws.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws/logs" {
				http.NotFound(w, r)
				return
			}
			path := r.URL.Path
			if path == "/" {
				path = "index.html"
			}
			f, err := distSub.Open(strings.TrimPrefix(path, "/"))
			if err != nil {
				r.URL.Path = "/"
				fileServer.ServeHTTP(w, r)
				return
			}
			f.Close()
			fileServer.ServeHTTP(w, r)
		})
	} else {
		ws.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!DOCTYPE html><html><body>
			<h1>SniShaper-Linux</h1>
			<p>Web frontend not built. Run: cd web && npm install && npm run build</p>
			<p>API available at <a href="/api/status">/api/status</a></p>
			</body></html>`)
		})
	}
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (ws *webServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	rt := ws.runtime
	proxyRunning := rt.proxyServer.IsRunning()
	tunStatus := rt.getTUNStatus()

	certPath := filepath.Join(rt.execDir, "cert")
	cm, err := cert.InitCertManager(certPath)
	var certInstalled bool
	if err == nil {
		certInstalled = cm.IsCAInstalled()
	}

	jsonResponse(w, map[string]interface{}{
		"version":       version,
		"proxy_running": proxyRunning,
		"listen_addr":   rt.proxyServer.GetListenAddr(),
		"proxy_mode":    rt.proxyServer.GetMode(),
		"tun":           tunStatus,
		"cert_installed": certInstalled,
		"socks5_enabled": rt.proxyServer.IsSocks5Enabled(),
	})
}

func (ws *webServer) handleProxyStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	if err := ws.runtime.startProxy(); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "started"})
}

func (ws *webServer) handleProxyStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	if err := ws.runtime.stopProxy(); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "stopped"})
}

func (ws *webServer) handleProxyStats(w http.ResponseWriter, r *http.Request) {
	down, up, etc := ws.runtime.proxyServer.GetStats()
	_ = etc
	jsonResponse(w, map[string]int64{
		"bytes_down": down,
		"bytes_up":   up,
	})
}

func (ws *webServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		jsonResponse(w, map[string]interface{}{
			"listen_port":    ws.runtime.currentListenPort(),
			"socks5_port":    ws.runtime.ruleManager.GetSocks5Port(),
			"socks5_enabled": ws.runtime.ruleManager.GetSocks5Enabled(),
			"proxy_mode":     ws.runtime.proxyServer.GetMode(),
			"tun":            ws.runtime.ruleManager.GetTUNConfig(),
			"auto_routing":   ws.runtime.ruleManager.GetAutoRoutingConfig(),
			"cloudflare":     ws.runtime.ruleManager.GetCloudflareConfig(),
			"server_host":    ws.runtime.ruleManager.GetServerHost(),
			"server_auth":    ws.runtime.ruleManager.GetServerAuth(),
			"language":       ws.runtime.ruleManager.GetLanguage(),
			"theme":          ws.runtime.ruleManager.GetTheme(),
		})
		return
	}
	if r.Method == http.MethodPost {
		var cfg struct {
			ListenPort    *string `json:"listen_port"`
			Socks5Port    *string `json:"socks5_port"`
			Socks5Enabled *bool   `json:"socks5_enabled"`
			ProxyMode     *string `json:"proxy_mode"`
			Language      *string `json:"language"`
			Theme         *string `json:"theme"`
		}
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if cfg.ListenPort != nil {
			ws.runtime.ruleManager.SetListenPort(*cfg.ListenPort)
		}
		if cfg.Socks5Port != nil {
			ws.runtime.ruleManager.SetSocks5Port(*cfg.Socks5Port)
		}
		if cfg.Socks5Enabled != nil {
			ws.runtime.ruleManager.SetSocks5Enabled(*cfg.Socks5Enabled)
			ws.runtime.proxyServer.SetSocks5Enabled(*cfg.Socks5Enabled)
		}
		if cfg.ProxyMode != nil {
			ws.runtime.proxyServer.SetMode(*cfg.ProxyMode)
		}
		if cfg.Language != nil {
			ws.runtime.ruleManager.SetLanguage(*cfg.Language)
		}
		if cfg.Theme != nil {
			ws.runtime.ruleManager.SetTheme(*cfg.Theme)
		}
		_ = ws.runtime.ruleManager.SaveConfig()
		jsonResponse(w, map[string]string{"status": "saved"})
		return
	}
	jsonError(w, "Method not allowed", 405)
}

func (ws *webServer) handleCertStatus(w http.ResponseWriter, r *http.Request) {
	certPath := filepath.Join(ws.runtime.execDir, "cert")
	cm, err := cert.InitCertManager(certPath)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	status := cm.GetCAInstallStatus()
	jsonResponse(w, status)
}

func (ws *webServer) handleCertInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Password string `json:"password,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	certPath := filepath.Join(ws.runtime.execDir, "cert")
	cm, err := cert.InitCertManager(certPath)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	if err := cm.InstallCA(req.Password); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "installed"})
}

func (ws *webServer) handleCertUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Password string `json:"password,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	certPath := filepath.Join(ws.runtime.execDir, "cert")
	cm, err := cert.InitCertManager(certPath)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	certs, err := cm.GetInstalledCertificates()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	for _, c := range certs {
		_ = cm.UninstallCertificate(c.Token, req.Password)
	}
	jsonResponse(w, map[string]string{"status": "uninstalled"})
}

func (ws *webServer) handleCertRegenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Password string `json:"password,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	certPath := filepath.Join(ws.runtime.execDir, "cert")
	cm, err := cert.InitCertManager(certPath)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	if err := cm.RegenerateCA(req.Password); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "regenerated"})
}

func (ws *webServer) handleCertExport(w http.ResponseWriter, r *http.Request) {
	certPath := filepath.Join(ws.runtime.execDir, "cert")
	cm, err := cert.InitCertManager(certPath)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	pemData := cm.GetCACertPEM()
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", "attachment; filename=snishaper-ca.crt")
	fmt.Fprint(w, pemData)
}

func (ws *webServer) handleRules(w http.ResponseWriter, r *http.Request) {
	rm := ws.runtime.ruleManager
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, rm.GetSiteGroups())
	case http.MethodPost:
		var sg proxy.SiteGroup
		if err := json.NewDecoder(r.Body).Decode(&sg); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if err := rm.AddSiteGroup(sg); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "added"})
	case http.MethodPut:
		var sg proxy.SiteGroup
		if err := json.NewDecoder(r.Body).Decode(&sg); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if err := rm.UpdateSiteGroup(sg); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "updated"})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			jsonError(w, "missing id", 400)
			return
		}
		if err := rm.DeleteSiteGroup(id); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "deleted"})
	default:
		jsonError(w, "Method not allowed", 405)
	}
}

func (ws *webServer) handleTUNStatus(w http.ResponseWriter, r *http.Request) {
	status := ws.runtime.getTUNStatus()
	jsonResponse(w, status)
}

func (ws *webServer) handleTUNStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	if err := ws.runtime.startTUN(); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "started"})
}

func (ws *webServer) handleTUNStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	if err := ws.runtime.stopTUN(); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "stopped"})
}

func (ws *webServer) handleUpstreams(w http.ResponseWriter, r *http.Request) {
	rm := ws.runtime.ruleManager
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, rm.GetUpstreams())
	case http.MethodPost:
		var u proxy.Upstream
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if err := rm.AddUpstream(u); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "added"})
	case http.MethodPut:
		var u proxy.Upstream
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if err := rm.UpdateUpstream(u); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "updated"})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			jsonError(w, "missing id", 400)
			return
		}
		if err := rm.DeleteUpstream(id); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "deleted"})
	default:
		jsonError(w, "Method not allowed", 405)
	}
}

func (ws *webServer) handleProxyDiagnostics(w http.ResponseWriter, r *http.Request) {
	accepted, requests, connects, recent := ws.runtime.proxyServer.GetDiagnostics()
	jsonResponse(w, map[string]interface{}{
		"accepted":       accepted,
		"requests":       requests,
		"connects":       connects,
		"recent_ingress": recent,
		"listen_addr":    ws.runtime.proxyServer.GetListenAddr(),
		"proxy_running":  ws.runtime.proxyServer.IsRunning(),
	})
}

func (ws *webServer) handleProxySelfCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	running := ws.runtime.proxyServer.IsRunning()
	if !running {
		jsonError(w, "proxy not running", 400)
		return
	}
	// Basic connectivity check: verify proxy is listening
	addr := ws.runtime.proxyServer.GetListenAddr()
	jsonResponse(w, map[string]interface{}{
		"status":  "ok",
		"message": "proxy is running on " + addr,
	})
}

func (ws *webServer) handleDNSPriority(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	var req struct {
		ID          string `json:"id"`
		TargetIndex int    `json:"target_index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), 400)
		return
	}
	if err := ws.runtime.ruleManager.SetDNSNodePriority(req.ID, req.TargetIndex); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]string{"status": "updated"})
}

func (ws *webServer) handleLogCapture(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, map[string]bool{"capture_enabled": ws.runtime.isLogCaptureEnabled()})
	case http.MethodPost:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if req.Enabled {
			ws.runtime.startLogCapture()
		} else {
			ws.runtime.stopLogCapture()
		}
		jsonResponse(w, map[string]string{"status": "updated"})
	default:
		jsonError(w, "Method not allowed", 405)
	}
}

func (ws *webServer) handleRuleHits(w http.ResponseWriter, r *http.Request) {
	hits := ws.runtime.ruleManager.GetRuleHitCounts()
	jsonResponse(w, hits)
}

func (ws *webServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	logs := ws.runtime.recentLogs(limit)
	jsonResponse(w, map[string]string{"logs": logs})
}

func (ws *webServer) handleLogsClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	ws.runtime.clearLogs()
	jsonResponse(w, map[string]string{"status": "cleared"})
}

func (ws *webServer) handleRouteEvents(w http.ResponseWriter, r *http.Request) {
	events := ws.runtime.popRouteEvents()
	jsonResponse(w, events)
}

func (ws *webServer) handleRoutingConfig(w http.ResponseWriter, r *http.Request) {
	rm := ws.runtime.ruleManager
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, rm.GetAutoRoutingConfig())
	case http.MethodPost:
		var cfg proxy.AutoRoutingConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if err := rm.UpdateAutoRoutingConfig(cfg); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "saved"})
	default:
		jsonError(w, "Method not allowed", 405)
	}
}

func (ws *webServer) handleGFWListRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	count, err := ws.runtime.ruleManager.RefreshGFWList()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]interface{}{"count": count, "status": "refreshed"})
}

func (ws *webServer) handleRoutingStatus(w http.ResponseWriter, r *http.Request) {
	status := ws.runtime.ruleManager.GetAutoRoutingStatus()
	jsonResponse(w, status)
}

func (ws *webServer) handleServerConfig(w http.ResponseWriter, r *http.Request) {
	rm := ws.runtime.ruleManager
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, map[string]string{
			"host": rm.GetServerHost(),
			"auth": rm.GetServerAuth(),
		})
	case http.MethodPost:
		var cfg struct {
			Host string `json:"host"`
			Auth string `json:"auth"`
		}
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if err := rm.UpdateServerConfig(cfg.Host, cfg.Auth); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "saved"})
	default:
		jsonError(w, "Method not allowed", 405)
	}
}

func (ws *webServer) handleDNSNodes(w http.ResponseWriter, r *http.Request) {
	rm := ws.runtime.ruleManager
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, rm.GetDNSNodes())
	case http.MethodPost:
		var n proxy.DNSNode
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if err := rm.AddDNSNode(n); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "added"})
	case http.MethodPut:
		var n proxy.DNSNode
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if err := rm.UpdateDNSNode(n); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "updated"})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			jsonError(w, "missing id", 400)
			return
		}
		if err := rm.DeleteDNSNode(id); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "deleted"})
	default:
		jsonError(w, "Method not allowed", 405)
	}
}

func (ws *webServer) handleDNSTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), 400)
		return
	}
	node := getDNSNodeByID(ws.runtime.ruleManager, req.ID)
	if node == nil {
		jsonError(w, "DNS node not found", 404)
		return
	}
	ips, err := ws.runtime.proxyServer.GetDoHResolver().TestNode(r.Context(), *node)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]interface{}{"ips": ips, "status": "ok"})
}

func (ws *webServer) handleECHProfiles(w http.ResponseWriter, r *http.Request) {
	rm := ws.runtime.ruleManager
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, rm.GetECHProfiles())
	case http.MethodPost:
		var p proxy.ECHProfile
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if err := rm.UpsertECHProfile(p); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "saved"})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			jsonError(w, "missing id", 400)
			return
		}
		if err := rm.DeleteECHProfile(id); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "deleted"})
	default:
		jsonError(w, "Method not allowed", 405)
	}
}

func (ws *webServer) handleECHFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Domain string `json:"domain"`
		DoHURL string `json:"doh_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), 400)
		return
	}
	data, err := ws.runtime.proxyServer.FetchECH(r.Context(), req.Domain, req.DoHURL)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]interface{}{"config": data, "status": "ok"})
}

func (ws *webServer) handleCFConfig(w http.ResponseWriter, r *http.Request) {
	rm := ws.runtime.ruleManager
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, rm.GetCloudflareConfig())
	case http.MethodPost:
		var cfg proxy.CloudflareConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			jsonError(w, "invalid JSON: "+err.Error(), 400)
			return
		}
		if err := rm.UpdateCloudflareConfig(cfg); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, map[string]string{"status": "saved"})
	default:
		jsonError(w, "Method not allowed", 405)
	}
}

func (ws *webServer) handleCFFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	ips, err := proxy.FetchCloudflareIPs("")
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	ws.runtime.ruleManager.UpdateCloudflareConfig(proxy.CloudflareConfig{PreferredIPs: ips, AutoUpdate: true})
	ws.runtime.proxyServer.UpdateCloudflareIPPool(ips)
	jsonResponse(w, map[string]interface{}{"ips": ips, "count": len(ips)})
}

func (ws *webServer) handleCFHealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	ws.runtime.proxyServer.TriggerCFHealthCheck()
	jsonResponse(w, map[string]string{"status": "triggered"})
}

func (ws *webServer) handleCFStats(w http.ResponseWriter, r *http.Request) {
	stats := ws.runtime.proxyServer.GetAllCFIPsWithStats()
	jsonResponse(w, stats)
}

func (ws *webServer) handleConfigExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	data, err := ws.runtime.ruleManager.ExportConfig()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]string{"config": data})
}

func (ws *webServer) handleConfigImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Config string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON: "+err.Error(), 400)
		return
	}
	summary, err := ws.runtime.ruleManager.ImportConfigWithSummary(req.Config)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, summary)
}

func getDNSNodeByID(rm *proxy.RuleManager, id string) *proxy.DNSNode {
	for _, n := range rm.GetDNSNodes() {
		if n.ID == id {
			return &n
		}
	}
	return nil
}

func (ws *webServer) handleWSLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	logChan := make(chan string, 100)
	ws.logSubsMu.Lock()
	ws.logSubs[logChan] = struct{}{}
	ws.logSubsMu.Unlock()

	defer func() {
		ws.logSubsMu.Lock()
		delete(ws.logSubs, logChan)
		ws.logSubsMu.Unlock()
	}()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-logChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}
