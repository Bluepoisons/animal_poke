# 待补充信息清单（填完后可一键回填）

> 请复制下面的 **「填写区」** 整段回复给我（可只填你已确定的项，未知写 `暂无` 或 `待定`）。  
> 收到后我会一次性写回：`docs/legal/*` 中所有占位字段，并更新 README 如有需要。

**当前状态**：协议中仍有「运营主体 / 邮箱 / 地址 / 部分第三方合同主体」等占位；SDK 实装层已根据代码写入 [`sdk-and-services.md`](./sdk-and-services.md)。

---

## 填写区（复制从这里开始）

```yaml
# ============================================================
# Animal Poke 法律文本 — 运营信息回填表 v1.1
# 填好后整段发回即可
# ============================================================

# ---------- A. 运营主体（必填，上线必需）----------
operator:
  company_full_name: ""          # 例：上海某某科技有限公司
  company_short_name: ""         # 例：某某科技（文中可称「我们」）
  uscc: ""                       # 统一社会信用代码
  registered_address: ""         # 注册地址
  mailing_address: ""            # 邮寄/通讯地址（可与注册相同）
  website: ""                    # 官网（可选）
  app_name: "Animal Poke"        # 对外产品名
  app_name_cn: "动物宝可"         # 中文名（可改）

# ---------- B. 联系方式（必填）----------
contacts:
  privacy_email: ""              # 个人信息保护 / 隐私 例：privacy@yourco.com
  support_email: ""              # 用户服务 例：support@yourco.com
  security_email: ""             # 安全漏洞（可与 support 相同）
  phone: ""                      # 客服电话（可选，没有写 暂无）
  privacy_officer_name: ""       # 个人信息保护负责人姓名（可选，可只写职务）
  privacy_officer_title: "个人信息保护负责人"

# ---------- C. 管辖与协议生效（建议填）----------
legal:
  governing_court: ""            # 例：上海市浦东新区人民法院 / 公司住所地有管辖权的人民法院
  effective_date: "2026-07-10"   # 协议生效日
  policy_version: "1.0.0"
  service_region: "中华人民共和国大陆地区"

# ---------- D. 资质与备案（没有就写 暂无/申请中）----------
licenses:
  icp_beian: ""                  # ICP 备案号 例：沪ICP备xxxxxxxx号
  icp_license: ""                # ICP 经营许可证（若有）
  game_isbn_or_approval: ""      # 游戏版号 / 审批文号（若有）
  network_culture_license: ""    # 网络文化经营许可证（若有）
  police_beian: ""               # 公安联网备案（若有）
  other_notes: ""

# ---------- E. 第三方服务 — 合同主体与隐私链接 ----------
# 代码已确定服务类型；请补「签约公司全称 + 隐私政策 URL」
third_parties:
  tencent_map:
    enabled: true
    legal_name: ""               # 例：腾讯科技（深圳）有限公司 / 以合同为准
    product_name: "腾讯位置服务（逆地理编码）"
    privacy_url: ""              # 腾讯位置服务隐私政策链接
    data_shared: "经纬度（可粗化）"
    purpose: "逆地理解析省市区"
    notes: ""

  caiyun_weather:
    enabled: true
    legal_name: ""               # 彩云科技合同主体全称
    product_name: "彩云天气 API"
    privacy_url: ""
    data_shared: "经纬度"
    purpose: "天气预报与玩法修正"
    notes: ""

  vision_vlm:
    enabled: true
    legal_name: ""               # 例：阿里云计算有限公司 / 其他
    product_name: ""             # 例：通义千问-VL / 混元 / 自建网关
    privacy_url: ""
    endpoint_note: ""            # 可选：公开文档里的服务名，不要填 Key
    model_name: ""               # 例：qwen-vl-plus（可写「配置化，以线上为准」）
    data_shared: "图像二进制（瞬时）"
    purpose: "动物检测与分析"
    destroy_after_inference: true
    notes: ""

  text_llm:
    enabled: true
    legal_name: ""
    product_name: ""             # 例：通义千问 / 其他 OpenAI 兼容服务
    privacy_url: ""
    model_name: ""
    data_shared: "结构化文本/分析结果"
    purpose: "属性与叙事生成"
    notes: ""

  cloud_hosting:
    enabled: true
    legal_name: ""               # 例：华为云 / 阿里云 / 腾讯云 签约主体
    product_name: ""             # 例：弹性云服务器 + 云数据库 MySQL
    privacy_url: ""
    region: ""                   # 例：中国大陆-上海
    data_shared: "账号进度、日志等业务数据"
    purpose: "应用托管与数据库"
    notes: ""

  redis:
    enabled: false               # 若生产用云 Redis 改为 true
    legal_name: ""
    product_name: ""
    privacy_url: ""
    notes: "未配置时使用进程内限流"

  object_storage:
    enabled: false
    legal_name: ""
    product_name: ""
    privacy_url: ""
    notes: "用户主动分享图时才需要"

  push:
    enabled: false
    legal_name: ""
    product_name: ""
    privacy_url: ""

  analytics:
    enabled: false
    legal_name: ""
    product_name: ""
    privacy_url: ""

  crash_reporting:
    enabled: false
    legal_name: ""
    product_name: ""
    privacy_url: ""

  login_oauth:
    enabled: false               # 微信/苹果登录等
    legal_name: ""
    product_name: ""
    privacy_url: ""

  payment:
    enabled: false
    legal_name: ""
    product_name: ""
    privacy_url: ""

# ---------- F. 数据处理补充（建议）----------
data_processing:
  storage_location: "中华人民共和国境内"   # 若有境外写明
  cross_border: false                      # 是否跨境；true 则需补充接收方
  cross_border_receiver: ""
  cross_border_purpose: ""
  log_retention_days: 180                  # 日志保留天数（可改）
  account_inactive_months: 36              # 不活跃后删除/匿名化（可改）
  response_sla_workdays: 15                # 行权答复工作日

# ---------- G. 未成年人 / 防沉迷（建议）----------
minors:
  real_name_system: "未接入"               # 或：已接入国家实名认证系统 / 计划接入
  anti_addiction_note: ""                  # 补充说明
  age_gate_copy_ok: true                   # 是否沿用当前模板文案

# ---------- H. 产品链接（可选，便于协议内跳转）----------
links:
  privacy_policy_url: ""         # 正式托管后的隐私政策 URL
  terms_url: ""
  minors_url: ""
  sdk_list_url: ""
  official_site: ""
  app_store_or_download: ""

# ---------- I. 其他你想写进协议的内容 ----------
extra_notes: |
  （自由填写）
```

---

## 填写区结束

## 字段说明（对照表）

| 区块 | 会写进哪些文件 | 不填的影响 |
|------|----------------|------------|
| A 运营主体 | 隐私政策、用户协议页眉 | 无法作为正式上线文本 |
| B 联系方式 | 所有协议行权 / 客服段落 | 用户无法行权联系 |
| C 管辖法院 | 用户协议争议条款 | 保持「住所地法院」笼统表述 |
| D 资质 | 隐私政策 / README 可选展示 | 标注「暂无」 |
| E 第三方 | 第三方清单、SDK 清单、隐私政策共享章 | 仅保留服务类型，无合同主体名 |
| F 数据 | 存储期限、跨境 | 用模板默认值 |
| G 未成年人 | 未成年人规则 | 保持基线表述 |
| H 链接 | 协议互链、应用内跳转 | 继续用仓库相对路径 |

## 已根据代码写死、你一般不用改的内容

- 客户端 **无** 地图/天气/AI SDK，仅 **React / ReactDOM / idb** + 系统相机定位  
- 服务端固定对接类型：**腾讯地图逆地理、彩云天气、可配置 VLM/LLM、MySQL、可选 Redis**  
- 照片：**后端转发 VLM 后即时销毁**  
- 密钥：仅 `backend/.env`  

## 回填方式

1. 填好上方 YAML「填写区」  
2. 发回对话  
3. 我会执行一键替换并 commit / push（如你需要）
