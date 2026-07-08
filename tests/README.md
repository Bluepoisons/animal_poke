# 测试说明

Godot 客户端用 [GdUnit4](https://github.com/MikeSchulze/gdUnit4),Go 后端用 `testify` + 标准库 `testing`。

## 目录结构

```
tests/
  unit/           # 单元测试(纯逻辑/状态机/文件系统/SceneTree)
    rarity.test.gd
    config_loader.test.gd
    db_logic.test.gd
    animal_repository.test.gd
    network_manager.test.gd
    game_manager.test.gd
    config_manager.test.gd
    logger.test.gd
    scene_manager.test.gd
    audio_manager.test.gd
    ui/
      rarity_border.test.gd
      base_panel.test.gd
      loading_indicator.test.gd
      toast.test.gd
      base_button.test.gd
  integration/    # 集成测试(autoload 联动/DB CRUD/日志落盘/持久化)
    autoload_boot.test.gd
    config_logger_integration.test.gd
    save_db_integration.test.gd
    animal_persistence.test.gd
```

## Godot 测试

### 运行

- **编辑器内**: 菜单 `GdUnit4 → Run Test Suites`,或右键 `tests/` 目录 → `Run GdUnit4 Tests`。
- **CLI**(headless,需 Godot 可执行文件):
  ```sh
  godot --headless -s res://addons/gdUnit4/bin/GdUnitCmdTool.gd -a tests/ --ignoreHeadlessMode
  ```
  `--ignoreHeadlessMode` 必加(UI 交互类测试在纯 headless 下 InputEvent 不传递)。

### 依赖

- **GdUnit4 v5.0.4**(`addons/gdUnit4/`):已启用。
- **godot-sqlite v4.7**(`addons/godot-sqlite/`):已启用。`save_db_integration` 与 `db_logic` 的 open/close 测试依赖它;未加载时自动跳过 CRUD。

### ⚠️ GdUnit4 与 Godot 4.7 兼容性(本地补丁)

GdUnit4 v5.0.4 官方仅支持到 Godot 4.6,在 Godot 4.7 上有两处硬解析错误,已本地补丁(待上游发布 4.7 支持后可移除):

1. `addons/gdUnit4/src/core/GdUnitFileAccess.gd:191` — `get_as_text(true)` → `get_as_text()`(4.7 移除了 `skip_cr` 参数)。
2. `addons/gdUnit4/src/doubler/CallableDoubler.gd` — 注释掉 `func call(...)` 覆写(4.7 把原生变参方法覆写不匹配从警告升级为硬解析错误;该函数原为 `assert(false)` 占位,从不执行,移除不影响功能)。

升级 GdUnit4 后,若上游已修复,可恢复这两处。

## Go 后端测试

见 `backend/README.md`。要点:
- 单元测试:`cd backend && make test`(或 `go test ./...`)。
- 集成测试(需 MySQL):`make db-up && make test-integration`。
- 集成测试用 `//go:build integration` 标签,默认 `go test ./...` 不跑;MySQL 不可达时自动 `t.Skip`。

## 测试结果

- Godot: **102 用例, 0 失败**(19 个测试套件)。
- Go: 单元测试全通过;集成测试在 `make db-up` 起 MySQL 后通过。
