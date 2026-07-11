package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMySQLConfig_SpecialCharacterPasswords(t *testing.T) {
	passwords := []string{
		`p@ss:w/rd`,
		`a:b@c/d?e%f`,
		`p@ss word!`,
		`中文密码`,
		`%40%3A`,
		`foo'bar"baz`,
		`a\b\n\t`,
	}
	for _, pass := range passwords {
		t.Run(pass, func(t *testing.T) {
			cfg := DatabaseConfig{
				Host:     "db.example",
				Port:     3306,
				User:     "root",
				Password: pass,
				DBName:   "prod",
				TLSMode:  "false",
			}
			mc, err := cfg.MySQLConfig()
			require.NoError(t, err)
			assert.Equal(t, pass, mc.Passwd, "password must survive mysql.Config without URL mangling")
			assert.Equal(t, "root", mc.User)
			assert.Equal(t, "db.example:3306", mc.Addr)
			assert.Equal(t, "prod", mc.DBName)
			// FormatDSN is available; open path uses NewConnector so re-parse is not required.
			dsn := mc.FormatDSN()
			assert.NotEmpty(t, dsn)
			assert.Contains(t, dsn, "charset=utf8mb4")
			assert.Contains(t, dsn, "parseTime=true")
		})
	}
}

func TestMySQLConfig_IPv6HostPort(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "::1",
		Port:     3306,
		User:     "u",
		Password: "p",
		DBName:   "db",
		TLSMode:  "false",
	}
	mc, err := cfg.MySQLConfig()
	require.NoError(t, err)
	assert.Equal(t, "[::1]:3306", mc.Addr)
}

func TestMySQLConfig_RequireTLSRegistersCustom(t *testing.T) {
	cfg := DatabaseConfig{
		Host:          "mysql.production.svc.cluster.local",
		Port:          3306,
		User:          "u",
		Password:      "p",
		DBName:        "db",
		TLSMode:       "require",
		TLSServerName: "mysql.production.svc.cluster.local",
	}
	mc, err := cfg.MySQLConfig()
	require.NoError(t, err)
	assert.NotEmpty(t, mc.TLSConfig)
	assert.True(t, len(mc.TLSConfig) > 0)
	assert.NotEqual(t, "false", mc.TLSConfig)
	assert.NotEqual(t, "skip-verify", mc.TLSConfig)
}

func TestValidateDatabaseTLS_ProductionRejectsDowngrade(t *testing.T) {
	for _, mode := range []string{"false", "skip-verify", "preferred", "off"} {
		cfg := DatabaseConfig{Host: "h", Port: 3306, DBName: "db", TLSMode: mode}
		err := cfg.ValidateDatabaseTLS(true)
		require.Error(t, err, mode)
		assert.Contains(t, err.Error(), "production forbids DB_TLS")
	}
	cfg := DatabaseConfig{Host: "h", Port: 3306, DBName: "db", TLSMode: "require"}
	assert.NoError(t, cfg.ValidateDatabaseTLS(true))
}

func TestValidateDatabaseTLS_CARotationChangesTLSName(t *testing.T) {
	dir := t.TempDir()
	ca1 := writeTestCA(t, dir, "ca1")
	ca2 := writeTestCA(t, dir, "ca2")

	base := DatabaseConfig{
		Host:          "db.example",
		Port:          3306,
		User:          "u",
		Password:      "p",
		DBName:        "db",
		TLSMode:       "verify-full",
		TLSServerName: "db.example",
	}
	a := base
	a.TLSCAFile = ca1
	b := base
	b.TLSCAFile = ca2

	ma, err := a.MySQLConfig()
	require.NoError(t, err)
	mb, err := b.MySQLConfig()
	require.NoError(t, err)
	assert.NotEqual(t, ma.TLSConfig, mb.TLSConfig, "CA rotation must register a distinct TLS config name")
}

func TestValidateRedisURL(t *testing.T) {
	assert.NoError(t, ValidateRedisURL("", true))
	assert.NoError(t, ValidateRedisURL("redis://127.0.0.1:6379/0", false))
	assert.Error(t, ValidateRedisURL("redis://127.0.0.1:6379/0", true))
	assert.Error(t, ValidateRedisURL("rediss://redis.example:6379/0", true), "missing password")
	assert.Error(t, ValidateRedisURL("rediss://:secret@/0", true), "missing host")
	assert.NoError(t, ValidateRedisURL("rediss://:s3cret@redis.example:6379/0", true))
	assert.NoError(t, ValidateRedisURL("rediss://default:s3cret@redis.example:6379/1", false))
	assert.Error(t, ValidateRedisURL("rediss://:s3cret@redis.example:6379/0?skip_verify=true", true))
}

func TestClassifyDBError(t *testing.T) {
	assert.Equal(t, "auth", ClassifyDBError(errors.New("Error 1045: Access denied for user")))
	assert.Equal(t, "cert", ClassifyDBError(errors.New("tls: failed to verify certificate: x509: certificate signed by unknown authority")))
	assert.Equal(t, "pool", ClassifyDBError(errors.New("Error 1040: Too many connections")))
	assert.Equal(t, "network", ClassifyDBError(errors.New("dial tcp: i/o timeout")))
	assert.Equal(t, "unavailable", ClassifyDBError(errors.New("something else")))
	assert.Equal(t, "", ClassifyDBError(nil))
}

func TestNormalizeTLSMode(t *testing.T) {
	assert.Equal(t, "false", NormalizeTLSMode(""))
	assert.Equal(t, "false", NormalizeTLSMode("off"))
	assert.Equal(t, "require", NormalizeTLSMode("true"))
	assert.Equal(t, "preferred", NormalizeTLSMode("prefer"))
	assert.Equal(t, "verify-full", NormalizeTLSMode("verify_full"))
}

func writeTestCA(t *testing.T, dir, name string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	path := filepath.Join(dir, name+".pem")
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}))
	require.NoError(t, f.Close())
	return path
}
