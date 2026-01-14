package https

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

// ACMEUser 实现 lego 的 User 接口
type ACMEUser struct {
	Email        string                 `json:"email"`
	Registration *registration.Resource `json:"registration"`
	KeyPEM       string                 `json:"key_pem"`
	key          crypto.PrivateKey
}

func (u *ACMEUser) GetEmail() string {
	return u.Email
}

func (u *ACMEUser) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u *ACMEUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// ACMEClient ACME 客户端
type ACMEClient struct {
	cfg      *Config
	certsDir string
	certFile string
	keyFile  string
}

// NewACMEClient 创建 ACME 客户端
func NewACMEClient(cfg *Config, certsDir, certFile, keyFile string) *ACMEClient {
	return &ACMEClient{
		cfg:      cfg,
		certsDir: certsDir,
		certFile: certFile,
		keyFile:  keyFile,
	}
}

// ObtainCertificate 申请证书
func (c *ACMEClient) ObtainCertificate() error {
	// 加载或创建用户
	user, err := c.loadOrCreateUser()
	if err != nil {
		return fmt.Errorf("failed to load/create user: %w", err)
	}

	// 选择 CA
	caURL := c.cfg.ACME.Directories[0]
	if len(c.cfg.ACME.Directories) == 0 {
		caURL = "https://acme-v02.api.letsencrypt.org/directory"
	}

	// 创建 lego 配置
	config := lego.NewConfig(user)
	config.CADirURL = caURL
	config.Certificate.KeyType = certcrypto.EC256

	// 创建客户端
	client, err := lego.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create lego client: %w", err)
	}

	// 设置 DNS 提供商
	dnsProvider, err := NewDNSProvider(c.cfg)
	if err != nil {
		return fmt.Errorf("failed to create DNS provider: %w", err)
	}

	if err := client.Challenge.SetDNS01Provider(dnsProvider); err != nil {
		return fmt.Errorf("failed to set DNS provider: %w", err)
	}

	// 注册用户（如果未注册）
	if user.Registration == nil {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return fmt.Errorf("failed to register: %w", err)
		}
		user.Registration = reg
		if err := c.saveUser(user); err != nil {
			log.Printf("Warning: failed to save user: %v", err)
		}
	}

	// 申请证书
	log.Printf("Requesting certificate for %s from %s", c.cfg.ACME.Domain, caURL)

	request := certificate.ObtainRequest{
		Domains: []string{c.cfg.ACME.Domain},
		Bundle:  true,
	}

	// 重试逻辑
	var cert *certificate.Resource
	retryCount := c.cfg.ACME.RetryCount
	if retryCount <= 0 {
		retryCount = 3
	}
	retryDelay := time.Duration(c.cfg.ACME.RetryDelaySeconds) * time.Second
	if retryDelay <= 0 {
		retryDelay = 5 * time.Second
	}

	for i := 0; i < retryCount; i++ {
		cert, err = client.Certificate.Obtain(request)
		if err == nil {
			break
		}
		log.Printf("Certificate request failed (attempt %d/%d): %v", i+1, retryCount, err)
		if i < retryCount-1 {
			time.Sleep(retryDelay)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to obtain certificate after %d attempts: %w", retryCount, err)
	}

	// 保存证书
	if err := c.saveCertificate(cert); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	log.Printf("Certificate obtained successfully for %s", c.cfg.ACME.Domain)
	return nil
}

// loadOrCreateUser 加载或创建用户
func (c *ACMEClient) loadOrCreateUser() (*ACMEUser, error) {
	userFile := filepath.Join(c.certsDir, "acme_user.json")

	// 尝试加载现有用户
	if data, err := os.ReadFile(userFile); err == nil {
		var user ACMEUser
		if err := json.Unmarshal(data, &user); err == nil {
			// 解析私钥
			block, _ := pem.Decode([]byte(user.KeyPEM))
			if block != nil {
				key, err := x509.ParseECPrivateKey(block.Bytes)
				if err == nil {
					user.key = key
					return &user, nil
				}
			}
		}
	}

	// 创建新用户
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	user := &ACMEUser{
		Email:  c.cfg.ACME.Email,
		KeyPEM: string(keyPEM),
		key:    privateKey,
	}

	return user, nil
}

// saveUser 保存用户
func (c *ACMEClient) saveUser(user *ACMEUser) error {
	userFile := filepath.Join(c.certsDir, "acme_user.json")
	data, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(userFile, data, 0600)
}

// saveCertificate 保存证书
func (c *ACMEClient) saveCertificate(cert *certificate.Resource) error {
	// 保存证书
	if err := os.WriteFile(c.certFile, cert.Certificate, 0644); err != nil {
		return err
	}

	// 保存私钥
	if err := os.WriteFile(c.keyFile, cert.PrivateKey, 0600); err != nil {
		return err
	}

	log.Printf("Certificate saved to %s", c.certFile)
	return nil
}
