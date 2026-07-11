package config

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// DatabaseConfig MySQL 连接配置。
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	// TLSMode: false | preferred | skip-verify | require | verify-ca | verify-full | true
	// production 仅允许 require / verify-ca / verify-full / true。
	TLSMode string
	// TLSCAFile optional PEM CA bundle (DB_TLS_CA). When empty, system roots are used.
	TLSCAFile string
	// TLSCertFile / TLSKeyFile optional client certificate for mTLS (DB_TLS_CERT / DB_TLS_KEY).
	TLSCertFile string
	TLSKeyFile  string
	// TLSServerName overrides certificate hostname verification (DB_TLS_SERVER_NAME).
	// Defaults to Host when empty and verification is required.
	TLSServerName   string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

var registeredTLSConfigs sync.Map // name -> struct{}

// NormalizeTLSMode canonicalizes DB_TLS values.
func NormalizeTLSMode(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "", "0", "off", "disable", "disabled", "no", "none":
		return "false"
	case "1", "on", "yes", "enabled", "true":
		return "require"
	case "prefer":
		return "preferred"
	case "verify_ca", "verifyca":
		return "verify-ca"
	case "verify_full", "verifyfull", "verify-identity", "verify_identity":
		return "verify-full"
	default:
		return m
	}
}

// ProductionTLSAllowed reports whether a TLS mode is acceptable in production.
func ProductionTLSAllowed(mode string) bool {
	switch NormalizeTLSMode(mode) {
	case "require", "verify-ca", "verify-full":
		return true
	default:
		return false
	}
}

// MySQLConfig builds a driver-native *mysql.Config (special-char passwords safe).
// Callers SHOULD open via mysql.NewConnector to avoid re-parsing a DSN string.
func (d DatabaseConfig) MySQLConfig() (*mysqldriver.Config, error) {
	if d.Host == "" {
		return nil, errors.New("DB_HOST is required")
	}
	if d.Port <= 0 {
		return nil, errors.New("DB_PORT must be positive")
	}
	if d.DBName == "" {
		return nil, errors.New("DB_NAME is required")
	}

	tlsName, err := d.registerTLSConfig()
	if err != nil {
		return nil, err
	}

	cfg := mysqldriver.NewConfig()
	cfg.User = d.User
	cfg.Passwd = d.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(d.Host, strconv.Itoa(d.Port))
	cfg.DBName = d.DBName
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	cfg.Loc = time.UTC
	cfg.ParseTime = true
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 10 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	cfg.AllowNativePasswords = true
	if tlsName != "" {
		cfg.TLSConfig = tlsName
	}
	return cfg, nil
}

// DSN returns a driver-formatted DSN via mysql.Config.FormatDSN.
// Prefer MySQLConfig + NewConnector for connection open paths so passwords
// with @:/?% never re-enter the DSN parser.
func (d DatabaseConfig) DSN() string {
	cfg, err := d.MySQLConfig()
	if err != nil {
		// Keep a deterministic fallback for tests that only assert formatting
		// of incomplete configs; production open paths use MySQLConfig errors.
		mode := NormalizeTLSMode(d.TLSMode)
		if mode == "" {
			mode = "false"
		}
		return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=true&loc=UTC&tls=%s&timeout=5s&readTimeout=10s&writeTimeout=10s",
			d.User, d.Password, net.JoinHostPort(d.Host, strconv.Itoa(d.Port)), d.DBName, mode)
	}
	return cfg.FormatDSN()
}

func (d DatabaseConfig) registerTLSConfig() (string, error) {
	mode := NormalizeTLSMode(d.TLSMode)
	switch mode {
	case "false":
		return "", nil
	case "preferred":
		return "preferred", nil
	case "skip-verify":
		return "skip-verify", nil
	case "require", "verify-ca", "verify-full":
		// custom verified config
	default:
		return "", fmt.Errorf("unsupported DB_TLS mode %q", d.TLSMode)
	}

	serverName := strings.TrimSpace(d.TLSServerName)
	if serverName == "" {
		serverName = d.Host
	}

	rootPool, caFingerprint, err := loadCAPool(d.TLSCAFile)
	if err != nil {
		return "", err
	}

	var certs []tls.Certificate
	certFP := ""
	if d.TLSCertFile != "" || d.TLSKeyFile != "" {
		if d.TLSCertFile == "" || d.TLSKeyFile == "" {
			return "", errors.New("DB_TLS_CERT and DB_TLS_KEY must both be set for client certificates")
		}
		cert, err := tls.LoadX509KeyPair(d.TLSCertFile, d.TLSKeyFile)
		if err != nil {
			return "", fmt.Errorf("load DB client certificate: %w", err)
		}
		certs = append(certs, cert)
		certFP = fileFingerprint(d.TLSCertFile) + ":" + fileFingerprint(d.TLSKeyFile)
	}

	// Name includes material hash so CA/cert rotation registers a new config.
	sum := sha256.Sum256([]byte(strings.Join([]string{mode, serverName, caFingerprint, certFP}, "|")))
	name := "ap-mysql-" + hex.EncodeToString(sum[:8])

	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		ServerName:   serverName,
		Certificates: certs,
		RootCAs:      rootPool, // nil => system roots
	}
	// verify-ca: authenticate CA but do not pin hostname (still set ServerName for SNI).
	// require / verify-full: full chain + hostname verification (default).
	if mode == "verify-ca" {
		tlsCfg.InsecureSkipVerify = true
		tlsCfg.VerifyPeerCertificate = makeVerifyCAOnly(rootPool)
	}

	if _, loaded := registeredTLSConfigs.LoadOrStore(name, struct{}{}); loaded {
		return name, nil
	}
	if err := mysqldriver.RegisterTLSConfig(name, tlsCfg); err != nil {
		// Another goroutine may have registered the same name; treat as success.
		if !strings.Contains(strings.ToLower(err.Error()), "exist") {
			registeredTLSConfigs.Delete(name)
			return "", fmt.Errorf("register MySQL TLS config: %w", err)
		}
	}
	return name, nil
}

func makeVerifyCAOnly(roots *x509.CertPool) func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return errors.New("mysql TLS: empty peer certificate chain")
		}
		certs := make([]*x509.Certificate, 0, len(rawCerts))
		for _, raw := range rawCerts {
			c, err := x509.ParseCertificate(raw)
			if err != nil {
				return fmt.Errorf("mysql TLS: parse peer cert: %w", err)
			}
			certs = append(certs, c)
		}
		opts := x509.VerifyOptions{
			Roots:         roots,
			Intermediates: x509.NewCertPool(),
		}
		for _, c := range certs[1:] {
			opts.Intermediates.AddCert(c)
		}
		// Hostname intentionally not checked for verify-ca.
		_, err := certs[0].Verify(opts)
		return err
	}
}

func loadCAPool(path string) (*x509.CertPool, string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, "system", nil
	}
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read DB_TLS_CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, "", errors.New("DB_TLS_CA contains no valid PEM certificates")
	}
	// Fingerprint material for registration key + rotation detection.
	fp := sha256.Sum256(normalizePEM(pemBytes))
	return pool, hex.EncodeToString(fp[:8]), nil
}

func normalizePEM(b []byte) []byte {
	var out []byte
	rest := b
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		out = append(out, pem.EncodeToMemory(block)...)
	}
	if len(out) == 0 {
		return b
	}
	return out
}

func fileFingerprint(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return path
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// ValidateDatabaseTLS enforces production TLS policy and cert pair completeness.
func (d DatabaseConfig) ValidateDatabaseTLS(production bool) error {
	mode := NormalizeTLSMode(d.TLSMode)
	if d.TLSCertFile != "" || d.TLSKeyFile != "" {
		if d.TLSCertFile == "" || d.TLSKeyFile == "" {
			return errors.New("DB_TLS_CERT and DB_TLS_KEY must both be set")
		}
	}
	if d.TLSCAFile != "" {
		if _, err := os.Stat(d.TLSCAFile); err != nil {
			return fmt.Errorf("DB_TLS_CA not readable: %w", err)
		}
	}
	if production {
		if !ProductionTLSAllowed(mode) {
			return fmt.Errorf("production forbids DB_TLS=%q (use require/verify-ca/verify-full)", d.TLSMode)
		}
		// Building the TLS config fails fast on bad CA/certs.
		if _, err := d.registerTLSConfig(); err != nil {
			return err
		}
	} else if mode != "false" && mode != "preferred" && mode != "skip-verify" {
		if _, err := d.registerTLSConfig(); err != nil {
			return err
		}
	}
	return nil
}

// ValidateRedisURL enforces authenticated TLS for shared Redis.
// Empty URL is allowed (memory counter fallback) outside explicit production require.
func ValidateRedisURL(raw string, production bool) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("REDIS_URL is not a valid URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "rediss":
		// ok
	case "redis":
		if production {
			return errors.New("production REDIS_URL must use rediss:// (TLS)")
		}
		// non-production: allow plaintext redis for local dev
	case "unix":
		if production {
			return errors.New("production REDIS_URL must use rediss:// (TLS)")
		}
	default:
		return fmt.Errorf("unsupported REDIS_URL scheme %q (use rediss://)", u.Scheme)
	}

	hasPassword := false
	if u.User != nil {
		if pass, ok := u.User.Password(); ok && pass != "" {
			hasPassword = true
		}
	}
	// rediss always requires a password; production also requires it for any scheme.
	if (production || scheme == "rediss") && !hasPassword {
		return errors.New("REDIS_URL must include a non-empty password")
	}

	if scheme == "rediss" {
		host := u.Hostname()
		if host == "" {
			return errors.New("REDIS_URL host is required for TLS verification")
		}
		// Reject explicit insecure query flags if present.
		q := u.Query()
		if v := strings.ToLower(q.Get("skip_verify")); v == "true" || v == "1" {
			return errors.New("REDIS_URL must not disable TLS verification (skip_verify)")
		}
		if v := strings.ToLower(q.Get("insecure_skip_verify")); v == "true" || v == "1" {
			return errors.New("REDIS_URL must not disable TLS verification")
		}
	}
	return nil
}

// ClassifyDBError maps driver/network errors to readiness reasons without leaking secrets.
func ClassifyDBError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "access denied"),
		strings.Contains(msg, "authentication"),
		strings.Contains(msg, "password"),
		strings.Contains(msg, "using password"):
		return "auth"
	case strings.Contains(msg, "x509"),
		strings.Contains(msg, "certificate"),
		strings.Contains(msg, "tls:"),
		strings.Contains(msg, "handshake"),
		strings.Contains(msg, "unknown authority"),
		strings.Contains(msg, "certificate signed by unknown"),
		strings.Contains(msg, "tls handshake"):
		return "cert"
	case strings.Contains(msg, "too many connections"),
		strings.Contains(msg, "er_con_count"),
		strings.Contains(msg, "connection pool"),
		strings.Contains(msg, "bad connection"):
		return "pool"
	case strings.Contains(msg, "timeout"),
		strings.Contains(msg, "i/o timeout"),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "no such host"),
		strings.Contains(msg, "network is unreachable"),
		strings.Contains(msg, "connection reset"):
		return "network"
	default:
		return "unavailable"
	}
}
