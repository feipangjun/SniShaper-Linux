//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const systemdServiceTemplate = `[Unit]
Description=SniShaper CLI - Cloudflare IP Shaper
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`

const systemdUserServiceTemplate = `[Unit]
Description=SniShaper CLI - Cloudflare IP Shaper (User Service)
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
`

func buildAutoStartCommand(execPath string) string {
	trimmed := strings.TrimSpace(execPath)
	if trimmed == "" {
		return ""
	}
	return trimmed + " --startup"
}

func setAutoStartEnabled(enabled bool, command string) error {
	if !enabled {
		return disableAutoStart()
	}

	if command == "" {
		return fmt.Errorf("command is empty")
	}

	return enableAutoStart(command)
}

func enableAutoStart(command string) error {
	serviceName := "snishaper.service"
	serviceContent := fmt.Sprintf(systemdServiceTemplate, command)

	systemdDir := "/etc/systemd/system"
	servicePath := filepath.Join(systemdDir, serviceName)

	if err := os.MkdirAll(systemdDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd directory: %w", err)
	}

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	if err := runCommand("systemctl", "enable", serviceName); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	return nil
}

func disableAutoStart() error {
	serviceName := "snishaper.service"
	servicePath := "/etc/systemd/system/" + serviceName

	if err := runCommand("systemctl", "disable", serviceName); err != nil {
		return fmt.Errorf("failed to disable service: %w", err)
	}

	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	return nil
}

func isAutoStartEnabled() bool {
	if err := runCommand("systemctl", "is-enabled", "snishaper.service"); err != nil {
		return false
	}
	return true
}

func runCommand(cmd string, args ...string) error {
	output, err := execCommand(cmd, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %s: %w", cmd, args, strings.TrimSpace(string(output)), err)
	}
	return nil
}

func execCommand(cmd string, args ...string) *exec.Cmd {
	return exec.Command(cmd, args...)
}
