package config

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDSN(t *testing.T) {
	tests := []struct {
		name string
		cfg  DatabaseConfig
		want string
	}{
		{
			name: "standard",
			cfg:  DatabaseConfig{Host: "127.0.0.1", Port: 3306, User: "u", Password: "p", DBName: "db"},
			want: "u:p@tcp(127.0.0.1:3306)/db?charset=utf8mb4&parseTime=True&loc=Local",
		},
		{
			name: "custom",
			cfg:  DatabaseConfig{Host: "db.host", Port: 3307, User: "root", Password: "secret", DBName: "prod"},
			want: "root:secret@tcp(db.host:3307)/prod?charset=utf8mb4&parseTime=True&loc=Local",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.DSN())
		})
	}
}

// 用例需要环境变量为空以验证默认值; t.Setenv 无法 unset, 故手动保存/清除/恢复。
func TestLoad_Defaults(t *testing.T) {
	keys := []string{
		"SERVER_ADDR", "LOG_LEVEL", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"TENCENT_MAP_KEY", "CAIYUN_WEATHER_KEY", "VLM_ENDPOINT", "VLM_KEY", "LLM_ENDPOINT", "LLM_KEY", "LLM_MODEL",
	}
	saved := map[string]string{}
	for _, k := range keys {
		v, ok := os.LookupEnv(k)
		if ok {
			saved[k] = v
		}
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for k, v := range saved {
			os.Setenv(k, v)
		}
	})

	cfg := Load()
	assert.Equal(t, ":8080", cfg.ServerAddr)
	assert.Equal(t, "INFO", cfg.LogLevel)
	assert.Equal(t, "127.0.0.1", cfg.Database.Host)
	assert.Equal(t, 3306, cfg.Database.Port)
	assert.Equal(t, "animal_poke", cfg.Database.User)
	assert.Equal(t, "animal_poke", cfg.Database.Password)
	assert.Equal(t, "animal_poke", cfg.Database.DBName)
	assert.Equal(t, "", cfg.ThirdParty.TencentMapKey)
	assert.Equal(t, "", cfg.ThirdParty.VLMKey)
	assert.Equal(t, "", cfg.ThirdParty.LLMModel)
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("SERVER_ADDR", ":9999")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "13306")
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "pw")
	t.Setenv("DB_NAME", "prod")
	t.Setenv("TENCENT_MAP_KEY", "tk")
	t.Setenv("LLM_MODEL", "qwen3.6-flash")

	cfg := Load()
	assert.Equal(t, ":9999", cfg.ServerAddr)
	assert.Equal(t, "DEBUG", cfg.LogLevel)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 13306, cfg.Database.Port)
	assert.Equal(t, "root", cfg.Database.User)
	assert.Equal(t, "pw", cfg.Database.Password)
	assert.Equal(t, "prod", cfg.Database.DBName)
	assert.Equal(t, "tk", cfg.ThirdParty.TencentMapKey)
	assert.Equal(t, "qwen3.6-flash", cfg.ThirdParty.LLMModel)
}

func TestGetEnv_Default(t *testing.T) {
	os.Unsetenv("MY_MISSING_KEY")
	assert.Equal(t, "fallback", getEnv("MY_MISSING_KEY", "fallback"))

	t.Setenv("MY_PRESENT_KEY", "val")
	assert.Equal(t, "val", getEnv("MY_PRESENT_KEY", "fallback"))

	// 空字符串视为未设置(getEnv 检查 v != "")
	t.Setenv("MY_EMPTY_KEY", "")
	assert.Equal(t, "fallback", getEnv("MY_EMPTY_KEY", "fallback"))
}

func TestGetEnvInt_Fallback(t *testing.T) {
	t.Setenv("MY_INT_BAD", "not-a-number")
	assert.Equal(t, 42, getEnvInt("MY_INT_BAD", 42))

	t.Setenv("MY_INT_GOOD", "7")
	assert.Equal(t, 7, getEnvInt("MY_INT_GOOD", 42))

	os.Unsetenv("MY_INT_MISSING")
	assert.Equal(t, 42, getEnvInt("MY_INT_MISSING", 42))

	// 空字符串视为未设置
	t.Setenv("MY_INT_EMPTY", "")
	assert.Equal(t, 42, getEnvInt("MY_INT_EMPTY", 42))
}

func TestSetupLogger_NoPanic(t *testing.T) {
	for _, lvl := range []string{"DEBUG", "INFO", "WARN", "WARNING", "ERROR", "UNKNOWN"} {
		assert.NotPanics(t, func() { SetupLogger(lvl) }, "level=%s", lvl)
	}
}

func TestSetupLogger_LevelFilter(t *testing.T) {
	oldDefault := slog.Default()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		slog.SetDefault(oldDefault)
	})

	SetupLogger("ERROR")
	slog.Info("info_should_be_filtered")
	slog.Error("error_should_appear")

	_ = w.Close()
	out, _ := io.ReadAll(r)
	s := string(out)

	assert.Contains(t, s, "error_should_appear")
	assert.False(t, strings.Contains(s, "info_should_be_filtered"),
		"INFO 级别在 ERROR 阈值下应被过滤")
}
