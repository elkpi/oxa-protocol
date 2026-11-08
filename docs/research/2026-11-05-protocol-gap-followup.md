# 协议缺口跟进（相对 v2.5.1）

**调查日期：** 2026-11-05

**状态：** 调研结论。不改 `spec/`，不改实现，不改向量，不提交 git。不写字节级映射。

对照的是当前 main 的规格 **2.5.1**（IR contract **0.2.0**，`spec/README.md`）。仓库侧「已支持」只看规格正文。语言实现谁没跟上规格，不进这份清单。唯一例外是文内那条 P0：规格没写、Go 已经在报的 loss，并给出文件路径。

上次清单是 `docs/research/2026-10-31-three-face-gap-inventory.md`，对照的是规格 2.4.13。那份 P1 里已经在 2.5.0 接通的，本轮不再当成缺口。

判定只收 wire：请求/响应 JSON、SSE/WebSocket 事件、会改变消息形状的 beta header。产品模式、CLI、运行时策略、模型能力、定价、限流、HTTP 客户端、鉴权、重试、路由、工具执行、Realtime 传输本身，都不算缺口。

一个特性只有在至少一张脸的 wire 上出现，并且转换时会丢信息或记 loss，才算缺口。两张脸有、第三张没有：缺的那张脸记有序 loss，不造假字段。

## 1. 结论

用户点名的三项都不进 oxa 转换层。

**Claude auto mode：不支持，也不该支持。** 官方有两处同名，都不是 `POST /v1/messages` 的字段。Claude Code 的 auto 是权限模式：`permissions.defaultMode` 取 `"auto"`，或 `claude --permission-mode auto`。另一个模型（文档称 classifier）在动作执行前做审查，deny/ask 规则先于它生效。Managed Agents 的 auto 是工具集上的 `permission_policy: {"type":"auto"}`，beta header 是 `managed-agents-2026-04-01`，事件上有 `evaluated_permission` 和 `evaluation`。那是 `/v1/agents` 与 `/v1/sessions`，不是 Messages。Messages 里已有的 `tool_choice` `"auto"` 是另一件事，规格已经映射。把 auto mode 接进 oxa，会变成权限策略引擎，越界。

**safeguards_classifier：没核完，已打开的官方页里没有这个 wire 字段。** 在 Claude Code 权限文档、auto mode 配置、Managed Agents 权限策略、Messages 总览、Chat/Responses create reference、beta header 说明里，字符串 `safeguards_classifier` 都没有出现。文档写的是 classifier，或「safety classifiers 拒绝请求」。后者落到 Messages 上是已接通的 `stop_reason: refusal` 与 `stop_details`，不是一个名叫 `safeguards_classifier` 的字段。不能据此写成「全网不存在」。Codex app-server 协议全文本轮没逐页打开。

**codex reviewer：不支持，也不该支持。** 和转换层有关的 wire 不在三张脸上。Agents API（beta）创建会话 `POST /v1/agents/sessions` 有字符串字段 `review`：`"auto_review"` 或 `"none"`，省略等于 `"none"`。官方原句是 automatic review 把 Codex harness 的 “Approve for me” 带到 agents；由 OpenAI 管理的 reviewer model 按风险和用户指令批准或拒绝。同一页写明：只设 `review` 不会配置 sandbox，也不改变环境的网络限制。Responses create 的参数表里没有 `review`。Codex Code Review 的自定义规则是产品/app-server 侧的事。接进 oxa 会变成审批执行器。

官网相对 2026-10-31 清单、且本轮在 reference 里读到的新 wire，只有两条，都是单脸、没有跨脸角色：Responses 请求 `image.context_compression`，以及 Messages beta `thread`。都不建议进近期 minor。

仍值得做的旧缺口没有消失：推理回放密文、`logprobs`、`web_search_call` 的流式、Anthropic `cache_control` 对 OpenAI 缓存断点、压缩上下文、原生工具的调用/结果。另有一条 P0：Go 把 thinking `signature` 编到 Responses 时已经报 `unmapped-field`，规格没写这句话。

## 2. 来源与没核完的部分

| 读到的页面 | 用来核什么 |
|---|---|
| https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create | Chat create 顶层参数 |
| https://developers.openai.com/api/reference/resources/responses/methods/create | Responses create 顶层参数，含 `image.context_compression` |
| https://platform.claude.com/docs/en/api/messages | Messages 总览；`docs.anthropic.com/en/api/messages` 打开后落到这页 |
| https://platform.claude.com/docs/en/api/beta-headers | `anthropic-beta` 怎么送；端点级 header |
| https://platform.claude.com/docs/en/build-with-claude/message-threads | `thread` 对象与 beta 值 |
| https://platform.claude.com/docs/en/release-notes/overview | 2026-10-01 之后的 API 变更条目 |
| https://platform.claude.com/docs/en/managed-agents/permission-policies | Managed Agents 的 `permission_policy.type = auto` |
| https://code.claude.com/docs/en/permission-modes | Claude Code auto 权限模式 |
| https://code.claude.com/docs/en/auto-mode-config | classifier 信任哪些仓库/域名 |
| https://developers.openai.com/api/docs/guides/agents-api/sessions/approvals | Agents 会话 `review` |
| `spec/README.md`、`spec/CHANGELOG.md`、`spec/10`、`spec/11`、`spec/12`、`spec/15`、`spec/16`、`spec/20` | 当前规格写保留还是写丢 |

没核完，不当成「不存在」：

- `https://platform.openai.com/docs/api-reference/responses/create` 这次返回 403。OpenAI 字段以 `developers.openai.com` 的 create reference 为准，不是无缓存浏览器的完整快照。
- Responses streaming events 专页（https://developers.openai.com/api/reference/resources/responses/streaming-events）本轮没有逐条重读。流式缺口只沿用规格已经写明的 `web_search_call` 整项 loss，不新报「没见到的事件」。
- https://developers.openai.com/api/docs/guides/agents/guardrails-approvals 第一次抓取的摘录写：Responses API 与 Agents SDK 不会自动继承 Codex Auto-review，且该摘录没有 wire 字段名。本轮没能再打开正文核对原句。Codex 的判定以 approvals 页的原句为准。
- Codex app-server 协议、computer / search / code execution 的独立指南，没有逐页打开。
- Messages create 的逐字段 schema 页没有单独再抓一份；`thread` 以 message-threads 指南和 2026-11-02 发布说明为准。

Chat create 顶层参数与 10-31 清单一致，没有新的顶层名字：`audio`、`frequency_penalty`、`function_call`、`functions`、`logit_bias`、`logprobs`、`max_completion_tokens`、`max_tokens`、`metadata`、`modalities`、`moderation`、`n`、`parallel_tool_calls`、`prediction`、`presence_penalty`、`prompt_cache_key`、`prompt_cache_options`、`prompt_cache_retention`、`reasoning_effort`、`response_format`、`safety_identifier`、`seed`、`service_tier`、`stop`、`store`、`stream`、`stream_options`、`temperature`、`tool_choice`、`tools`、`top_logprobs`、`top_p`、`user`、`verbosity`、`web_search_options`。

Responses create 顶层里，10-31 清单没单独写出的是 `image`（子字段 `context_compression`）。其余顶层名仍是当时那一批：`access_programs`、`background`、`context_management`、`conversation`、`include`、`input`、`instructions`、`max_output_tokens`、`max_tool_calls`、`metadata`、`model`、`moderation`、`parallel_tool_calls`、`previous_response_id`、`prompt`、`prompt_cache_key`、`prompt_cache_options`、`prompt_cache_retention`、`reasoning`、`safety_identifier`、`service_tier`、`store`、`stream`、`stream_options`、`temperature`、`text`、`tool_choice`、`tools`、`top_logprobs`、`top_p`、`truncation`、`user`。

## 3. 2.5.x 已接通，本轮不再列为缺口

规格 2.5.0（2026-11-01）和 2.5.1（2026-11-03）已经写进 changelog 的，不要再按 10-31 清单报一遍。

| 项 | 规格现在怎么处理 |
|---|---|
| `metadata` | `Request.Metadata`。CC / Responses 字符串图双向零损失。Anthropic 只留 `user_id`，其余键 `unmapped-field`。 |
| `developer` | system 块可选 `role: "developer"`。Responses 同脸可回转。Anthropic 文本仍进 system，逐块 `degraded`。 |
| `verbosity` | `Params.Verbosity`。CC 与 Responses 双向。Anthropic 编码报损。 |
| 用户文件 | `input_file` / Chat `file` / Anthropic `document` 进 `DocumentBlock`。对不上的 source、`filename`、`detail` 仍按项 loss。 |
| 拒绝正文 | `RefusalBlock`。CC `message.refusal`、Responses `type: "refusal"`、Anthropic `stop_details.explanation` 互通。`category` 编到 OpenAI 是 `unsupported-semantic`。 |
| 并行工具 | `ToolChoice.disable_parallel`。`parallel_tool_calls` 与 `disable_parallel_tool_use` 极性取反。 |
| OpenAI 缓存键与断点 | `prompt_cache_key`、`prompt_cache_options`、`prompt_cache_breakpoint` 在两张 OpenAI 脸之间零损失。编到 Anthropic 是 `unmapped-field`。这不是「已经和 `cache_control` 接上」，见下一节。 |
| 引用进 IR | `TextCitation.source` 增加 `"openai"`。Responses `output_text.annotations` 同脸可回转。编到另一来源仍是规格写明的整段省略，不是没槽。 |
| thinking 预算表 | `budget_tokens <= 1024` 改为 `minimal`。这是映射修订，不是新字段。 |
| 空 `additional_tools` | 2.5.1：空数组是 no-op，一条 `unmapped-field`，不拆 run。 |

引用的跨脸省略留在规格里（N-AN-18 的 detail 写目标脸保不住位置和回放元数据）。2.5.0 已经选择整段省略，不是漏做的槽。除非以后单独立项做位置类型投影，否则不排进近期补充。

## 4. 仍开放、且规格仍写丢弃的旧缺口

P0 只有一条。P1 是同一语义角色，规格今天记 loss，或根本没槽。不发明别的 P0。

### P0 — 规格没写、Go 已经在报

Thinking `signature` 编到 Responses。N-R-13 只写 `encrypted_content` 丢弃，以及其他 `reasoning` 属性记 `unmapped-field`。`spec/20` §11.2 写的是 CC 编码上的 `SignatureDelta`，以及 `encrypted_content` 事件被吸收。没有写 Responses 编码路径上的 signature。

Go 已经在报，本轮只核对了 Go：

- `go/openai/responses/encode.go`：请求路径约 479–483 行，detail `Responses request reasoning items carry no provider signature`；响应路径约 619–623 行，detail `Responses reasoning items carry no provider signature`。都是 `unmapped-field`。
- `go/openai/responses/streamout.go` 约 205–212 行：`SignatureDelta` 记 `unmapped-field`，detail `Responses carries no signature delta in streaming reasoning`。

建议只做规格补句，把今天的 loss 写死。不要在这条里规定它如何进不透明槽。其他语言有没有同一条 loss，本轮不查。

### P1 — 规格仍写丢，值得做

| 项 | 规格今天 | 建议 | beta |
|---|---|---|---|
| 推理回放密文 `encrypted_content` | N-R-13：丢弃并记 `unmapped-field`。Anthropic `redacted_thinking` 同脸保留，编到 OpenAI 整块 `unsupported-semantic`（N-AN-17），密文禁止写进 loss。 | 一个不透明槽，原样搬运，不解码。没有 wire 落点的那张脸记 loss，不造可见 thinking 文本。与上面的 P0 分开：P0 只记录现状。 | reference 未标 beta |
| `logprobs` / `top_logprobs` | `spec/10` 请求表写明 `unmapped-field`，loss catalog 写 log-probability sampling 没有 IR 等价物。Responses create 另有 `top_logprobs`；`spec/11` 映射表没点这个名字，未点名的按 `spec/02` 报损。 | 两张 OpenAI 脸接通采样开关和输出载荷。Anthropic 记 loss。不要改文本块的必填形状。 | 未标 |
| `web_search_call` 流式 | 非流式 N-R-16 已进 `WebSearchCallBlock`。`spec/20` 与 N-R-16 写流式对任何目标脸，包括 Responses 自己，都是整项 `unsupported-semantic`，不发内容事件。 | 同脸流式发出与非流式相同的块。跨脸仍可再记 `unsupported-semantic`。 | 未标 |
| 显式缓存断点跨脸 | OpenAI 三个 `prompt_cache_*` 已在家族内回转。Anthropic `cache_control` 仍是 NativeAnnotation。`spec/15` §3.2 仍写：编到 CC / Responses 时 detail 为 `anthropic-native cache breakpoint; the target face has no cache_control representation`。这句在 2.5.0 之后已经过时：OpenAI 有断点字段，只是没和 `cache_control` 接上。 | 断点对 `cache_control`。对不上的 TTL 记 loss，不要把 `1h` 折成别的值。`prompt_cache_key` 继续只在两张 OpenAI 脸之间回转。 | OpenAI 字段 reference 未标 beta |
| 压缩上下文 | Anthropic `context_management` 是原生注解。`spec/15` §3.3 只识别 `clear_tool_uses_20250919` 与 `clear_thinking_20251015`。未知 edit `type` 整项 `unsupported-semantic`。Responses create 的 `context_management` 是数组，元素里读到 `type` 与 `compact_threshold`，说明是 “Context management configuration for this request.” `spec/11` 映射表没有这项。 | 接通「压缩上下文」这个角色。clear-tool / clear-thinking 在 Responses 没有同名落点时记 loss，不要当成 compaction 本身。 | Anthropic 侧仍是 beta 能力；本轮没在 reference 里逐项读到 compaction edit 的 header 值。Responses 字段未标 beta |
| 原生工具的调用和结果 | `spec/16` 只做同源 opaque 声明。跨来源整份声明 `unsupported-semantic`。正文写明不新增 `server_tool_use`、结果块、响应和历史映射。`web_search_call` 非流式是声明之外已经单开的块，不代表其他角色已接通。 | 角色接通要声明、调用、结果一起看。带日期的 type 字符串各留各的脸。preview 不接正式版。这是单独评审，不放进下面的前几步。 | 部分 Anthropic 工具带 beta；以 `spec/16` 已写的范围为限，本轮不重开工具指南 |

仍丢、但不排期：请求 `reasoning` 上除 `effort` 以外的属性（含 summary 偏好）按 N-R-13 记 `unmapped-field`。它和 `thinking.display` 是不是同一角色，10-31 清单标成待定。本轮没有新的一手证据把它们收成一个字段，不推翻那条损失。

规格没点名、只靠 `spec/02` 未知字段审计的，不升成新 P1，也不为它们发版。10-31 清单里的 `moderation`（create reference：可选对象，子字段 `model`、`policy`；两张 OpenAI 脸都有，说明是请求级审核配置）仍是这种情况。Anthropic 没有同名字段。要做的话是两张 OpenAI 脸回转，优先级低于上表里规格已经写明的 loss。

## 5. 新缺口

相对 10-31 清单，本轮在官方 reference 里新读到、且规格没有槽的，只有下面两行。都是 P3：只有一张脸，对另外两张脸没有可对齐的角色。原脸若出现，按 `spec/02` 报 `unmapped-field` 即可。不要扩 IR，不要造另外两张脸的字段。

| 项 | 官方字段 | 脸 | IR / 规格今天 | 建议 | beta | 来源 |
|---|---|---|---|---|---|---|
| 图像上下文压缩 | Responses 请求 `image`，可选对象。子字段 `context_compression`：`"auto"` \| `"enabled"` \| `"disabled"`。读到的句子：Controls image context optimization. Defaults to `auto`, which uses the model, transport, and input to select a policy. Use `enabled` to require compression support or `disabled` to turn it off. Chat create 参数表里没有同名字段。 | 仅 Responses | 映射表没点名 | 不进近期。它是压缩策略，不是可跨脸搬运的图像内容。 | reference 未标 | https://developers.openai.com/api/reference/resources/responses/methods/create |
| 消息线程 | Messages 请求增加 `thread`。创建：`{"type":"create"}`。续接：`{"type":"continue","previous_message_id":"<id>"}`。指南写：后面的请求只送新消息，`system` 和 `tools` 沿用创建时的值；线程在最后一次请求 5 分钟后过期，最迟 1 小时删除。响应仍是普通 message。 | 仅 Messages | 映射表没点名 | 不进近期。这是服务端会话续接，同类于 Responses 的 `previous_response_id`（10-31 已列为单脸，不扩 IR）。续接失败时的重放由调用方保留全文，不是转换层的职责。 | `anthropic-beta: message-threads-2026-08-12`。发布说明日期 2026-11-02。 | https://platform.claude.com/docs/en/build-with-claude/message-threads ；https://platform.claude.com/docs/en/release-notes/overview |

发布说明里同期还有这些，不是三张脸的新内容字段，不进缺口表：Messages 请求体上限从 32 MB 提到 128 MB（2026-10-28）；Opus 4.8 的 `speed: "fast"` 将于 2026-11-30 移除（`speed` 在 10-31 已是单脸待定）；Usage and Cost Analytics、Claude Tag、Models API 的 `line` / `capabilities.*` 都不是这三张脸。

## 6. 明确排除

| 项 | 一手来源里它是什么 | 为什么不进 oxa |
|---|---|---|
| Claude Code auto mode | 权限模式，不是 Messages 字段。权限文档：auto mode 让 Claude 不必对常规动作逐次询问；另一个 classifier 在执行前审查，挡住超出请求、指向未识别基础设施、或看起来受恶意内容驱动的动作。显式 ask 规则仍然会提示。设置键是 `permissions.defaultMode: "auto"`。同页没有字符串 `safeguards_classifier`。 | 产品运行时策略。wire 上没有可转换的 Messages 字段。接进来就是策略引擎。https://code.claude.com/docs/en/permission-modes |
| Managed Agents `permission_policy.type = "auto"` | 工具集 `default_config.permission_policy` 或单个工具的 `configs[]`。取值还有 `always_allow`、`always_ask`。`auto` 由服务端对每次调用给出运行、拒绝或暂停待批。事件 `agent.tool_use` / `agent.mcp_tool_use` 带 `evaluated_permission`（`"allow"` / `"ask"` / `"deny"`）和 `evaluation`。`evaluation.type = "auto"` 时，拒绝的 `reason_code` 读到 `"high_risk"`，暂停读到 `"indeterminate"`。beta：`managed-agents-2026-04-01`。适用端点是 `/v1/agents`、`/v1/sessions`、`/v1/environments`。 | 不是三张脸。而且语义是「服务端决定工具能不能跑」。oxa 不执行工具，也不做审批。https://platform.claude.com/docs/en/managed-agents/permission-policies |
| `safeguards_classifier` | 本轮打开的官方页没有这个字段名。Claude Code 文档把审查者叫 classifier。发布说明写 Claude Fable 5 的 safety classifiers 会拒绝请求；那在 Messages 上是已接通的 refusal / `stop_details`，不是新字段。 | 没有可转换的 wire 字段就不进。没读完的协议页见第 2 节，不把「没核完」写成「不存在」。 |
| Codex reviewer / Agents `review` | approvals 页原句：Set the session’s `review` field。`"auto_review"`：Automatically approve or deny supported requests based on the action’s risks and the user’s instructions in the conversation。`"none"`：Automatic review is not applied。省略则为 `"none"`。创建示例是 `client.beta.agents.sessions.create({..., review: "auto_review"})`，路径是 `/v1/agents/sessions`。同一页：Automatic review brings the “Approve for me” functionality of the Codex harness to your agents。Setting `review` alone does not configure sandboxing。Automatic review does not change the environment’s network restrictions。 | 不是 CC / Responses / Messages 的字段。批准或拒绝工具副作用是执行器。Responses create 参数表没有 `review`。https://developers.openai.com/api/docs/guides/agents-api/sessions/approvals |
| Codex Code Review 产品与 app-server | 官方博客把自定义 review 规则放在 Codex 产品上，并提到 app-server 通知 `rawResponseItem/completed` 不应改名。本轮没打开该协议的完整事件表。 | 产品/运行时协议，不是三张脸的响应事件。在事件表读完之前，不把它报成 Responses 流式缺口。https://developers.openai.com/blog/custom-code-review-rules-for-codex |
| Responses WebSocket 传输、Realtime | 已有调研：WebSocket mode 每个 response 的服务端事件复用 Responses 流式模型；连接、`response.create`、`response.steer`、lane 不是 oxa 的事。Realtime 是另一套协议。 | 传输层与第四张脸。见 `docs/research/2026-10-25-openai-responses-websocket.md`。 |
| HTTP 客户端、鉴权、重试、路由、模型能力表、定价、限流、SDK 便捷属性 | 项目契约已排除。Responses 顶层 `output_text`、SDK 的 `messages.thread()` helper 属于调用方便捷，不是另一套 wire。 | 不进转换层。 |

Messages 的 `tool_choice` 取值 `"auto"` 继续按现有工具选择映射，不要和上面的 auto mode 合并。

## 7. 建议的补充顺序

只排序。不写字节级映射，不把下面任何一步写成已经发布的版本号。

1. **patch。** 只把 Go 已经在报的 Responses signature loss 写进规格，并加一条锁定「今天就是 loss」的向量。不接通不透明槽。
2. **下一次 minor 的第一刀。** `encrypted_content` 的不透明槽（密文不进 loss detail）；`logprobs` / `top_logprobs`（只在两张 OpenAI 脸）；`web_search_call` 的同脸流式。三块互不依赖，可以分开提交。
3. **同一 minor 的第二刀，单独评审。** 把 Anthropic `cache_control` 和 OpenAI `prompt_cache_breakpoint` 接成同一角色，并改掉 `spec/15` §3.2 那句「目标脸没有 cache 表示」。TTL 对不上就 loss。
4. **再往后，单独评审。** 压缩上下文。然后才是五个原生工具角色的调用和结果；每刀一个角色，先同脸回转，日期版本不互映射。
5. **不进 oxa。** auto mode、`safeguards_classifier`、Codex reviewer / Agents `review`、`image.context_compression`、Messages `thread`、Realtime、WebSocket/HTTP 传输、通用未知字段透传。

10-31 清单里标成待定的（shell 对 bash、`reasoning.summary` 对 `thinking.display`、`service_tier`、Apply Patch 对 text editor、`safety_identifier` 对 `metadata.user_id` 等）本轮没有新的一手证据，维持待定，不进上面五步。
