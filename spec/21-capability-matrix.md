# 21 — Control-Plane Fidelity Matrix（控制面保真度矩阵）

Status: normative (since 3.2.0). 扩展 [02](02-loss-policy.md)、[10](10-mapping-openai-chat-completions.md)、[11](11-mapping-openai-responses.md)、[12](12-mapping-anthropic-messages.md)、[20](20-streaming-semantics.md)；不引入新 IR 结构、新 loss reason 或新转换行为。

## 1. 目的与范围

一部分调用方把同一组协议面用于「控制面」流量：请求携带特权指令、结构化输出契约、工具选择与输出预算，客户端自行解析响应流并据此执行策略。控制面请求在协议层面**就是普通请求**——oxa 不设 control-plane 专用机制、不识别客户端、不参与路由或授权决策。

本文档的职责是把已有的、分散在 spec/10、11、12、15、20 中的映射规则，对一组 vendor-neutral 的控制面语义给出**一份权威的 3×3 跨面定级矩阵**、一条流生命周期契约摘要、以及 loss 契约的稳定性承诺，使网关可以在不解析自由文本 detail 的前提下做出确定性的放行/降级/拒绝决策。

**定级基准（normative）**：矩阵按**规范期望**定级——每格的等级与 loss 锚点由 spec/10、11、12、15、20 的映射规则决定。实现行为与矩阵不符属于实现缺陷，MUST 以修实现或修向量的方式消除，MUST NOT 通过改矩阵迁就实现现状。定级的语义细节一律回指各映射文档；本文档不复制它们的正文。

矩阵覆盖 Chat Completions（CC）、Responses（RE）、Anthropic Messages（AN）三个对话面。Jev 面（[17](17-systemone-jev.md)）是评估协议投影，不在本矩阵范围内。非流响应矩阵与流生命周期契约同样适用于 Jev 的适用部分，见 spec/17。

**非目标**：识别任何客户端或产品、检查提示词或会话标识、选择模型或提供方、实现路由/回退/限流、判定工具动作安全性、合成任何 verdict。这些是网关或客户端的职责；oxa 提供的是本矩阵所述的协议保真度。

## 2. 定级词汇

每个「语义 × 方向」格标注以下之一：

- **P（preserved）**：目标面存在语义等价的表示，转换零语义损失（0 loss 或仅有 spec 规定的 native 回转省略）。
- **D（degraded）**：目标面存在尽力表示，但 MUST 产生确定性 loss；格标注锚点为 `reason | path | field`。
- **U（unsupported）**：语义不可表示。转换 MUST 省略该语义并产生显式 `unsupported-semantic` loss；调用方可确定性映射到请求侧 4xx。MUST NOT 静默丢弃，MUST NOT 合成替代。
- **R（envelope-regenerated）**：该语义属于传输信封（ID、索引、chunk 编号），跨面重建且**不承诺保真**；它是传输事实而非协议语义，不参与 P/D/U 定级。

D 与 U 的区别是契约性区别：D 允许请求继续（调用方决定是否接受降级），U 表示目标面无法承载该语义（调用方通常应拒绝或改道）。两者的判定只依据本矩阵与回指规则，不依据 loss 的 `detail` 文本。

## 3. 控制面语义清单

| # | 语义 | IR 承载 | CC wire | RE wire | AN wire | 权威规则 |
|---|------|---------|---------|---------|---------|----------|
| 1 | privileged instruction | `System[]`（`SystemBlock.Role`: system/developer tier） | `system` / `developer` 消息 | `instructions` / developer item | 顶层 `system` | spec/01 §3.2；N-R-5；N-AN-* |
| 2 | 结构化输出 | `Params.ResponseFormat` | `response_format`（嵌套 `json_schema`） | `text.format`（扁平） | `output_config.format` | N-CC-13；N-R-14；N-AN-12 |
| 3 | 强制工具选择（具名） | `ToolChoice{Mode:"tool", Name}` | `tool_choice` 具名对象 | `tool_choice` 具名对象 | `tool_choice.type:"tool"` | N-CC-7；N-R-9；N-AN-6 |
| 4 | 禁用工具（`none`） | `ToolChoice{Mode:"none"}` | `tool_choice:"none"` | `tool_choice:"none"` | `tool_choice.type:"none"` | N-CC-7；N-R-9；N-AN-6 |
| 5 | 工具循环历史 | `ToolUseBlock` + `ToolResultBlock` | assistant `tool_calls` + `role:"tool"` | `function_call` / `function_call_output` 项 | `tool_use` / `tool_result` 块 | N-CC-*；N-R-5；N-AN-* |
| 6 | 输出预算 | `Params.MaxOutputTokens` | `max_tokens` / `max_completion_tokens` | `max_output_tokens` | `max_tokens` | N-CC-*；N-R-*；N-AN-*；N-NAT-2 |
| 7 | 响应正文文本 | `TextBlock` | message `content` | `output_text` | `text` 块 | 各面映射 |
| 8 | 停止原因 | `Response.StopReason` + `StopSequence` | `finish_reason` | `status` | `stop_reason` / `stop_sequence` | 各面映射；N-S-1 |
| 9 | usage | `Response.Usage` | usage chunk / 非流 usage | `response.completed` usage | `message_delta` usage / 非流 usage | spec/20 §2.2 |
| 10 | 流终端类型 | `MessageDelta` + `MessageDone` | `finish_reason` chunk（usage-only 尾块 MAY 跟随） | `response.completed` / `.incomplete` / `.failed` | `message_delta` → `message_stop` | N-S-1；N-S-5/6/7 |
| — | 响应关联标识 | （传输信封） | chunk `id` / `index` | response/item `id` | `message_start` `id` / block `index` | spec/20 §2.3 |

语义 1–6 为请求侧，7–10 为响应侧；第 10 行（响应关联标识）不设 IR 语义，只有传输信封表示。

## 4. 请求转换矩阵（3×3）

方向记号：`R→C` 表示 Responses 源 → Chat Completions 目标。列顺序固定：R→R、R→A、R→C、A→R、A→C、A→A、C→R、C→A、C→C。

| 语义 | R→R | R→A | R→C | A→R | A→C | A→A | C→R | C→A | C→C |
|------|-----|-----|-----|-----|-----|-----|-----|-----|-----|
| privileged instruction | P | P | P | P | P | P | P | D¹ | P |
| 结构化输出 | P | D² | P | U³ | U³ | P | P | D² | P |
| 强制工具选择（具名） | P | P | P | P | P | P | P | P | P |
| 禁用工具（`none`） | P | P | P | P | P | P | P | P | P |
| 工具循环历史 | P | P | P | P | P | P | P | P | P |
| 输出预算 | P | P | P | P | P | P | P | P | P |

降级与不支持格的 loss 锚点（每格一条语义 omission 一条 loss，spec/02 §6）：

- **¹ C→A privileged instruction**：`degraded | system[i].role | role`（Chat `developer` 消息降为 Anthropic system 内容；Anthropic 无 developer tier。detail 固定提及 "Anthropic has no developer role"）。i 为该指令块在 `System[]` 中的下标。
- **² R→A 与 C→A 结构化输出**：`unmapped-field | params.response_format.name | name` 与 `unmapped-field | params.response_format.strict | strict`（N-AN-12：Anthropic 只承载 schema；若源带 description 再加一条 `params.response_format.description`）。
- **³ A→R 与 A→C 结构化输出**：`unsupported-semantic | params.response_format | name`，整个 format 省略，MUST NOT 合成 name（N-R-14 / N-CC-13）。这是本矩阵唯一的 unsupported 格。

其余格为 P 的依据：N-CC-7 / N-R-9 / N-AN-6 的工具选择 1:1 映射、工具循环与预算的 1:1 映射（`max_completion_tokens` 经 N-NAT-2 native 保留）、 privileged instruction 的 system/developer tier 映射（spec/01 §3.2）。

未知字段与未知枚举值始终按 spec/02 报 `unmapped-field` / `unmapped-value`，与上表「已知但不可表示」的语义区别开：前者是输入审计，后者是能力定级。

## 5. 响应转换矩阵（3×3）

| 语义 | R→R | R→A | R→C | A→R | A→C | A→A | C→R | C→A | C→C |
|------|-----|-----|-----|-----|-----|-----|-----|-----|-----|
| 响应正文文本 | P | P | P | P | P | P | P | P | P |
| 工具调用 | P | P | P | P | P | P | P | P | P |
| 停止原因 | P | P | P | P | P | P | P | P | P |
| usage | P | P | P | P | P | P | P | P | P |
| 响应关联标识 | R | R | R | R | R | R | R | R | R |

响应侧三面映射为 1:1（各映射文档）；refusal、thinking、citations、web_search_call、Apply Patch 等扩展语义不在控制面清单内，其定级回指 spec/11、12、13、14、16、20 的对应规则与 loss catalog。R 格的含义见 §2：ID 与索引由目标面重建，内容语义不受影响。

## 6. 流生命周期契约

流语义的完整规范在 [spec/20](20-streaming-semantics.md)；本节只声明跨面消费方依赖的契约要点：

1. **终端观察表**：CC 的 `finish_reason`（usage-only 尾块 MAY 跟随）、RE 的 `response.completed` / `.incomplete` / `.failed`、AN 的 `message_delta` → `message_stop`（N-S-1）。
2. **禁假终端**：解码器 MUST NOT 在原生生机缺失时虚构 stop reason、usage 或终端事件（N-S-1）；上游流在部分文本交付后失败时，不存在任何产生成功终端的路径。
3. **增量保真**：文本 delta 保序、工具参数分片按遭遇顺序拼接等价（INV-5/INV-6；N-S-9 等价规则；M7 工具参数聚合 N-S-10）。
4. **信封重建**：目标面的 chunk/item ID 与索引由编码器重建（spec/20 §2.3 允许合成），**不承诺跨面 round-trip 保真**；调用方 MUST NOT 假设跨面流保持传输标识恒等。同面 round-trip 的信封保真由各面向量锁定。
5. **usage 保留**：流式 usage 经 IR 语义保留（spec/20 §2.2、各面 profile）。

## 7. `stream` 请求标志的传输语义

请求体中的 `stream` 标志是**调用方的传输意图**，不是协议负载语义：

- **CC**：自 2.4.10 起作为 `Request.Native`（`kind: "stream"`）受限保留（N-NAT-2），同面 round-trip 精准还原，跨面转换 0 loss 静默省略。
- **RE / AN**：无 native 表示；decode 按 spec/02 报 `unmapped-field | stream | stream`（v3.1.0 现状，三面不对称，本节如实记录为有意行为）。
- **调用方契约**：一次转换是否流式由调用方选择的转换器 API（streaming decoder/encoder 与 nonstreaming converter）决定；MUST NOT 依赖请求体标志推断目标行为。`err == nil` 且无 stream 相关 loss 时，语义负载已完整转换；流式与否是调用侧事实。

将 RE/AN 对齐到 CC 的 native 保留属于 spec 变更，必须走 minor 版本 + 向量锁定流程；在此之前上表即契约。

## 8. Loss 稳定性承诺

调用方可以据以做策略决策的稳定契约（与 spec/02 一致并升格为兼容性承诺）：

1. `LossReason` 是四值闭集：`unmapped-field`、`unmapped-value`、`unsupported-semantic`、`degraded`。已发布值 MUST NOT 删除或改变含义。
2. `Loss.Path` 恒为源文档 root-relative 路径，按遭遇顺序确定性产出（spec/02 §3、§7）。
3. `Loss.Field` 标识符一经发布不再改名。
4. 每个语义 omission 产出一条 loss，不按字节或事件重复计数（spec/02 §6）。
5. 版本规则：新增 reason 值允许 minor；改变已发布 reason/path/field 语义或顺序保证属于破坏性变更，MUST 走 major 并附迁移说明。

`Loss.Detail` 是补充信息，不构成契约；调用方 MUST NOT 解析其文本做决策。

## 9. 格级向量锁定

golden vectors 是本矩阵的可执行形式。自 3.2.0 起：

- 每一个 D 格与 U 格 MUST 有至少一个 golden vector 锁定其 loss 锚点（`(path, field, reason)` 集）；新增此类格 MUST 与锁定向量同 commit。
- §4 的每行至少一个 P 格由跨面请求向量覆盖（现状已满足）。
- §6 的流契约 MUST 由跨面流向量锁定：`vectors/cross/stream/` 向量（源面 native 事件 → 目标面 native 事件，经 IR 事件流组合）自 3.2.0 起进入向量 schema 的合法组合，vectest 类框架 MUST 支持其执行。

3.2.0 落地的锁定集合（Go 参考实现已全数执行通过）：

- R→A 结构化输出 degraded 锚点：`cross.nonstream.responses-to-anthropic-json-schema-request`；
- C→A privileged instruction degraded 锚点（含 schema 降级）：`cross.nonstream.chatcompletions-to-anthropic-developer-json-schema-request`；
- 禁用工具（`none`）：RE 面 `responses.nonstream.tool-choice-none-{to,from}-ir` 与全部 6 个跨面向量 `cross.nonstream.{responses,anthropic,chatcompletions}-to-{chatcompletions,responses,anthropic}-tool-choice-none`；
- `cross/stream/` 全部 9 对：`{responses,anthropic,chatcompletions}-to-{responses,anthropic,chatcompletions}-multi-delta-text`（文本 delta 分片与信封重建断言）、`chatcompletions-to-anthropic-tool-arguments`（工具参数分片）、`chatcompletions-to-anthropic-usage-only-terminal`（usage-only 终止）。

## 10. 参考

- [02 — Loss Policy](02-loss-policy.md)：loss 记录结构、顺序、约定。
- [10](10-mapping-openai-chat-completions.md) / [11](11-mapping-openai-responses.md) / [12](12-mapping-anthropic-messages.md)：各面映射规则与 loss catalog（N-CC-*、N-R-*、N-AN-*）。
- [15](15-native-request-options.md)：native annotation 与传输标志保留（N-NAT-*）。
- [20](20-streaming-semantics.md)：流契约（N-S-*）。
- 向量盘点依据：`docs/research/2026-11-08-control-plane-vector-inventory.md`（v3.1.0 实测记录）。
