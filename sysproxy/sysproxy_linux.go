//go:build linux

package sysproxy

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

var (
	proxyMu     sync.Mutex
	proxyActive bool
	proxyPort   int
	apiPort     int
)

type SystemProxyStatus struct {
	Enabled bool
	Port    int
}

func GetSystemProxyStatus() SystemProxyStatus {
	proxyMu.Lock()
	defer proxyMu.Unlock()

	return SystemProxyStatus{
		Enabled: proxyActive,
		Port:    proxyPort,
	}
}

func EnableSystemProxy(port int, apiP int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}

	proxyMu.Lock()
	defer proxyMu.Unlock()

	if proxyActive {
		if err := disableSystemProxyLocked(); err != nil {
			return fmt.Errorf("failed to disable existing proxy: %w", err)
		}
	}

	apiPort = apiP
	if err := setupIptables(port, apiPort); err != nil {
		return fmt.Errorf("failed to setup iptables: %w", err)
	}

	proxyActive = true
	proxyPort = port
	return nil
}

func DisableSystemProxy() error {
	proxyMu.Lock()
	defer proxyMu.Unlock()

	return disableSystemProxyLocked()
}

func disableSystemProxyLocked() error {
	if !proxyActive {
		return nil
	}

	if err := cleanupIptables(proxyPort); err != nil {
		return fmt.Errorf("failed to cleanup iptables: %w", err)
	}

	proxyActive = false
	proxyPort = 0
	return nil
}

// Cloudflare IPv4 CIDR ranges
var cloudflareCIDRs = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22",
	"103.31.4.0/22", "141.101.64.0/18", "108.162.192.0/18",
	"190.93.240.0/20", "188.114.96.0/20", "197.234.240.0/22",
	"198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
}

func setupIptables(port int, apiPort int) error {
	// Create a custom chain for Cloudflare IPs
	if err := runIptables("-t", "nat", "-N", "SNI_CF"); err != nil {
		// Chain might already exist, clean it first
		runIptables("-t", "nat", "-F", "SNI_CF")
	}

	// Add exclusion rules: skip proxy for local API and proxy ports
	// This prevents the proxy from intercepting its own traffic
	if err := runIptables("-t", "nat", "-A", "SNI_CF", "-p", "tcp", "--dport", fmt.Sprintf("%d", port), "-j", "RETURN"); err != nil {
		cleanupIptables(port)
		return fmt.Errorf("failed to add proxy port exclusion: %w", err)
	}
	if apiPort > 0 && apiPort != port {
		if err := runIptables("-t", "nat", "-A", "SNI_CF", "-p", "tcp", "--dport", fmt.Sprintf("%d", apiPort), "-j", "RETURN"); err != nil {
			cleanupIptables(port)
			return fmt.Errorf("failed to add API port exclusion: %w", err)
		}
	}

	// Add Cloudflare CIDR rules to the custom chain
	for _, cidr := range cloudflareCIDRs {
		args := []string{"-t", "nat", "-A", "SNI_CF", "-d", cidr, "-p", "tcp", "-m", "multiport", "--dports", "80,443", "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", port)}
		if err := runIptables(args...); err != nil {
			cleanupIptables(port)
			return fmt.Errorf("failed to add CF rule: %v", args)
		}
	}

	// Jump to our chain from OUTPUT for CF IPs only
	if err := runIptables("-t", "nat", "-A", "OUTPUT", "-j", "SNI_CF"); err != nil {
		cleanupIptables(port)
		return fmt.Errorf("failed to add OUTPUT jump: %v", err)
	}

	return nil
}

func cleanupIptables(port int) error {
	var firstErr error

	// Remove the OUTPUT jump rule
	if err := runIptables("-t", "nat", "-D", "OUTPUT", "-j", "SNI_CF"); err != nil {
		if firstErr == nil {
			firstErr = fmt.Errorf("failed to remove OUTPUT jump: %w", err)
		}
	}

	// Flush and delete the custom chain
	if err := runIptables("-t", "nat", "-F", "SNI_CF"); err != nil {
		if firstErr == nil {
			firstErr = fmt.Errorf("failed to flush chain: %w", err)
		}
	}
	if err := runIptables("-t", "nat", "-X", "SNI_CF"); err != nil {
		if firstErr == nil {
			firstErr = fmt.Errorf("failed to delete chain: %w", err)
		}
	}

	return firstErr
}

func runIptables(args ...string) error {
	cmd := exec.Command("iptables", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables %v failed: %s: %w", args, strings.TrimSpace(string(output)), err)
	}
	return nil
}

func CleanupIptablesOnExit() {
	runIptables("-t", "nat", "-D", "OUTPUT", "-j", "SNI_CF")
	runIptables("-t", "nat", "-F", "SNI_CF")
	runIptables("-t", "nat", "-X", "SNI_CF")
}

func SetSystemProxy(enable bool, server string) error {
	if enable {
		parts := strings.Split(server, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid server format: %s", server)
		}
		port := 0
		fmt.Sscanf(parts[1], "%d", &port)
		return EnableSystemProxy(port, 5173)
	}
	return DisableSystemProxy()
}

func GetSystemProxyStatusSafe() (SystemProxyStatus, error) {
	return GetSystemProxyStatus(), nil
}

var originalProxySettings *SystemProxyStatus

func SaveOriginalProxySettings() error {
	status := GetSystemProxyStatus()
	originalProxySettings = &status
	return nil
}

func SetOriginalProxySettings(status SystemProxyStatus) {
	copy := status
	originalProxySettings = &copy
}

func RestoreOriginalProxySettings() error {
	if originalProxySettings == nil {
		return nil
	}

	if originalProxySettings.Enabled {
		return EnableSystemProxy(originalProxySettings.Port, apiPort)
	}
	return DisableSystemProxy()
}
