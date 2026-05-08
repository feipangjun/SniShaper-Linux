//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const lockFileName = "snishaper.lock"

func getLockFilePath() string {
	configDir := getDefaultConfigDir()
	return filepath.Join(configDir, lockFileName)
}

func acquireSingleInstanceLock() error {
	lockPath := getLockFilePath()

	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	if existingPID, err := readLockFile(lockPath); err == nil && existingPID > 0 {
		if isProcessRunning(existingPID) {
			return fmt.Errorf("another instance is already running (PID: %d)", existingPID)
		}
	}

	currentPID := os.Getpid()
	if err := writeLockFile(lockPath, currentPID); err != nil {
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	return nil
}

func releaseSingleInstanceLock() error {
	lockPath := getLockFilePath()
	return os.Remove(lockPath)
}

func readLockFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid lock file content: %w", err)
	}

	return pid, nil
}

func writeLockFile(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func recoverBrokenSingleInstance(uniqueID string) {
	if strings.TrimSpace(uniqueID) == "" {
		return
	}

	lockPath := getLockFilePath()
	existingPID, err := readLockFile(lockPath)
	if err != nil || existingPID <= 0 {
		return
	}

	if isProcessRunning(existingPID) {
		return
	}

	if err := os.Remove(lockPath); err != nil {
		return
	}

	time.Sleep(250 * time.Millisecond)
}
