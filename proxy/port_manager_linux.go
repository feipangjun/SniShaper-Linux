package proxy

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func FindProcessByPort(port int) (int, error) {
	file, err := os.Open("/proc/net/tcp")
	if err != nil {
		return findProcessByPortSS(port)
	}
	defer file.Close()

	hexPort := fmt.Sprintf("%04X", port)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 10 {
			continue
		}
		localAddr := parts[1]
		addrParts := strings.Split(localAddr, ":")
		if len(addrParts) != 2 {
			continue
		}
		if strings.EqualFold(addrParts[1], hexPort) {
			inode := parts[9]
			pid, err := findPIDByInode(inode)
			if err == nil && pid > 0 {
				return pid, nil
			}
		}
	}

	return 0, nil
}

func findProcessByPortSS(port int) (int, error) {
	out, err := exec.Command("ss", "-tlnp").CombinedOutput()
	if err != nil {
		return 0, err
	}

	portStr := fmt.Sprintf(":%d", port)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, portStr) {
			continue
		}
		if idx := strings.Index(line, "pid="); idx >= 0 {
			rest := line[idx+4:]
			if endIdx := strings.IndexAny(rest, ",)"); endIdx >= 0 {
				pid, err := strconv.Atoi(rest[:endIdx])
				if err == nil {
					return pid, nil
				}
			}
		}
	}
	return 0, nil
}

func findPIDByInode(targetInode string) (int, error) {
	procDirs, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}

	for _, entry := range procDirs {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}

		fdsPath := filepath.Join("/proc", entry.Name(), "fd")
		fds, err := os.ReadDir(fdsPath)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdsPath, fd.Name()))
			if err != nil {
				continue
			}
			if strings.Contains(link, "socket:["+targetInode+"]") {
				return pid, nil
			}
		}
	}

	return 0, nil
}

func GetProcessNameByPID(pid int) (string, error) {
	commPath := fmt.Sprintf("/proc/%d/comm", pid)
	data, err := os.ReadFile(commPath)
	if err != nil {
		return "", fmt.Errorf("process not found: %d", pid)
	}
	return strings.TrimSpace(string(data)), nil
}

func KillProcessByPID(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
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
				if strings.EqualFold(name, self) {
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
