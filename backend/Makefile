.PHONY: run build tidy db-up db-down db-logs test test-integration test-integration-ci test-cover migrate-up migrate-status

# 本地启动后端(需先起 MySQL)
run:
	go run ./cmd

# 编译 API + migrate CLI
build:
	go build -o bin/server ./cmd
	go build -o bin/migrate ./cmd/migrate

# 拉取/整理依赖
tidy:
	go mod tidy

# 用 Docker 起 MySQL 开发服务器(数据持久化于 volume)
db-up:
	docker compose up -d

# 停止并移除 MySQL 容器(保留 volume 数据)
db-down:
	docker compose down

# 查看 MySQL 日志
db-logs:
	docker compose logs -f mysql

# 单元测试(不含集成,无需 MySQL)
test:
	go test ./...

# 集成测试(需先 make db-up 起 MySQL)
test-integration:
	go test -tags=integration ./...

# Schema 迁移 CLI（需 DB 可达）
migrate-up:
	go run ./cmd/migrate up

migrate-status:
	go run ./cmd/migrate status

# CI 强制 MySQL 集成（不可达则 fail）
test-integration-ci:
	RUN_MYSQL_TESTS=1 go test -tags=integration ./...

# 覆盖率报告
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
