package https

import (
	"fmt"

	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/providers/dns/alidns"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/tencentcloud"
)

// NewDNSProvider 根据配置创建 DNS 提供商
func NewDNSProvider(cfg *Config) (challenge.Provider, error) {
	creds := cfg.ACME.DNS.Credentials

	switch cfg.ACME.DNS.Provider {
	case "tencentcloud", "dnspod", "tencent":
		// 腾讯云 DNS（使用腾讯云 API，SecretId/SecretKey）
		return newTencentCloudProvider(creds)
	case "alidns", "aliyun":
		return newAliDNSProvider(creds)
	case "cloudflare":
		return newCloudflareProvider(creds)
	default:
		return nil, fmt.Errorf("unsupported DNS provider: %s, supported: tencentcloud/dnspod, alidns, cloudflare", cfg.ACME.DNS.Provider)
	}
}

// newTencentCloudProvider 创建腾讯云 DNS 提供商
// 使用腾讯云 API（SecretId + SecretKey），而非旧版 DNSPod API
func newTencentCloudProvider(creds map[string]string) (challenge.Provider, error) {
	// 支持多种配置名称
	secretID := getCredValue(creds, "secret_id", "secretid", "dnspod_id")
	secretKey := getCredValue(creds, "secret_key", "secretkey", "dnspod_token")

	if secretID == "" || secretKey == "" {
		return nil, fmt.Errorf("tencentcloud requires secret_id and secret_key (or dnspod_id and dnspod_token)")
	}

	config := tencentcloud.NewDefaultConfig()
	config.SecretID = secretID
	config.SecretKey = secretKey

	return tencentcloud.NewDNSProviderConfig(config)
}

// newAliDNSProvider 创建阿里云 DNS 提供商
func newAliDNSProvider(creds map[string]string) (challenge.Provider, error) {
	keyID := getCredValue(creds, "access_key_id", "accesskeyid")
	keySecret := getCredValue(creds, "access_key_secret", "accesskeysecret")

	if keyID == "" || keySecret == "" {
		return nil, fmt.Errorf("alidns requires access_key_id and access_key_secret")
	}

	config := alidns.NewDefaultConfig()
	config.APIKey = keyID
	config.SecretKey = keySecret

	return alidns.NewDNSProviderConfig(config)
}

// newCloudflareProvider 创建 Cloudflare 提供商
func newCloudflareProvider(creds map[string]string) (challenge.Provider, error) {
	token := getCredValue(creds, "api_token", "apitoken")

	if token == "" {
		return nil, fmt.Errorf("cloudflare requires api_token")
	}

	config := cloudflare.NewDefaultConfig()
	config.AuthToken = token

	return cloudflare.NewDNSProviderConfig(config)
}

// getCredValue 从 credentials map 中获取值，支持多个 key 名称
func getCredValue(creds map[string]string, keys ...string) string {
	for _, key := range keys {
		if v, ok := creds[key]; ok && v != "" {
			return v
		}
	}
	return ""
}
