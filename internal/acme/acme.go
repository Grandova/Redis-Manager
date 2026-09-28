package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/acme"

	"redis-manager/internal/config"
	"redis-manager/internal/system"
)

type IssueCertOptions struct {
	Domain     string `json:"domain"`
	Email      string `json:"email"`
	Provider   string `json:"provider"`    // "letsencrypt", "zerossl", "self-signed"
	UseStaging bool   `json:"use_staging"` // Use Let's Encrypt Staging environment for testing
}

type CertInfo struct {
	Domain        string    `json:"domain"`
	Issuer        string    `json:"issuer"`
	Subject       string    `json:"subject"`
	NotBefore     time.Time `json:"not_before"`
	NotAfter      time.Time `json:"not_after"`
	DaysRemaining int       `json:"days_remaining"`
	CertPath      string    `json:"cert_path"`
	KeyPath       string    `json:"key_path"`
	CAPath        string    `json:"ca_path"`
	IsExpired     bool      `json:"is_expired"`
}

// RequestCertificate initiates ACME HTTP-01 certificate issuance for domain
func RequestCertificate(ctx context.Context, opts IssueCertOptions) (*CertInfo, error) {
	opts.Domain = strings.TrimSpace(strings.ToLower(opts.Domain))
	if opts.Domain == "" {
		return nil, fmt.Errorf("域名不能为空")
	}

	// 1. Verify DNS resolution matches host
	dnsCheck, err := system.CheckDomainDNS(opts.Domain)
	if err != nil {
		return nil, fmt.Errorf("DNS 预检查失败: %w", err)
	}
	if !dnsCheck.IsMatched {
		return nil, fmt.Errorf("域名尚未正确解析到当前服务器公网 IP (%s)", dnsCheck.MatchError)
	}

	// 2. Verify Port 80 availability
	portCheck, err := system.CheckPort(80)
	if err != nil {
		return nil, fmt.Errorf("检查 80 端口出错: %w", err)
	}
	if portCheck.InUse {
		proc := portCheck.ProcessName
		if proc == "" {
			proc = "其他服务"
		}
		return nil, fmt.Errorf("80 端口正在被 %s (PID: %d) 占用，ACME HTTP-01 验证需要使用 80 端口", proc, portCheck.PID)
	}

	cfg := config.Get()
	domainCertDir := filepath.Join(cfg.CertDir, opts.Domain)
	_ = os.MkdirAll(domainCertDir, 0755)

	certPath := filepath.Join(domainCertDir, "fullchain.pem")
	keyPath := filepath.Join(domainCertDir, "privkey.pem")
	caPath := filepath.Join(domainCertDir, "ca.pem")

	// If domain is localhost or provider is self-signed, generate trusted self-signed cert
	if opts.Domain == "localhost" || opts.Domain == "127.0.0.1" || opts.Provider == "self-signed" {
		return generateSelfSignedCert(opts.Domain, certPath, keyPath, caPath)
	}

	// 3. Setup ACME Client
	directoryURL := acme.LetsEncryptURL
	if opts.UseStaging {
		directoryURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
	}

	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成 ACME 账户私钥失败: %w", err)
	}

	client := &acme.Client{
		Key:          accountKey,
		DirectoryURL: directoryURL,
	}

	// Register ACME account
	account := &acme.Account{
		Contact: []string{},
	}
	if opts.Email != "" {
		account.Contact = append(account.Contact, "mailto:"+opts.Email)
	}

	_, err = client.Register(ctx, account, acme.AcceptTOS)
	if err != nil && !strings.Contains(err.Error(), "already registered") {
		return nil, fmt.Errorf("注册 Let's Encrypt 账户失败: %w", err)
	}

	// 4. Create Order / Authorization
	order, err := client.AuthorizeOrder(ctx, []acme.AuthzID{{Type: "dns", Value: opts.Domain}})
	if err != nil {
		return nil, fmt.Errorf("创建证书订单失败: %w", err)
	}

	// Handle HTTP-01 challenge
	challengeToken := ""
	challengeKeyAuth := ""

	for _, authURL := range order.AuthzURLs {
		authz, err := client.GetAuthorization(ctx, authURL)
		if err != nil {
			return nil, fmt.Errorf("获取授权信息失败: %w", err)
		}

		if authz.Status == acme.StatusValid {
			continue
		}

		for _, ch := range authz.Challenges {
			if ch.Type == "http-01" {
				challengeToken = ch.Token
				keyAuth, err := client.HTTP01ChallengeResponse(ch.Token)
				if err != nil {
					return nil, fmt.Errorf("计算 HTTP-01 challenge 响应失败: %w", err)
				}
				challengeKeyAuth = keyAuth

				// Start temporary HTTP server on :80
				srv := &http.Server{
					Addr: ":80",
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						expectedPath := client.HTTP01ChallengePath(challengeToken)
						if r.URL.Path == expectedPath {
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(challengeKeyAuth))
							return
						}
						http.NotFound(w, r)
					}),
				}

				ln, err := net.Listen("tcp", ":80")
				if err != nil {
					return nil, fmt.Errorf("绑定 80 端口验证服务失败: %w", err)
				}

				go func() {
					_ = srv.Serve(ln)
				}()
				defer srv.Shutdown(ctx)

				// Accept challenge
				if _, err := client.Accept(ctx, ch); err != nil {
					return nil, fmt.Errorf("提交 ACME Challenge 失败: %w", err)
				}

				// Wait for authorization status valid
				if _, err := client.WaitAuthorization(ctx, authURL); err != nil {
					return nil, fmt.Errorf("Let's Encrypt 验证 HTTP-01 失败: %w。请确认 80 端口公网可达", err)
				}
				break
			}
		}
	}

	// 5. Generate Private Key and CSR for Domain
	domainKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成域名私钥失败: %w", err)
	}

	csrTemplate := &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: opts.Domain},
		DNSNames: []string{opts.Domain},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, domainKey)
	if err != nil {
		return nil, fmt.Errorf("生成 CSR 请求失败: %w", err)
	}

	// 6. Finalize Order and Fetch Certificate
	derCerts, certURL, err := client.CreateOrderCert(ctx, order.FinalizeURL, csrDER, true)
	if err != nil {
		return nil, fmt.Errorf("签发证书失败: %w", err)
	}
	_ = certURL

	// 7. Write certificates to disk
	certFile, err := os.OpenFile(certPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("保存证书文件失败: %w", err)
	}
	for _, b := range derCerts {
		_ = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: b})
	}
	certFile.Close()

	// Write private key with 0644 permissions (for service access)
	keyDER, _ := x509.MarshalECPrivateKey(domainKey)
	keyFile, err := os.OpenFile(keyPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("保存私钥文件失败: %w", err)
	}
	_ = pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyFile.Close()

	// If there's an intermediate/CA cert, write it
	if len(derCerts) > 1 {
		caFile, _ := os.OpenFile(caPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		_ = pem.Encode(caFile, &pem.Block{Type: "CERTIFICATE", Bytes: derCerts[len(derCerts)-1]})
		caFile.Close()
	}

	return InspectCertificate(certPath, keyPath, caPath)
}

// InspectCertificate parses an existing certificate file on disk
func InspectCertificate(certPath, keyPath, caPath string) (*CertInfo, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取证书文件: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("证书 PEM 解析失败")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析 X509 证书内容失败: %w", err)
	}

	now := time.Now()
	days := int(cert.NotAfter.Sub(now).Hours() / 24)
	isExpired := now.After(cert.NotAfter)

	info := &CertInfo{
		Domain:        cert.Subject.CommonName,
		Issuer:        cert.Issuer.CommonName,
		Subject:       cert.Subject.CommonName,
		NotBefore:     cert.NotBefore,
		NotAfter:      cert.NotAfter,
		DaysRemaining: days,
		CertPath:      certPath,
		KeyPath:       keyPath,
		CAPath:        caPath,
		IsExpired:     isExpired,
	}

	return info, nil
}

// generateSelfSignedCert generates a valid local certificate for testing or intranet use
func generateSelfSignedCert(domain, certPath, keyPath, caPath string) (*CertInfo, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			Organization: []string{"Redis Manager Local"},
			CommonName:   domain,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if ip := net.ParseIP(domain); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, domain)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certOut, err := os.OpenFile(certPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyDER, _ := x509.MarshalECPrivateKey(priv)
	keyOut, err := os.OpenFile(keyPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	_ = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyOut.Close()

	// Write ca file as well
	caOut, _ := os.OpenFile(caPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	_ = pem.Encode(caOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	caOut.Close()

	return InspectCertificate(certPath, keyPath, caPath)
}

// GetOrInspectExistingCert checks if a valid certificate for the given domain already exists locally
func GetOrInspectExistingCert(domain string) (*CertInfo, bool) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return nil, false
	}

	cfg := config.Get()
	domainCertDir := filepath.Join(cfg.CertDir, domain)
	certPath := filepath.Join(domainCertDir, "fullchain.pem")
	keyPath := filepath.Join(domainCertDir, "privkey.pem")
	caPath := filepath.Join(domainCertDir, "ca.pem")

	// 1. Check /etc/redis-manager/certs/<domain>/
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			if info, err := InspectCertificate(certPath, keyPath, caPath); err == nil && !info.IsExpired && info.DaysRemaining > 2 {
				return info, true
			}
		}
	}

	// 2. Fallback check /etc/redis/tls/<domain>.crt
	altCert := filepath.Join("/etc/redis/tls", domain+".crt")
	altKey := filepath.Join("/etc/redis/tls", domain+".key")
	altCA := filepath.Join("/etc/redis/tls", domain+"-ca.crt")
	if _, err := os.Stat(altCert); err == nil {
		if _, err := os.Stat(altKey); err == nil {
			if info, err := InspectCertificate(altCert, altKey, altCA); err == nil && !info.IsExpired && info.DaysRemaining > 2 {
				return info, true
			}
		}
	}

	return nil, false
}

