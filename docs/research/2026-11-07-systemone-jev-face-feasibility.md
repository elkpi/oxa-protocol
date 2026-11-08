# SystemOne Jev face 可行性评估

**调查日期：** 2026-11-07

**状态：** 调研结论 + 共识决策记录。不改 `spec/`，不改实现，不加向量，不提交 git。

结论来自 grilling 访谈，共 12 项决策全部 settle（见附录 A）。协议事实以一手 wire 文档为准；oxa 侧事实以当前 main 的规格正文为准（IR contract 0.2.0）。本文不写字节级映射，不预设版本号。

## 1. 结论

**可行，条件成立。**

Jev 是结构化决策协议（`state + questions → answers`），不是对话协议；接入 oxa 不是既有模式的第四张"对话脸"，而是一次有损语义投影。投影的三个语义锚点（请求序列化、answers 解析、报损边界）已定出确定性约定，且全部落在既有 IR 槽位上——核心锚点是 `Params.ResponseFormat`（spec/01 §3.7.1，since 2.1）。IR contract 不需要 bump。

真正的工程风险只有一个：IR→Jev 方向要**把 LLM 自由文本解析回结构化 `answers`**。oxa 此前所有 face 都是确定性结构搬运，从无"解析"语义。error 与 loss 的边界必须写死在规格里，这是整个立项里评审密度最高的一块。

## 2. 协议事实（wire）

来源见第 9 节。端点 `POST /v1/systemone`，Bearer 认证，UTF-8 JSON（上限 32 MiB / 64 层）。

**请求：**

| 字段 | 形状 | 说明 |
|---|---|---|
| `model` | string | 如 `typesafe-jev-1.13.0`、别名 `jev-latest` |
| `state` | string \| object \| array | 被评估的内容，必填 |
| `questions` | map<ID, question> | 至少 1 个，官方文档称上限 32 |
| `assets` | map<ID, asset> | 实验性图像，需 `TypeSafe-Danger-Zone-Images: true` |

question 三型（每型有 `type`、`instructions`）：

- `noul`：0–1 判定；`criteria` 可选（true/false 描述）。
- `choice`：`criteria` 必填，选项名→描述的 map，1–255 项。
- `score`：`criteria` 必填，有序描述数组，schema 允许 1–10 级。
- 另有实验性 `box`（图像定位，需 Jev 1.14）。

**响应：** `model`、`answers`（键同 questions，每项 `type` 与问题匹配：noul→`noul` 0–1 数值；choice→`choice` + `probabilities` + `confidence`；score→`score` + `legend` + `probabilities` + `confidence`）、`usage`（整数 `input_tokens` / `output_tokens`）。

**能力边界：** 无流式、无工具调用；图像输入仅实验性。错误为 JSON body：`400` 的 `detail` 是 string 或 object，`422` 是 validation 列表（`loc`/`msg`/`type`，有时含 `input`/`ctx`）。

文档明确说该端点不是 OpenAI 兼容协议：不用 `messages`，用 `state` 与 `questions`。

## 3. 方向与链路（共识）

```
agent(Jev 客户端) → jev→IR → IR → responses/chat/messages → openai / anthropic
agent(Jev 客户端) ← IR→jev ← IR ← responses/chat/messages ← openai / anthropic
```

Jev 是**客户端侧**协议（agent 发起 Jev 形状请求），openai/anthropic 是**服务端侧**。jev→IR 与 IR→jev 是一次性的新代码，三条链路复用既有 IR↔face 转换，边际成本主要在向量与 Anthropic 侧差异处理。

随方向反转而消失的问题：上一轮讨论的"chat 请求里 questions 从哪来"不再存在——questions 由 agent 主动提供，信息完整。取而代之的是序列化（§4.1）与解析（§4.2）两个新锚点。

**随事实 settle 的范围项：**

- 非流式。Jev 无流式，spec/20 的事件语法对这张脸没有对应物。
- 无工具。Jev 无工具概念，三脸的工具定义在 jev→IR 方向天然不出现。
- `assets` / `box` 是实验性能力，v1 不做，出现即整项 `unsupported-semantic`。
- `model` 按 modelmap 契约 identity 透传；`jev-latest` 等别名由调用方配置映射，不内置。

## 4. 映射锚点（共识决策）

### 4.1 请求方向 jev→IR

| Jev wire | IR 落点 | 约定 |
|---|---|---|
| `state` | system 消息 | 语境进 system；object/array 序列化为 JSON 文本（无结构损失，JSON 本身是文本） |
| `questions` 的 `instructions`/`criteria` | user 消息 | 判定任务序列化进 user；固定模板，便于向量锁定 |
| 输出契约 | `Params.ResponseFormat`（`json_schema` 变体） | 由 questions 合成投影 schema：question ID→类型（noul→number 0–1、choice→enum、score→number 枚举）；Schema 按 INV-1 verbatim 携带 |
| 输出指令 | system 消息 | 一段固定模板声明"输出严格 JSON、键为 question ID"，与 schema 双通道互备 |

合成 schema 是**投影**而非原文：`criteria` 的语义描述走 prompt 文本通道，schema 只编码类型/枚举/范围。两条通道都是确定性的，这是该 face 能进 oxa 的前提（转换必须可判定，不依赖模型行为）。

### 4.2 响应方向 IR→jev

- 解码器接受**可选的请求侧 `questions` 上下文**参数，`type` 按上下文精确恢复；未带上下文时按值形状推断，数字的 noul/score 歧义记 loss。与流式解码器带构造参数的既有风格一致。
- `probabilities`、`confidence`、`legend` 无 IR 槽：省略，逐项记有序 loss。不编造数值。
- `usage` 直通：Jev `input_tokens`/`output_tokens` 与 IR usage 形状对齐。
- **解析失败的边界（规格必须写死）**：assistant 文本不是合法 JSON、question ID 缺失、类型不匹配——结构 error，不是 loss。
- `noul` 解码约定：0–1 数值原样；boolean 映射 1/0。

### 4.3 IR 策略

不扩 IR contract。questions/criteria 的唯一 IR 落点是 `Params.ResponseFormat.Schema` 与 prompt 文本；`answers` 专属字段全部按 §4.2 报损。仓库惯例明确反对为单脸扩 IR，这里没有例外。

### 4.4 预期 loss 目录

| 项 | 方向 | 类型 |
|---|---|---|
| `answers[].probabilities` / `confidence` / `legend` | IR→jev | 每字段一条有序 loss |
| 数字 type 歧义（noul/score） | IR→jev（未带上下文时） | 推断失败记 loss |
| `assets` / `box` | jev→IR | 整项 `unsupported-semantic`（实验性） |
| `criteria` 语义描述 | jev→IR | 不算 loss：进 prompt 文本通道，信息保留 |
| 三脸 encode 侧的 `ResponseFormat` name/description/strict 差异 | IR→face | 按既有 spec 条目（如 N-AN-12）报损，不新增规则 |

## 5. 范围排除

- HTTP 传输、认证、重试、错误体 face 转换（Jev 的 `detail`/422 形状）——项目契约已排除传输层，错误由调用方处理。
- 流式、工具、`assets`/`box`。
- 五语言同步首发——分波，Go 先行。
- 模型能力/别名内置——modelmap identity，调用方配置。

## 6. 风险

| # | 风险 | 缓解 |
|---|---|---|
| R1 | 模型不守 schema，解析失败率高（Anthropic 侧无 `response_format` 参数名，但 N-AN-12 走 `output_config.format`） | 三脸都能携带 json_schema 意图；strict 变体可用处用足；解析失败=error 的边界写死 |
| R2 | `type` 恢复依赖请求上下文，响应解码 API 形状偏离既有"纯 IR→face"签名 | 可选参数设计；评审点 |
| R3 | `jev-latest` identity 透传到 openai/anthropic 会 400 | 文档明示 modelmap 配置责任；不做内置别名 |
| R4 | choice/score 越界值：保留原值（数据保真，但 wire 不合规）还是 error（wire 合规，但丢数据） | 开放点 O-1，立项时定 |
| R5 | schema 管形状、criteria 在 prompt，两通道可能被上游割裂理解 | prompt 模板锁定 + 向量覆盖双向 |

## 7. 里程碑草案（Go 先行）

1. **spec/17**（systemone/jev 映射文档）+ `spec/00`/`spec/README.md` 的"三脸"契约表述修订为四脸 + 阅读顺序更新。
2. **vectors**：`vectors/systemone/jev/` 下 jev→ir 请求、ir→jev 响应、三链路往返、报损锁定各若干条；`veccheck -write-manifest` 刷新。
3. **go/systemone/jev**：face↔IR 双向（请求构造 + schema 合成；响应解析 + 报损）；`go/internal/deps` spoke 白名单加入该包；模型名处理只经 modelmap。
4. 其余语言按 wave 跟进（TS/Python 先，Rust/C++ 后，沿用 Spec 2.0 rollout 节奏）。

工作量估算：spec 1–2 天、向量 1–2 天、Go 实现 + 测试 2–4 天；Go 先行约一周量级，五语言完整约 2–3 周（不含发布流程）。

## 8. 开放点（立项时定，不阻塞可行性结论）

- **O-1** choice/score 越界值：保留原值 + degraded loss（推荐）vs 结构 error。
- **O-2** face 命名：`systemone/jev`（推荐，与端点/产品名一致）vs `typesafe/jev`。
- **O-3** noul 的合成 schema 形状：number[0,1]（推荐，兼容概率语义）vs boolean。
- **O-4** 解码器上下文参数形态：可选参数（推荐）vs 独立构造函数。

## 9. 来源

**Jev wire：**

- https://jevwiki.ai/wiki/reference/http-api.md （HTTP API 参考，请求/响应 schema、错误体）
- https://docs.digitalocean.com/products/inference/how-to/use-system-one-api/ （完整请求/响应示例；"not OpenAI-compatible"、无流式/多模态的表述）
- https://system-one.dev/en/api （overview；最多 32 questions）
- https://jev.pro/api/ （SDK 与版本化模型 ID）

**oxa 侧事实：**

- `spec/01-intermediate-representation.md` §3.7.1（`ResponseFormat` 三变体，schema verbatim，INV-1）
- `spec/12-mapping-anthropic-messages.md` N-AN-12（Anthropic `output_config.format` ↔ `Params.ResponseFormat`）
- `go/ir/request.go` `Params.ResponseFormat` 字段
- `docs/research/2026-11-05-protocol-gap-followup.md`（单脸不扩 IR 的惯例依据）

## 附录 A：共识决策记录（grilling，2026-11-07）

| # | 决策 | 结论 |
|---|---|---|
| 1 | 目标协议身份 | 一个协议：TypeSafe AI Jev，端点 `/v1/systemone`，产品名 System One |
| 2 | 方向 | agent 讲 Jev 协议 → oxa → openai/anthropic；响应反向 |
| 3 | 流式 | 无（Jev 不支持，settled by fact） |
| 4 | 请求序列化 | state→system；questions→user；合成 schema 进 `Params.ResponseFormat`；固定输出指令模板 |
| 5 | answers type 恢复 | 解码器带可选请求侧 questions 上下文，精确恢复 |
| 6 | probabilities/confidence/legend | 省略 + 逐项有序 loss，不编造 |
| 7 | IR 策略 | 不扩 IR contract；复用 §3.7.1 槽位 |
| 8 | 链路范围 | Responses / Chat / Messages 三条 v1 全做 |
| 9 | 落点 | oxa 仓库内新增第四张 face |
| 10 | 语言 | Go 先行分波，其余语言后续 wave |
| 11 | 交付物 | 本评估报告；spec/ 与实现待立项另动 |
| 12 | 能力排除 | assets/box 实验性不做；无内置模型别名；非流式；无工具 |
