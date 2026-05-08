//go:build windows

package proxy

type SystemProxyManager struct{}

func NewSystemProxyManager(apiPort int) *SystemProxyManager {
	return &SystemProxyManager{}
}

func (m *SystemProxyManager) Enable(port int) error {
	return nil
}

func (m *SystemProxyManager) Disable() error {
	return nil
}

type SystemProxyStatus struct {
	Enabled bool
	Port    int
}

func (m *SystemProxyManager) Status() SystemProxyStatus {
	return SystemProxyStatus{
		Enabled: false,
		Port:    0,
	}
}
