//go:build linux

package main

import (
	"encoding/json"
	"net/http"
	"os"
	"snishaper/proxy"
	"snishaper/sysproxy"
	"strconv"
	"sync"
)

type LinuxFeaturesManager struct {
	mu          sync.RWMutex
	sysProxy    *proxy.SystemProxyManager
	tunManager  *proxy.TUNManager
	ruleManager *proxy.RuleManager
	proxyServer *proxy.ProxyServer
	app         *CLIApp
}

func NewLinuxFeaturesManager(app *CLIApp, ruleManager *proxy.RuleManager, proxyServer *proxy.ProxyServer, apiPort int) *LinuxFeaturesManager {
	return &LinuxFeaturesManager{
		sysProxy:    proxy.NewSystemProxyManager(apiPort),
		tunManager:  proxy.NewTUNManager(),
		ruleManager: ruleManager,
		proxyServer: proxyServer,
		app:         app,
	}
}

func (m *LinuxFeaturesManager) handleSystemProxy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status := m.sysProxy.Status()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": status.Enabled,
			"port":    status.Port,
		})
	case http.MethodPost:
		action := r.URL.Query().Get("action")
		switch action {
		case "enable":
			port := r.URL.Query().Get("port")
			if port == "" {
				port = m.ruleManager.GetListenPort()
				if port == "" {
					port = "8080"
				}
			}
			p, err := strconv.Atoi(port)
			if err != nil {
				http.Error(w, "invalid port", 400)
				return
			}
			if err := m.sysProxy.Enable(p); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
		case "disable":
			if err := m.sysProxy.Disable(); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
		default:
			http.Error(w, "unknown action", 400)
		}
	}
}

func (m *LinuxFeaturesManager) handleTUN(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := m.ruleManager.GetTUNConfig()
		status := m.tunManager.Status(cfg)
		json.NewEncoder(w).Encode(status)
	case http.MethodPost:
		action := r.URL.Query().Get("action")
		switch action {
		case "enable":
			var cfg proxy.TUNConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			port := m.ruleManager.GetListenPort()
			if port == "" {
				port = "8080"
			}
			p, err := strconv.Atoi(port)
			if err != nil {
				http.Error(w, "invalid port", 400)
				return
			}
			if err := m.tunManager.Start(cfg, p, m.app.appendLog); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			m.ruleManager.UpdateTUNConfig(cfg)
			json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
		case "disable":
			if err := m.tunManager.Stop(m.app.appendLog); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
		case "restart":
			cfg := m.ruleManager.GetTUNConfig()
			port := m.ruleManager.GetListenPort()
			if port == "" {
				port = "8080"
			}
			p, err := strconv.Atoi(port)
			if err != nil {
				http.Error(w, "invalid port", 400)
				return
			}
			if err := m.tunManager.RestartIfRunning(cfg, p, m.app.appendLog); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "restarted"})
		default:
			http.Error(w, "unknown action", 400)
		}
	}
}

func (m *LinuxFeaturesManager) handleAutoStart(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		enabled := isAutoStartEnabled()
		json.NewEncoder(w).Encode(map[string]bool{"enabled": enabled})
	case http.MethodPost:
		action := r.URL.Query().Get("action")
		switch action {
		case "enable":
			execPath, err := os.Executable()
			if err != nil {
				http.Error(w, "failed to get executable path", 500)
				return
			}
			command := buildAutoStartCommand(execPath)
			if err := setAutoStartEnabled(true, command); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
		case "disable":
			if err := setAutoStartEnabled(false, ""); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
		default:
			http.Error(w, "unknown action", 400)
		}
	}
}

func (m *LinuxFeaturesManager) handleSingleInstance(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		lockPath := getLockFilePath()
		exists := false
		if _, err := os.Stat(lockPath); err == nil {
			exists = true
		}
		json.NewEncoder(w).Encode(map[string]bool{"enabled": exists})
	}
}

func (m *LinuxFeaturesManager) handleEnhancedSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.Header.Get("Accept") == "application/json" {
			tunCfg := m.ruleManager.GetTUNConfig()
			tunStatus := m.tunManager.Status(tunCfg)
			sysProxyStatus := m.sysProxy.Status()
			autoStartEnabled := isAutoStartEnabled()

			json.NewEncoder(w).Encode(map[string]interface{}{
				"tun":        tunCfg,
				"tun_status": tunStatus,
				"sys_proxy":  sysProxyStatus,
				"auto_start": autoStartEnabled,
				"cert":       m.app.certManager.GetCAInstallStatus(),
				"host":       m.ruleManager.GetServerHost(),
				"auth":       m.ruleManager.GetServerAuth(),
				"port":       m.ruleManager.GetListenPort(),
				"api_port":   m.ruleManager.GetApiPort(),
			})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := PageData{Title: "Settings"}
		if err := getPageTemplate("settings").ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		action := r.URL.Query().Get("action")
		switch action {
		case "tun":
			var cfg proxy.TUNConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			m.ruleManager.UpdateTUNConfig(cfg)
			port := m.ruleManager.GetListenPort()
			if port == "" {
				port = "8080"
			}
			p, err := strconv.Atoi(port)
			if err == nil {
				_ = m.tunManager.RestartIfRunning(cfg, p, m.app.appendLog)
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "sysproxy":
			var req struct {
				Enabled bool `json:"enabled"`
				Port    int  `json:"port"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if req.Enabled {
				if req.Port <= 0 {
					port := m.ruleManager.GetListenPort()
					if port == "" {
						port = "8080"
					}
					p, err := strconv.Atoi(port)
					if err != nil {
						http.Error(w, "invalid port", 400)
						return
					}
					req.Port = p
				}
				if err := m.sysProxy.Enable(req.Port); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			} else {
				if err := m.sysProxy.Disable(); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "autostart":
			var req struct {
				Enabled bool `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if req.Enabled {
				execPath, err := os.Executable()
				if err != nil {
					http.Error(w, "failed to get executable path", 500)
					return
				}
				command := buildAutoStartCommand(execPath)
				if err := setAutoStartEnabled(true, command); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			} else {
				if err := setAutoStartEnabled(false, ""); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "general":
			var settings map[string]string
			if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			host := settings["host"]
			auth := settings["auth"]
			port := settings["port"]
			apiPort := settings["api_port"]
			if host != "" || auth != "" {
				m.ruleManager.UpdateServerConfig(host, auth)
			}
			if port != "" {
				m.ruleManager.SetListenPort(port)
			}
			if apiPort != "" {
				m.ruleManager.SetApiPort(apiPort)
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			http.Error(w, "unknown action", 400)
		}
	}
}

func (m *LinuxFeaturesManager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/sysproxy", m.handleSystemProxy)
	mux.HandleFunc("/api/tun", m.handleTUN)
	mux.HandleFunc("/api/autostart", m.handleAutoStart)
	mux.HandleFunc("/api/singleinstance", m.handleSingleInstance)
	mux.HandleFunc("/api/settings", m.handleEnhancedSettings)
}

func (m *LinuxFeaturesManager) EnableSysProxy(port int) error {
	return m.sysProxy.Enable(port)
}

func (m *LinuxFeaturesManager) DisableSysProxy() error {
	return m.sysProxy.Disable()
}

func (m *LinuxFeaturesManager) StartTUN(cfg proxy.TUNConfig, proxyPort int) error {
	return m.tunManager.Start(cfg, proxyPort, m.app.appendLog)
}

func (m *LinuxFeaturesManager) StopTUN() error {
	return m.tunManager.Stop(m.app.appendLog)
}

func (m *LinuxFeaturesManager) GetTUNStatus(cfg proxy.TUNConfig) proxy.TUNStatus {
	return m.tunManager.Status(cfg)
}

func (m *LinuxFeaturesManager) GetSysProxyStatus() sysproxy.SystemProxyStatus {
	return m.sysProxy.Status()
}
