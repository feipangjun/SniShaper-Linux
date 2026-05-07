//go:build linux || darwin

package proxy

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

func FindProcessByPort(port int) (int, error) {
	cmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, nil
	}

	pidStr := strings.TrimSpace(string(out))
	if pidStr == "" {
		return 0, nil
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, err
	}

	return pid, nil
}

func GetProcessNameByPID(pid int) (string, error) {
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "comm=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	name := strings.TrimSpace(string(out))
	return name, nil
}

func KillProcessByPID(pid int) error {
	cmd := exec.Command("kill", "-9", fmt.Sprintf("%d", pid))
	return cmd.Run()
}

func EnsurePortAvailable(startPort int, selfNames []string) (int, error) {
	currentPort := startPort
	maxAttempts := 10

	for i := 0; i < maxAttempts; i++ {
		pid, err := FindProcessByPort(currentPort)
		if err != nil || pid == 0 {
			ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", currentPort))
			if err == nil {
				ln.Close()
				return currentPort, nil
			}
		} else {
			name, _ := GetProcessNameByPID(pid)
			isSelf := false
			for _, self := range selfNames {
				if strings.EqualFold(name, self) || strings.EqualFold(name, self+".exe") {
					isSelf = true
					break
				}
			}

			if isSelf {
				if err := KillProcessByPID(pid); err == nil {
					return currentPort, nil
				}
			}
		}

		currentPort++
	}

	return startPort, fmt.Errorf("could not find available port after %d attempts", maxAttempts)
}
