package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/migrate"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/routes"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("未找到 .env 文件, 使用 OS 环境变量", "err", err)
	}

	cfg := config.Load()
	config.SetupLogger(cfg.LogLevel)

	if err := cfg.Validate(); err != nil {
		if cfg.IsProduction() {
			slog.Error("配置校验失败, 拒绝启动", "err", err)
			os.Exit(1)
		}
		slog.Warn("配置校验告警", "err", err)
	}

	db, err := repo.InitDB(cfg.Database)
	if err != nil {
		slog.Error("数据库连接失败, 服务以降级模式启动(部分接口将不可用)", "err", err)
	} else {
		slog.Info("数据库连接成功")
		autoMigrate := os.Getenv("AUTO_MIGRATE")
		if autoMigrate != "false" {
			if err := migrate.Apply(db); err != nil {
				slog.Error("数据库迁移失败", "err", err)
				if cfg.IsProduction() {
					os.Exit(1)
				}
			} else {
				slog.Info("数据库迁移完成", "version", migrate.CurrentVersion)
			}
		} else {
			if err := migrate.CheckVersion(db, migrate.CurrentVersion); err != nil {
				slog.Error("schema 版本不匹配", "err", err)
				if cfg.IsProduction() {
					os.Exit(1)
				}
			}
		}
		if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
			defer sqlDB.Close()
		}
	}

	r := routes.NewRouter(cfg, db)

	srv := &http.Server{
		Addr:              cfg.ServerAddr,
		Handler:           r,
		ReadHeaderTimeout: cfg.Server.ReadHeader,
		ReadTimeout:       cfg.Server.Read,
		WriteTimeout:      cfg.Server.Write,
		IdleTimeout:       cfg.Server.Idle,
		MaxHeaderBytes:    cfg.Server.MaxHeader,
	}

	// AP-036: management-only metrics on METRICS_ADDR (default :9090).
	// Not registered on the public Ingress-facing router.
	var metricsSrv *http.Server
	if cfg.MetricsAddr != "" && cfg.MetricsAddr != "off" && cfg.MetricsAddr != "-" {
		metricsSrv = middleware.NewMetricsServer(cfg.MetricsAddr)
		go func() {
			slog.Info("metrics server starting", "addr", cfg.MetricsAddr)
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("metrics server exited", "err", err)
			}
		}()
	}

	go func() {
		slog.Info("服务启动", "addr", cfg.ServerAddr, "app_env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("服务异常退出", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("收到关闭信号, 停止接入新请求...")
	shutdownTimeout := cfg.Server.Shutdown
	if shutdownTimeout <= 0 {
		shutdownTimeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if metricsSrv != nil {
		if err := metricsSrv.Shutdown(ctx); err != nil {
			slog.Error("metrics server shutdown failed", "err", err)
		}
	}
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("关闭失败", "err", err)
	}
	slog.Info("服务已关闭")
}
