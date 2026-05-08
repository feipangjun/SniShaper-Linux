package proxy

import (
	"runtime"
	"sync"
)

type TUNManager struct {
	mu      sync.Mutex
	impl    TUNManagerImpl
	created bool
}

type TUNManagerImpl interface {
	Start(cfg TUNConfig, proxyPort int, logf func(string)) error
	Stop(logf func(string)) error
	RestartIfRunning(cfg TUNConfig, proxyPort int, logf func(string)) error
	Status(cfg TUNConfig) TUNStatus
}

func NewTUNManager() *TUNManager {
	return &TUNManager{}
}

func (m *TUNManager) getImpl() TUNManagerImpl {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.impl != nil {
		return m.impl
	}

	if runtime.GOOS == "linux" {
		m.impl = newExternalTUNManager()
	} else {
		m.impl = newNoopTUNManager()
	}

	m.created = true
	return m.impl
}

func (m *TUNManager) Start(cfg TUNConfig, proxyPort int, logf func(string)) error {
	return m.getImpl().Start(cfg, proxyPort, logf)
}

func (m *TUNManager) Stop(logf func(string)) error {
	return m.getImpl().Stop(logf)
}

func (m *TUNManager) RestartIfRunning(cfg TUNConfig, proxyPort int, logf func(string)) error {
	return m.getImpl().RestartIfRunning(cfg, proxyPort, logf)
}

func (m *TUNManager) Status(cfg TUNConfig) TUNStatus {
	return m.getImpl().Status(cfg)
}

type noopTUNManager struct{}

func newNoopTUNManager() *noopTUNManager {
	return &noopTUNManager{}
}

func (n *noopTUNManager) Start(cfg TUNConfig, proxyPort int, logf func(string)) error {
	return nil
}

func (n *noopTUNManager) Stop(logf func(string)) error {
	return nil
}

func (n *noopTUNManager) RestartIfRunning(cfg TUNConfig, proxyPort int, logf func(string)) error {
	return nil
}

func (n *noopTUNManager) Status(cfg TUNConfig) TUNStatus {
	return TUNStatus{
		Supported: false,
		Enabled:   false,
		Running:   false,
		Message:   "TUN not supported on this platform",
	}
}
