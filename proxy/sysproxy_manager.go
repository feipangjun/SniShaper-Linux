//go:build linux

package proxy

import (
	"snishaper/sysproxy"
)

type SystemProxyManager struct {
	apiPort int
}

func NewSystemProxyManager(apiPort int) *SystemProxyManager {
	return &SystemProxyManager{
		apiPort: apiPort,
	}
}

func (m *SystemProxyManager) Enable(port int) error {
	return sysproxy.EnableSystemProxy(port, m.apiPort)
}

func (m *SystemProxyManager) Disable() error {
	return sysproxy.DisableSystemProxy()
}

func (m *SystemProxyManager) Status() sysproxy.SystemProxyStatus {
	return sysproxy.GetSystemProxyStatus()
}
