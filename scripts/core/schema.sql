-- ============================================================
-- 本地数据库 schema(F4)
-- SQLite 方言。db.gd 在 open() 时按分号拆分并执行, 幂等(CREATE TABLE IF NOT EXISTS)。
-- 加密: 启用 SQLCipher 的 godot-sqlite 构建下, 打开时设置 encryption_key 静态加密。
--
-- 表: animals(动物元数据) / player_progress(玩家进度)
--      inventory(背包) / checkin(签到)
-- 同步字段见 4.4: UUID / 稀有度 / 属性 / 品种 / 物种 / 坐标 / 生成时间 / 推理请求ID
-- ============================================================

-- 动物元数据表
CREATE TABLE IF NOT EXISTS animals (
    uuid                TEXT    PRIMARY KEY,                 -- 唯一 ID(捕获时生成)
    species             TEXT    NOT NULL DEFAULT '',          -- 物种(如 cat)
    breed               TEXT    NOT NULL DEFAULT '',          -- 品种
    rarity              INTEGER NOT NULL DEFAULT 0,           -- 稀有度(0-4 对应 灰/绿/蓝/紫/金)
    -- 六维属性(5.2): HP / ATK / DEF / SPD / 职业 / 元素
    attr_hp             INTEGER NOT NULL DEFAULT 0,
    attr_atk            INTEGER NOT NULL DEFAULT 0,
    attr_def            INTEGER NOT NULL DEFAULT 0,
    attr_spd            INTEGER NOT NULL DEFAULT 0,
    attr_class          TEXT    NOT NULL DEFAULT '',
    attr_element        TEXT    NOT NULL DEFAULT '',
    lat                 REAL    NOT NULL DEFAULT 0,           -- 捕获坐标纬度(可选脱敏)
    lng                 REAL    NOT NULL DEFAULT 0,           -- 捕获坐标经度(可选脱敏)
    generated_at        TEXT    NOT NULL DEFAULT '',          -- 生成时间(ISO8601)
    inference_request_id TEXT   NOT NULL DEFAULT '',          -- 云端推理请求 ID(可追溯, 4.5)
    model_version       TEXT    NOT NULL DEFAULT '',          -- 推理模型版本
    capture_method      TEXT    NOT NULL DEFAULT '',          -- 捕获方式(throw/...)
    created_at          TEXT    NOT NULL DEFAULT '',
    updated_at          TEXT    NOT NULL DEFAULT ''
);

-- 玩家进度表(单行, id 固定为 1)
CREATE TABLE IF NOT EXISTS player_progress (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    player_id           TEXT    NOT NULL DEFAULT '',
    device_id           TEXT    NOT NULL DEFAULT '',
    level               INTEGER NOT NULL DEFAULT 1,
    exp                 INTEGER NOT NULL DEFAULT 0,
    coins               INTEGER NOT NULL DEFAULT 0,           -- 金币(软货币)
    stamina             INTEGER NOT NULL DEFAULT 120,         -- 体力(上限随等级, 3.4)
    stamina_updated_at  TEXT    NOT NULL DEFAULT '',          -- 体力自然恢复基准时间
    total_captured      INTEGER NOT NULL DEFAULT 0,
    updated_at          TEXT    NOT NULL DEFAULT ''
);

-- 背包表
CREATE TABLE IF NOT EXISTS inventory (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    item_type           TEXT    NOT NULL DEFAULT '',          -- 类型(throw_ball/food/stamina_potion...)
    item_id             TEXT    NOT NULL DEFAULT '',
    quantity            INTEGER NOT NULL DEFAULT 0,
    updated_at          TEXT    NOT NULL DEFAULT ''
);

-- 签到表
CREATE TABLE IF NOT EXISTS checkin (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    checkin_date        TEXT    NOT NULL DEFAULT '',          -- 签到日期(YYYY-MM-DD)
    streak              INTEGER NOT NULL DEFAULT 0,           -- 连续签到天数
    reward_coins        INTEGER NOT NULL DEFAULT 0,
    reward_item_type    TEXT    NOT NULL DEFAULT '',
    reward_item_id      TEXT    NOT NULL DEFAULT '',
    created_at          TEXT    NOT NULL DEFAULT ''
);
