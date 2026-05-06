package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CertManager struct {
	caCert  *x509.Certificate
	caKey   *rsa.PrivateKey
	certMu  sync.RWMutex
	caPath  string
	keyPath string
}

func InitCertManager(certPath string) (*CertManager, error) {
	if certPath == "" {
		certPath = filepath.Join(os.Getenv("HOME"), ".snishaper", "cert")
	}

	if err := os.MkdirAll(certPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cert directory: %w", err)
	}

	caPath := filepath.Join(certPath, "ca.crt")
	keyPath := filepath.Join(certPath, "ca.key")

	cm := &CertManager{
		caPath:  caPath,
		keyPath: keyPath,
	}

	if err := cm.LoadCA(); err != nil {
		return nil, fmt.Errorf("failed to load CA: %w", err)
	}

	return cm, nil
}

func (cm *CertManager) LoadCA() error {
	cm.certMu.Lock()
	defer cm.certMu.Unlock()

	caData, err := os.ReadFile(cm.caPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cm.generateCAUnlocked()
		}
		return err
	}

	block, _ := pem.Decode(caData)
	if block == nil {
		return fmt.Errorf("failed to decode CA certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	keyData, err := os.ReadFile(cm.keyPath)
	if err != nil {
		return err
	}

	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode CA key PEM")
	}

	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}

	cm.caCert = cert
	cm.caKey = key

	return nil
}

func (cm *CertManager) generateCAUnlocked() error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate CA private key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"SniShaper"},
			CommonName:   "SniShaper CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return err
	}

	cm.caCert = cert
	cm.caKey = privateKey

	if err := cm.saveCA(); err != nil {
		return err
	}

	fmt.Println("[Cert] CA certificate generated successfully")
	return nil
}

func (cm *CertManager) GenerateCA() error {
	cm.certMu.Lock()
	defer cm.certMu.Unlock()
	return cm.generateCAUnlocked()
}

func (cm *CertManager) saveCA() error {
	caFile, err := os.Create(cm.caPath)
	if err != nil {
		return err
	}
	defer caFile.Close()

	if err := pem.Encode(caFile, &pem.Block{Type: "CERTIFICATE", Bytes: cm.caCert.Raw}); err != nil {
		return err
	}

	keyFile, err := os.Create(cm.keyPath)
	if err != nil {
		return err
	}
	defer keyFile.Close()

	keyBytes := x509.MarshalPKCS1PrivateKey(cm.caKey)
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return err
	}

	return nil
}

func (cm *CertManager) GetCACertPath() string {
	return cm.caPath
}

func (cm *CertManager) GetCertPool() *x509.CertPool {
	pool := x509.NewCertPool()
	if cm.caCert != nil {
		pool.AddCert(cm.caCert)
	}
	return pool
}

func (cm *CertManager) GetCA() *x509.Certificate {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()
	return cm.caCert
}

func (cm *CertManager) GetCACert() *x509.Certificate {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()
	return cm.caCert
}

func (cm *CertManager) GetCAKey() interface{} {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()
	return cm.caKey
}

func (cm *CertManager) IsCAInstalled() bool {
	return true
}

type CAInstallStatus struct {
	Installed   bool
	Platform    string
	CertPath    string
	InstallHelp string
}

func (cm *CertManager) GetCAInstallStatus() CAInstallStatus {
	cm.certMu.RLock()
	defer cm.certMu.RUnlock()

	return CAInstallStatus{
		Installed:   true,
		Platform:    "linux",
		CertPath:    cm.caPath,
		InstallHelp: "CA 证书已生成在: " + cm.caPath + "\n如需手动安装，请将 ca.crt 复制到 /usr/local/share/ca-certificates/ 并运行 update-ca-certificates",
	}
}

func (cm *CertManager) InstallCA() error {
	caDest := "/usr/local/share/ca-certificates/snishaper-ca.crt"
	caDir := "/usr/local/share/ca-certificates"

	fi, err := os.Stat(caDir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(caDir, 0755); err != nil {
				return fmt.Errorf("failed to create CA certificates directory: %w\nCA 证书已生成在: %s", err, cm.caPath)
			}
		} else {
			return fmt.Errorf("failed to check CA certificates directory: %w", err)
		}
	} else if !fi.IsDir() {
		return fmt.Errorf("CA certificates directory is not a directory: %s", caDir)
	}

	data, err := os.ReadFile(cm.caPath)
	if err != nil {
		return err
	}

	if err := os.WriteFile(caDest, data, 0644); err != nil {
		return fmt.Errorf("failed to copy CA certificate: %w\nCA 证书已生成在: %s", err, cm.caPath)
	}

	fmt.Println("[Cert] CA 证书已安装到: " + caDest)
	return nil
}

func bigint(b []byte) *big.Int {
	return new(big.Int).SetBytes(b)
}
