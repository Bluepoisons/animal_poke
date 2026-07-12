# 第一季架构 · 城市手账（AP-116）

> 数据源：`frontend/src/narrative/content/season1/architecture.ts`  
> 主题：**谁有权替一座城市讲故事**  
> 验证：`validateSeasonArchitecture()` 桌面推演（非真机用户测试）

## 章节地图

| 顺序 | ID | 标题 | 预算 | 主题问题 |
|------|-----|------|------|----------|
| 0 | prologue.blank_page | 第一张空白页 | 15–20 min | 矛盾注释由谁改写？ |
| 1 | ch01.alley_echo | 巷口的回声 | 30–45 min | 共享空间谁的记忆上墙？ |
| 2 | ch02.along_river_sleepless | 沿河不眠 | 30–45 min | 灯光/施工/夜间活动如何共存？ |
| 3 | ch03.rain_on_eaves | 雨落屋檐 | 30–45 min | 如何保留不确定？ |
| 4 | ch04.map_blank | 地图上的空白 | 30–45 min | 缺席如何被讲述？ |
| 5 | finale.who_tells_the_city | 把城市交给谁讲 | 30–45 min | 策展权与空白权 |

## 依赖图

```
prologue → ch01 → ch02 → ch03 → ch04 → finale
```

每章选择的 `echoIn` 指向后续章，形成延迟回响链。

## 每章必备槽位

- 开端 / 升级 / 转折 / 余韵 beats  
- 城市探索节点（粗粒度地名）  
- 收藏/观察触发（不依赖稀有动物）  
- 人物冲突  
- ≥1 次价值选择 + 跨章回响  
- main / optional / noAnimal / homeMode / noCamera / badWeather 路径  

## 结局矩阵（与收集率无关）

| 结局 | 倾向选择 |
|------|----------|
| 多声部城市 | multivocal_board + preserve_gaps + multivocal_show |
| 策展者的弧 | managed_board + curated_arc |
| 空白优先 | blank_wall + blank_first + preserve_gaps |

## 内容预算

- 序章 15–20 min  
- 其余每章 30–45 min  
- 主线合计约 165–245 min  

## 未完成的真机验证（明确声明）

- 5–8 名目标玩家复述测试：**未执行**  
- 仅完成数据校验与桌面推演结构门禁  

## 回滚

删除 `frontend/src/narrative/content/season1/` 与本文件即可；无 API / DB 迁移。
