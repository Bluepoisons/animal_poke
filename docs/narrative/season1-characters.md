# 第一季角色群像（AP-117）

数据源：`frontend/src/narrative/content/characters/cast.ts`

## 常驻角色

| ID | 名称 | 角色 | 虚构 |
|----|------|------|------|
| archivist | 林溯 | 社区档案员 | 否 |
| street_photographer | 阿柯 | 街拍者 | 否 |
| urban_planner | 周衡 | 城市规划研究者 | 否 |
| journal_aide | 页页 | 手账助手 | **是** |

动物只作为城市关系线索，不担任会说话的科普 NPC。

## 关系网

- 林溯 ↔ 阿柯：方法与现场感
- 林溯 ↔ 周衡：证据标准与公共责任
- 阿柯 ↔ 周衡：什么算证据 / 夜间安全
- 页页 连接三人：标注、伦理、可解释性（非作者嘴替）

## 三阶段弧

每位角色跨 ≥3 章，分 stage 1/2/3；详见数据文件 `arcs`。

## 知识状态

`knowledgeGates` 禁止角色在章节完成前知道终章展览等未发生事件。

## 测试

- `validateCast()` 结构门禁  
- 语气盲测卡片 `voiceBlindCards()`  
- **真人盲测 / 三语言一致性：未执行**（需后续内容 QA）

## 回滚

删除 `characters/` 包与本文件。
