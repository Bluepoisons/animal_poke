# animal_poke 后端服务(Go)

所有联网服务的总枢纽(Gin + Gorm + godotenv + MySQL)。客户端只与本后端通信,
任何第三方 Key(腾讯地图/彩云/VLM/LLM)只存于本工程 `.env`, 客户端永不含第三方 Key。

## 目录结构

```
backend/
├── cmd/main.go              # 入口: 加载配置 -> 连 DB -> 拉起 Gin
├── docker-compose.yml       # 本地 MySQL 开发服务器
├── Makefile                 # 常用命令
├── go.mod
└── internal/
    ├── config/              # 配置读取层(OS 环境变量 > .env > 默认), 含第三方 Key
    ├── middleware/           # Logger / Recovery / CORS
    ├── routes/              # 路由装配
    ├── handlers/            # 请求处理(当前: /health)
    ├── services/            # 联网服务骨架(MB2 地图/天气, MB3 VLM/LLM, 占位)
    └── repo/                # 数据访问(GORM MySQL 连接)
```

## 快速开始

```bash
# 1. 起 MySQL(本地 Docker 开发服务器)
make db-up

# 2. 复制并填写配置(把第三方 Key 填入 .env)
cp .env.example .env

# 3. 启动后端
make run            # 或: go run ./cmd
```

健康检查: `curl http://127.0.0.1:8080/health` 应返回 `{"status":"ok",...}`。

## 配置字段(见 .env.example)

- 服务: `SERVER_ADDR`、`LOG_LEVEL`
- MySQL: `DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME`
- 第三方: `TENCENT_MAP_KEY` / `CAIYUN_WEATHER_KEY` / `VLM_ENDPOINT` / `VLM_KEY` / `LLM_ENDPOINT` / `LLM_KEY`

## 中间件链

`Logger -> Recovery -> CORS`, 在 `internal/routes/router.go` 装配。

## 后续任务

- MB1 设备鉴权 + API 网关(`/auth/device` 签发 Token, 中间件校验)
- MB2 第三方代理(腾讯地图 / 彩云天气)
- MB3 AI 编排(VLM 检测 / 深度分析 / LLM 数值生成)
- MB4 同步 + 反作弊审计
- MB5 对象存储(玩家分享, 可选)
