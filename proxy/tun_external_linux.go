//go:build linux

package proxy

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type externalTUNManager struct {
	mu        sync.Mutex
	corePath  string
	tunName   string
	pidPath   string
	logBuffer *simpleLogBuffer
}

type simpleLogBuffer struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func newSimpleLogBuffer(max int) *simpleLogBuffer {
	if max <= 0 {
		max = 500
	}
	return &simpleLogBuffer{
		lines: make([]string, 0, max),
		max:   max,
	}
}

func (b *simpleLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	line := strings.TrimSpace(string(p))
	if line == "" {
		return len(p), nil
	}

	b.lines = append(b.lines, line)
	if len(b.lines) > b.max {
		b.lines = b.lines[len(b.lines)-b.max:]
	}
	return len(p), nil
}

func (b *simpleLogBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = b.lines[:0]
}

func (b *simpleLogBuffer) Snapshot(limit int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if limit <= 0 || limit > len(b.lines) {
		limit = len(b.lines)
	}
	if limit == 0 {
		return []string{}
	}

	start := len(b.lines) - limit
	result := make([]string, limit)
	copy(result, b.lines[start:])
	return result
}

func newExternalTUNManager() *externalTUNManager {
	root := resolveExternalTUNRoot()
	return &externalTUNManager{
		corePath:  filepath.Join(root, "tun2socks"),
		tunName:   "snishaper-tun",
		pidPath:   filepath.Join(root, "tun2socks.pid"),
		logBuffer: newSimpleLogBuffer(500),
	}
}

func resolveExternalTUNRoot() string {
	if execPath, err := os.Executable(); err == nil {
		if execDir := strings.TrimSpace(filepath.Dir(execPath)); execDir != "" {
			return execDir
		}
	}
	return "."
}

func (m *externalTUNManager) getTUN2socksPath() string {
	// 优先查找可执行文件同目录下的tun2socks
	if execPath, err := os.Executable(); err == nil {
		execDir := strings.TrimSpace(filepath.Dir(execPath))
		if execDir != "" {
			path := filepath.Join(execDir, "tun2socks")
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	// 查找当前工作目录下的tun2socks
	if cwd, err := os.Getwd(); err == nil {
		path := filepath.Join(cwd, "tun2socks")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 默认返回同目录下的tun2socks
	return filepath.Join(filepath.Dir(m.corePath), "tun2socks")
}

func (m *externalTUNManager) Start(cfg TUNConfig, proxyPort int, logf func(string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if runtime.GOOS != "linux" {
		return fmt.Errorf("external tun2socks TUN is only supported on Linux")
	}

	if proxyPort < 1 || proxyPort > 65535 {
		return fmt.Errorf("invalid proxy port: %d", proxyPort)
	}

	if err := m.stopLocked(nil); err != nil {
		return err
	}

	if _, err := os.Stat(m.getTUN2socksPath()); err != nil {
		return fmt.Errorf("tun2socks not found at %s, please download it first", m.getTUN2socksPath())
	}

	proxyAddr := fmt.Sprintf("http://127.0.0.1:%d", proxyPort)

	args := []string{
		"-device", m.tunName,
		"-proxy", proxyAddr,
	}

	cmd := exec.Command(m.getTUN2socksPath(), args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	m.logBuffer.Clear()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tun2socks: %w", err)
	}

	_ = os.WriteFile(m.pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

	readyChan := make(chan struct{}, 1)
	fatalChan := make(chan string, 1)

	checkLine := func(line string) {
		if isReadyTUNLine(line) {
			select {
			case readyChan <- struct{}{}:
			default:
			}
			return
		}
		if fatal := fatalTUNLine(line); fatal != "" {
			select {
			case fatalChan <- fatal:
			default:
			}
		}
	}

	pipeDrain := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = m.logBuffer.Write([]byte(line + "\n"))
			checkLine(line)
		}
	}

	if stdout != nil {
		go pipeDrain(stdout)
	}
	if stderr != nil {
		go pipeDrain(stderr)
	}

	go func() {
		err := cmd.Wait()
		if logf != nil {
			if err != nil {
				logf(fmt.Sprintf("[tun2socks] process exited: %v", err))
			} else {
				logf("[tun2socks] process exited")
			}
		}
	}()

	if err := m.setupTUNInterface(); err != nil {
		m.stopLocked(logf)
		return fmt.Errorf("failed to setup TUN interface: %w", err)
	}

	if cfg.AutoRoute {
		if err := m.setupAutoRoute(); err != nil {
			m.stopLocked(logf)
			return fmt.Errorf("failed to setup auto route: %w", err)
		}
	}

	if cfg.DNSHijack {
		if err := m.setupDNSHijack(proxyPort); err != nil {
			m.stopLocked(logf)
			return fmt.Errorf("failed to setup DNS hijack: %w", err)
		}
	}

	timeout := time.NewTimer(12 * time.Second)
	defer timeout.Stop()

	select {
	case <-readyChan:
		if logf != nil {
			logf("[tun2socks] external TUN started")
		}
		return nil
	case msg := <-fatalChan:
		m.stopLocked(logf)
		return fmt.Errorf("tun2socks fatal error: %s", msg)
	case <-timeout.C:
		_, message := m.runningStateLocked(cfg)
		if strings.TrimSpace(message) != "" {
			return fmt.Errorf("tun2socks timeout: %s", message)
		}
		return fmt.Errorf("external tun2socks TUN did not enter running state within 12s")
	}
}

func (m *externalTUNManager) Stop(logf func(string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked(logf)
}

func (m *externalTUNManager) stopLocked(logf func(string)) error {
	pid, _ := m.readPIDLocked()
	if pid > 0 {
		proc, err := os.FindProcess(pid)
		if err == nil && proc != nil {
			_ = proc.Kill()
		}
		_ = os.Remove(m.pidPath)
	}

	m.cleanupTUNInterface()

	if logf != nil {
		logf("[tun2socks] external TUN stopped")
	}
	return nil
}

func (m *externalTUNManager) setupTUNInterface() error {
	commands := []struct {
		cmd  string
		args []string
	}{
		{"ip", []string{"addr", "add", "198.18.0.1/15", "dev", m.tunName}},
		{"ip", []string{"link", "set", "dev", m.tunName, "up"}},
	}

	for _, c := range commands {
		if err := runCommand(c.cmd, c.args...); err != nil {
			return fmt.Errorf("failed to run %s %v: %w", c.cmd, c.args, err)
		}
	}

	return nil
}

func (m *externalTUNManager) setupAutoRoute() error {
	if err := runCommand("ip", "route", "add", "default", "via", "198.18.0.1", "dev", m.tunName, "metric", "1"); err != nil {
		return err
	}
	return nil
}

func (m *externalTUNManager) setupDNSHijack(_ int) error {
	dnsPort := 1053
	rules := []struct {
		table string
		chain string
		args  []string
	}{
		{"nat", "OUTPUT", []string{"-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", dnsPort)}},
		{"nat", "OUTPUT", []string{"-p", "tcp", "--dport", "53", "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", dnsPort)}},
	}

	for _, rule := range rules {
		args := append([]string{"-t", rule.table, "-A", rule.chain}, rule.args...)
		if err := runCommand("iptables", args...); err != nil {
			return err
		}
	}

	return nil
}

func (m *externalTUNManager) cleanupTUNInterface() error {
	dnsPort := 1053
	rules := []struct {
		table string
		chain string
		args  []string
	}{
		{"nat", "OUTPUT", []string{"-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", dnsPort)}},
		{"nat", "OUTPUT", []string{"-p", "tcp", "--dport", "53", "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", dnsPort)}},
	}

	for _, rule := range rules {
		args := append([]string{"-t", rule.table, "-D", rule.chain}, rule.args...)
		runCommand("iptables", args...)
	}

	runCommand("ip", "route", "del", "default", "dev", m.tunName)
	runCommand("ip", "link", "del", m.tunName)

	return nil
}

func (m *externalTUNManager) RestartIfRunning(cfg TUNConfig, proxyPort int, logf func(string)) error {
	m.mu.Lock()
	_, pid := m.statusMessageLocked(cfg)
	running := pid > 0
	m.mu.Unlock()

	if !running {
		return nil
	}

	if logf != nil {
		logf("[tun2socks] reloading external TUN config")
	}

	return m.Start(cfg, proxyPort, logf)
}

func (m *externalTUNManager) Status(cfg TUNConfig) TUNStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := TUNStatus{
		Supported: runtime.GOOS == "linux",
		Enabled:   false,
		Running:   false,
		Driver:    "tun2socks",
		Message:   "External tun2socks TUN is configured but not running",
	}

	if _, err := os.Stat(m.corePath); err != nil {
		status.Message = fmt.Sprintf("tun2socks not found: %s", m.corePath)
		return status
	}

	running, message := m.runningStateLocked(cfg)
	status.Running = running
	status.Enabled = running
	if strings.TrimSpace(message) != "" {
		status.Message = message
	}

	return status
}

func (m *externalTUNManager) runningStateLocked(cfg TUNConfig) (bool, string) {
	message, pid := m.statusMessageLocked(cfg)
	if pid == 0 {
		return false, message
	}

	lines := m.recentLogLinesLocked(200)
	for _, line := range lines {
		if isReadyTUNLine(line) {
			return true, "External tun2socks TUN is running"
		}
	}

	return false, message
}

func (m *externalTUNManager) statusMessageLocked(_ TUNConfig) (string, int) {
	pid, _ := m.readPIDLocked()
	if pid == 0 {
		return "External tun2socks TUN is configured but not running", 0
	}

	if !processRunning(pid) {
		_ = os.Remove(m.pidPath)
		return "External tun2socks process is not running", 0
	}

	lines := m.recentLogLinesLocked(200)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if fatal := fatalTUNLine(line); fatal != "" {
			return fatal, pid
		}
		if isReadyTUNLine(line) {
			return "External tun2socks TUN is running", pid
		}
	}

	return "External tun2socks process is starting", pid
}

func (m *externalTUNManager) recentLogLinesLocked(limit int) []string {
	if m.logBuffer == nil {
		return []string{}
	}
	return m.logBuffer.Snapshot(limit)
}

func (m *externalTUNManager) readPIDLocked() (int, error) {
	data, err := os.ReadFile(m.pidPath)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid tun2socks pid")
	}

	return pid, nil
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	_, err := os.FindProcess(pid)
	return err == nil
}

func isReadyTUNLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	if lower == "" {
		return false
	}

	return strings.Contains(lower, "tun") ||
		strings.Contains(lower, "running") ||
		strings.Contains(lower, "started")
}

func fatalTUNLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	lower := strings.ToLower(line)
	if strings.Contains(lower, "error") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "permission denied") {
		return line
	}

	return ""
}

func runCommand(cmd string, args ...string) error {
	output, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %s: %w", cmd, args, strings.TrimSpace(string(output)), err)
	}
	return nil
}

func isRoot() bool {
	return os.Geteuid() == 0
}
