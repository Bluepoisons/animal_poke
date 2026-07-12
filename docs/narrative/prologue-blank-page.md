# 序章垂直切片 · 第一张空白页（AP-124）

数据：`frontend/src/narrative/content/prologue/blankPage.ts`  
游玩入口：手账 →「开始序章」→ `PrologueScreen` + `PresentationPlayer`

## 节奏表（设计预算 15–20 min）

| 节拍 | 分钟 | sequence |
|------|------|----------|
| 训练观察 | 5 | prologue.train |
| 矛盾注释 | 5 | prologue.contradiction |
| 价值选择 | 5 | prologue.choice |
| 悬念便签 | 3 | prologue.hook |

## 验收映射

- 介绍 ≥3 角色：档案员 / 街拍者 / 手账助手  
- 一次收藏触发：训练观察写入手账线索  
- 一次无标准答案选择：矛盾注释处理  
- 延迟回响预告 + 跨章悬念：开启《巷口的回声》  
- 拒绝相机/定位：全程训练帧 / Home Mode  

## 未执行

- 8–12 人可用性测试、三语言真机：未做（仅结构与单元验证）

## 回滚

删除 prologue 包、`PrologueScreen` 与路由分支。
