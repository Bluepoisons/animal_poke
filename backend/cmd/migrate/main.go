// Command migrate — 生产/预发 schema 迁移 CLI（up | status）。
//
//	migrate up      应用全部待执行迁移（幂等，MySQL 带 GET_LOCK）
//	migrate status  打印已应用 / 待应用版本
//
// 环境变量与后端一致：DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME / DB_TLS 等。
package main

import (
	"fmt"
	"log/slog"
	"os"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/migrate"
	"animalpoke/backend/internal/repo"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		// 生产 Job 通常纯环境变量，无 .env
		_ = err
	}

	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}
	cmd := os.Args[1]
	switch cmd {
	case "up", "status":
	case "-h", "--help", "help":
		printUsage(os.Stdout)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage(os.Stderr)
		os.Exit(2)
	}

	cfg := config.Load()
	config.SetupLogger(cfg.LogLevel)

	db, err := repo.InitDB(cfg.Database)
	if err != nil {
		slog.Error("数据库连接失败", "err", err)
		os.Exit(1)
	}
	if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
		defer sqlDB.Close()
	}

	switch cmd {
	case "up":
		slog.Info("migrate up starting", "target", migrate.CurrentVersion)
		if err := migrate.Apply(db); err != nil {
			slog.Error("migrate up failed", "err", err)
			os.Exit(1)
		}
		slog.Info("migrate up complete", "version", migrate.CurrentVersion)
		// 成功后打印 status，便于 Job 日志审计
		if err := migrate.WriteStatus(os.Stdout, db); err != nil {
			slog.Error("status after up failed", "err", err)
			os.Exit(1)
		}
	case "status":
		if err := migrate.WriteStatus(os.Stdout, db); err != nil {
			slog.Error("migrate status failed", "err", err)
			os.Exit(1)
		}
	}
}

func printUsage(w *os.File) {
	fmt.Fprintf(w, `Usage: migrate <command>

Commands:
  up       Apply pending schema migrations (idempotent)
  status   Show applied / pending migration versions

Environment: same DB_* vars as the API server.
Notes:
  - Production: run as a pre-deploy Job with AUTO_MIGRATE=false on the API.
  - MySQL: uses GET_LOCK(%q, %ds) to serialize concurrent migrate Jobs.
  - Take a backup / confirm PITR before destructive expand→contract steps.
`, "animal_poke_schema_migrate", 120)
}
