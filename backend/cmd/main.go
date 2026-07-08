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
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/routes"

	"github.com/joho/godotenv"
)

func main() {
	// 加载 .env(若存在); 不存在则使用 OS 环境变量
	if err := godotenv.Load(); err != nil {
		slog.Warn("未找到 .env 文件, 使用 OS 环境变量", "err", err)
	}

	cfg := config.Load()
	config.SetupLogger(cfg.LogLevel)

	// 初始化数据库(开发期若 MySQL 未就绪, 仅告警不阻断启动)
	db, err := repo.InitDB(cfg.Database)
	if err != nil {
		slog.Error("数据库连接失败, 服务以降级模式启动(部分接口将不可用)", "err", err)
	} else {
		slog.Info("数据库连接成功")
		if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
			defer sqlDB.Close()
		}
	}

	r := routes.NewRouter(cfg, db)

	srv := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: r,
	}

	go func() {
		slog.Info("服务启动", "addr", cfg.ServerAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("服务异常退出", "err", err)
			os.Exit(1)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("收到关闭信号, 正在关闭...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("关闭失败", "err", err)
	}
	slog.Info("服务已关闭")
}
