.PHONY: run build tidy db-up db-down db-logs

# 本地启动后端(需先起 MySQL)
run:
	go run ./cmd

# 编译为 bin/server
build:
	go build -o bin/server ./cmd

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
