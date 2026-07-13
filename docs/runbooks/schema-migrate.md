# Schema 迁移 Job / expand-contract 约定（AP-038）
#
# 相关：
# - CLI: `backend/cmd/migrate` → `animal-poke-migrate up|status`
# - 代码: `backend/internal/migrate`
# - K8s Job: `deploy/k8s/base/migrate-job.yaml`
# - 生产 API: `AUTO_MIGRATE=false`，启动只做 `CheckVersion`

## 何时跑迁移

1. **备份检查**：确认最近全量备份 / PITR 窗口可用（见 `mysql-dr.md`）。
2. **pre-deploy Job**：用即将上线的镜像 tag 跑 `animal-poke-migrate up`。
3. **锁**：MySQL `GET_LOCK('animal_poke_schema_migrate', 120)`，并发 Job 串行。
4. **超时**：Job `activeDeadlineSeconds=600`；锁等待 120s。
5. **幂等**：`schema_migrations` 已有版本跳过；约束/FK 添加前查 `information_schema`。
6. **中断重试**：Job 失败后可安全 `kubectl delete job` 再 apply；已成功版本不会重做。

## expand / contract

破坏性 schema 变更拆两阶段，避免长锁与不可回滚：

| 阶段 | 做什么 | 回滚 |
|------|--------|------|
| **expand** | 加列（可空/有默认）、加索引、加 CHECK/FK（数据已清洗）、双写新路径 | 忽略新列/约束即可回滚应用 |
| **contract** | 删旧列、强制 NOT NULL、去掉兼容代码 | 需确认无旧版流量 |

### 0009_check_constraints 与广谱物种迁移

**Expand only（MySQL 8+）**：

- `animals`: `species` 列保持 `NOT NULL`；可捕获资格、非空 ID、别名和中文名称由版本化物种内容注册表校验；rarity 1–5
- `inferences`: status/kind/species 枚举
- `products` / `orders`: 金额 ≥ 0；状态枚举
- `data_requests` / `audit_logs`: 类型/状态/风险分
- FK：`animals.device_id → devices.device_id`，`orders.product_id → products.product_id`

**上线前数据清洗（contract 前置）**：

```sql
-- 空物种 ID / 非法 rarity 先修或隔离；不要再用静态三物种白名单清洗
SELECT id, uuid, species, rarity FROM animals
 WHERE TRIM(species) = '' OR rarity NOT BETWEEN 1 AND 5;

-- 孤儿 device 引用
SELECT a.device_id FROM animals a
 LEFT JOIN devices d ON d.device_id = a.device_id
 WHERE d.device_id IS NULL;

-- 孤儿 product 引用
SELECT o.order_id, o.product_id FROM orders o
 LEFT JOIN products p ON p.product_id = o.product_id
 WHERE p.product_id IS NULL;
```

清洗完成后再跑 `migrate up`。若 FK 因孤儿失败，错误信息会提示 expand 清洗。

**Contract（后续版本，不在 0007）**：例如收紧 stats 为业务真值域（HP 10–100 等）或删除废弃列。

## 本地

```bash
cd backend
make db-up
go run ./cmd/migrate status
go run ./cmd/migrate up
go run ./cmd/migrate status   # pending 应为空
```

## 验收

- 空库：`up` → `CurrentVersion`，`status` 无 pending
- 重复 `up`：无副作用
- `RUN_MYSQL_TESTS=1` 集成测试：NULL species / rarity=99 / 负金额 / 重复 inference_id 被 DB 拒绝
- 服务测试：未注册或未认证的 species 在进入捕获、发奖和同步前被物种注册表拒绝
