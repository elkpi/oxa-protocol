# 协议缺口：适合落地的下一批（相对规格 3.2.0）

**调查日期：** 2026-11-11

**状态：** 调研结论。不改 `spec/`，不改实现，不改向量，不提交 git。不写字节级映射。

对照的是当前 main 的规格 **3.2.0**（IR contract **0.3.0**，`spec/README.md`）。「已支持」只看规格正文；实现只用来核对规格已经写明的丢弃是否真的发生。语言之间谁没跟上规格，不进这份清单。

上一份缺口清单是 `docs/research/2026-11-05-protocol-gap-followup.md`，对照的是规格 2.5.1。那份里已经在 2.5.2–3.2.0 接通的，本轮不再当成缺口。

判定沿用上次：只收 wire（请求/响应 JSON、SSE 事件、会改变消息形状的 beta header）。产品模式、CLI、运行时策略、模型能力、定价、限流、HTTP 客户端、鉴权、重试、路由、工具执行、Realtime / Live 传输本身，都不算缺口。一个特性只有在至少一张脸的 wire 上出现，并且转换时会丢信息，才算缺口。两张脸有、第三张没有：缺的那张脸记有序 loss，不造假字段。

## 1. 结论

2026-11-05 之后，官方新增的、值得 oxa 接的 wire 很少。发布说明里的新条目大多是模型行为、企业控制台或另一套 API。三张对话脸上仍值得做的，是上次清单没做完、而且规格今天仍会丢掉的几项。其中会把**已经生成的对话状态**丢掉的，优先于只丢请求开关的。

建议落地的顺序：

1. **Responses 的 `compaction` 项，同脸回转。** 今天请求 `input[]`、响应 `output[]`、流式 `output_item.done` 都把它当未知项丢掉。这是服务端压缩后必须原样带回的检查点。密文不能译到 Anthropic 的 compaction 块上，也不要复用那块「只能出现在 `messages[0].content[0]`」的位置规则。
2. **各脸自己的压缩触发器，先同脸留住。** Anthropic `context_management.edits[].type = "compact_20260112"` 不在 spec/15 §3.3 的白名单里，同脸回转会被整项跳过。Responses 请求上的 `context_management: [{type: "compaction", compact_threshold}]` 规格映射表没有这个名字。两套触发器形状不同，第一刀不要互相折算。
3. **显式缓存断点跨脸。** Anthropic `cache_control` 和 OpenAI `prompt_cache_breakpoint` 都是「这块内容上有一个显式断点」。TTL 词汇没有交集（`"5m"` / `"1h"` 对 `"30m"` / `"12h"`），断点可以对齐，TTL 必须记 loss，不能折算。spec/15 §3.2 那句「目标脸没有 cache 表示」在 2.5.0 之后已经不成立。
4. **`logprobs` / `top_logprobs`，只在两张 OpenAI 脸。** 规格今天写明没有 IR 等价物。范围比上面三项小。
5. **原生工具的调用和结果，单独评审，一次一个角色。** 声明已经按来源不透明回转；`server_tool_use` 和结果块规格明确不做。先做 web search 的同脸历史，不要把带日期的 type 字符串互映射。

不进 oxa：消息线程、`previous_response_id`、`conversation`、`fallbacks`、`image.context_compression`、`access_programs`、Agents / Managed Agents、Realtime、WebSocket 传输。理由在第 6 节。

## 2. 来源

| 读到的页面或文件 | 用来核什么 |
|---|---|
| `spec/README.md`、`spec/CHANGELOG.md` 2.5.2–3.2.0 | 上次清单之后规格已经写了什么 |
| `spec/11` §3、§4、loss catalog；`go/openai/responses/decode.go`、`streamin.go` | 未知 Responses item 的丢弃 |
| `spec/15` §3.2、§3.3、§3.7；`go/anthropic/messages/request_native.go` | 缓存断点与 context edit 白名单 |
| `spec/12` N-AN-22；`spec/16` §1；`spec/20` 对 `server_tool_use` 的行 | compaction 块的位置约束、原生工具声明的边界 |
| `spec/00` §2 | 路由、鉴权、重试不是 oxa 的事 |
| https://developers.openai.com/api/reference/resources/responses/methods/create | Responses create 的 `context_management`、`image.context_compression` |
| https://developers.openai.com/api/docs/guides/compaction | 服务端压缩与 `/responses/compact` 要回放什么 |
| https://developers.openai.com/api/reference/resources/responses/methods/compact | `type: "compaction"` 项的字段；外层对象是 `response.compaction` |
| https://developers.openai.com/api/reference/resources/responses/streaming-events | `response.compaction.compacting` 不携带摘要；密文在 `output_item.done` |
| https://developers.openai.com/api/docs/changelog | 2026-11-02 至 2026-11-05 的 API 变更 |
| https://platform.claude.com/docs/en/release-notes/overview | 2026-10-15 之后的 Messages 变更，读到 2026-11-09 |
| https://platform.claude.com/docs/en/build-with-claude/compaction | 按需压缩与阈值压缩是两条 beta |
| https://platform.claude.com/docs/en/build-with-claude/compaction-threshold | `compact_20260112` 的请求形状 |
| https://platform.claude.com/docs/en/api/beta | `thread`、`fallbacks`、`speed`、`task_budget`、按需 `compaction` 参数 |

没核完，不当成「不存在」：

- Chat Completions create 的逐字段页本轮没有重抓。2026-11 的 OpenAI changelog 没有给 `/v1/chat/completions` 加新的内容字段；上次清单里的顶层参数表仍作 Chat 的基线。
- Anthropic 按需压缩页（`compaction-on-demand`）没有逐字段重读。按需请求参数以 beta Messages schema 里出现的顶层 `compaction`（`type: "summarize"`，可选 `instructions`）和发布说明 2026-09-14 的句子为准。
- Responses 流式事件页对 compaction 项的摘录，输入侧字段是 `encrypted_content`、`type`、`id`；compact 端点返回侧另有可选 `created_by`。两处是否还有未摘出的字段，本轮不发明。
- 原生工具结果块（`web_search_tool_result`、`bash_code_execution_tool_result`、advisor 结果）的当前字段表没有重开。第 5 节只沿用 spec/16 已经写明的边界。

## 3. 2.5.2 之后已接通，本轮不再列为缺口

| 项 | 规格现在怎么处理 |
|---|---|
| Thinking `signature` 编到 Responses | 2.5.2 写进 N-R-13 与 spec/20 §11.2。三条固定 detail，不放进 `encrypted_content`。 |
| 推理回放密文 | 2.5.2 的 `EncryptedContentBlock`。Responses 同脸回转，包括流式挂到未关闭的 reasoning 项。CC 与 Anthropic 各一条 `unsupported-semantic`，detail 不含密文。不译成 `redacted_thinking`。 |
| `web_search_call` 同脸流式 | 2.5.2。`output_item.done` 发出与非流式相同的块。进度事件（`.in_progress`、`.searching`、`.completed`）仍是整项 `unsupported-semantic`，不改状态。跨脸仍省略。 |
| `tool_reference` | 2.5.3。Anthropic 同脸回转；CC 与 Responses 整块 `unsupported-semantic`。 |
| 对话中途工具变更与 Anthropic compaction **块** | 3.0.0。`CompactionBlock` 只允许作为 `messages[0].content[0]`，同脸原样回写。编到 CC / Responses 是 `unsupported-semantic`，detail 为 `Anthropic compaction block cannot be represented on the target face`。 |
| 控制面矩阵 | 3.2.0 的 spec/21。不新增 IR、不新增 loss reason。本轮不把它当成未做的协议缺口。 |

进度事件仍丢，是有意的：官方流式页写 `response.compaction.compacting` 也不携带摘要、不改输出项。web search 的进度事件维持现状，不单独立项。

## 4. 值得落地

### 4.1 Responses `compaction` 项（同脸，先做）

官方项：

- `type` 恒为 `"compaction"`。
- `encrypted_content` 为字符串，是压缩摘要的密文，不供人读。
- `id` 可选。compact 端点的返回项上还有可选 `created_by`。
- 服务端压缩由请求里的 `context_management` 触发；独立端点是 `POST /responses/compact`，返回对象的 `object` 是 `"response.compaction"`，不是普通 `"response"`。下一次 `/responses` 必须把返回的 `output` 原样放进 `input`，不能只留这一项（独立端点的输出里还可以有保留下来的 message）。

oxa 今天：

- 规格列举的 input 项类型没有 `compaction`。未知 `input[].type` 是一条 `unsupported-semantic`（spec/11 §3 与 loss catalog）。
- 响应 `output[]` 的在册类型是 message、`function_call`、`reasoning`、`web_search_call`。Go 对未知输出项走 default：`unsupported-semantic`，detail `Responses output item type %q has no IR equivalent`（`go/openai/responses/decode.go`）。
- 流式未知输出项同样是 `unsupported-semantic`，detail `Responses streaming output item type %q is not decoded`（`go/openai/responses/streamin.go`）。

所以一条已经压过的 Responses 历史，只要经过 oxa 再送回去，检查点就没了。这是转换层会破坏官方要求回放的状态，不是「多一个请求开关」。

做法上的约束：

- 同脸：不透明搬运 `encrypted_content`，不解码，不写入 loss detail。`id` 与 `created_by` 若出现，同脸保留；本轮没在输入 schema 里见到 `created_by` 时，不要为了对称去造。
- 不要复用 Anthropic `CompactionBlock` 的位置规则。那个块只能是 `messages[0]` 的第一块（N-AN-22）。Responses 的 compaction 项可以跟在保留下来的 user message 后面。硬套会把合法历史判成结构错误。
- 跨脸：一条 `unsupported-semantic`，不把密文写进 Anthropic `encrypted_content`，也不把 Anthropic 的可读 `content` 合成 OpenAI 密文。两边的密文都是本提供方的回放状态，另一家服务器不会认。
- 流式：只在 `output_item.done` 上取最终密文。`response.compaction.compacting` 按 web search 进度事件的方式记一条 loss，不发内容增量。
- `POST /responses/compact` 这个端点本身不进 oxa。oxa 不发 HTTP。要接的是出现在 `input[]` / `output[]` 里的项。调用方若把 `object: "response.compaction"` 整包当成普通 response 来解码，外层信封差在哪，本轮没有逐字段对过，不在这一刀里承诺。

来源：compact 方法参考、compaction 指南、streaming events、spec/11、spec/12 N-AN-22、Go decoder。

### 4.2 压缩触发器（同脸留住，先不跨脸折算）

这是三件不同的 wire，上次清单把它们收成「压缩上下文」一个角色。本轮把形状核清之后，第一刀应该拆开。

| Wire | 脸 | 规格今天 | 建议 |
|---|---|---|---|
| `context_management.edits[]` 里 `type: "compact_20260112"`，`trigger: {type: "input_tokens", value}`，可选 `pause_after_compaction`。beta `compact-2026-01-12`。 | Messages | spec/15 §3.3 只认 `clear_tool_uses_20250919` 与 `clear_thinking_20251015`。未知 edit type 整项 `unsupported-semantic`，不进 native。Go 的 default 分支 detail 为 `unsupported context edit type %q`，且不放进保留的 edits（`request_native.go`）。同脸回转会丢掉阈值压缩。 | 把这个 edit type 加入白名单，同脸原样回转。`pause_after_compaction` 是它的字段，不要丢了再套默认。 |
| 顶层 `compaction`：`type: "summarize"`，可选 `instructions`。beta `compact-2026-09-04`。请求的是「现在就总结，响应里只放 compaction 块」。 | Messages | 请求映射表没有这个名字。未知顶层字段按 spec/02 报 `unmapped-field` 并丢掉。响应里的 compaction **块** 3.0.0 已经回转。 | 请求参数同脸 native 保留。没有这个参数，网关会把一次压缩请求变成一次普通补全。 |
| `context_management: [{type: "compaction", compact_threshold}]`。`compact_threshold` 最小 1000。reference 未标 beta。 | Responses | spec/11 映射表没有 `context_management`。 | 同脸 native 保留。不要映射成 Anthropic 的 edit。 |

明确不要在第一刀做的折算：

- `clear_tool_uses_*` / `clear_thinking_*` 不是压缩。Responses 没有同名落点，继续记 loss。
- Anthropic 的 token 触发器和 Responses 的 `compact_threshold` 都是「超过 N 个 token 就压缩」，但暂停、回放位置、beta 都不同。要收成同一个 IR 角色，单独评审；在那之前跨脸记 loss，不合成另一家的触发器。
- 按需 `compaction.instructions` 是替换默认总结提示词的字符串。Responses 的触发器没有这个字段。不能把提示词塞进 `compact_threshold`。

来源：compaction 总览、compaction-threshold 请求体、beta Messages schema、Responses create reference、spec/15 §3.3、spec/11 §3。

### 4.3 显式缓存断点（跨脸，TTL 不对齐）

两边现在都有「在这块内容上放一个显式断点」：

- Anthropic `cache_control`：`type` 必须是 `"ephemeral"`，`ttl` 只允许 `"5m"` 与 `"1h"`（spec/15 §3.2）。
- OpenAI `prompt_cache_breakpoint`：对象，`mode` 只允许 `"explicit"`，`ttl` 只允许 `"30m"` 与 `"12h"`（spec/15 §3.7）。宿主是请求侧的 text / image / document / system 块。

没有任何一个 TTL 值两边都能写。所以可共享的角色只是「此处有显式断点」。TTL、Anthropic 的 `type`、OpenAI 的 `mode` 跨脸都记 loss，不把 `"1h"` 折成 `"12h"` 或 `"30m"`。

对不上、继续记 loss 的部分：

- Anthropic 打在工具声明、顶层请求、以及 OpenAI 断点宿主名单之外的块上的 `cache_control`。
- OpenAI 的 `prompt_cache_key` 和 `prompt_cache_options`（请求级，`ttl` 只允许 `"30m"`）。它们不是断点位置。2.5.0 已经在两张 OpenAI 脸之间回转，编到 Anthropic 继续 `unmapped-field`。

spec/15 §3.2 与 §4 仍写：编到 CC / Responses 时 detail 为 `anthropic-native cache breakpoint; the target face has no cache_control representation`。这句话在 2.5.0 引入 `prompt_cache_breakpoint` 之后是错的。行为上的 loss 还在，因为两个注解没有接成同一个角色。改这句话要和映射一起做，不要单独改 detail：spec/21 §8 禁止调用方解析 detail，但改契约句子仍应跟向量一起落地。

这是行为变更（今天丢失的断点，明天会出现在另一张脸上），按 minor，并在 changelog 里写迁移说明。

来源：spec/15 §3.2、§3.7、§4；上次清单第 4 节已指出这句话过时，3.2.0 仍未改。

### 4.4 `logprobs` / `top_logprobs`（OpenAI 家族）

spec/10 请求表写明这两项是 `unmapped-field`，loss catalog 的理由是 log-probability sampling 没有 IR 等价物。Responses create 另有 `top_logprobs`；未点名的字段按 spec/02 报损。compact 参考页的示例里，`output_text` 可以带 `logprobs` 数组（`token`、`bytes`、`logprob`、`top_logprobs`）。本轮没有把 Chat 与 Responses 的响应侧 schema 逐字段对完，所以这一刀的范围应先定成：两张 OpenAI 脸的请求开关，加上已经在参考示例里出现的响应数组；Anthropic 记 loss。不要改文本块的必填形状，不要为了 logprobs 解析文本。

优先级低于 4.1–4.3。它不破坏多轮回放。

### 4.5 原生工具的调用和结果（单独评审）

spec/16 只覆盖带 `name`（或 Responses 的 `server_label` / 已知内置 type）的 `tools[]` 声明。原文写明：不新增 `server_tool_use`、`advisor_tool_result`、响应、流式或历史映射；无 `name` 的 toolset 不在范围内。spec/20 对 Anthropic `server_tool_use` 与 provider-hosted tool 是一条 `unsupported-semantic`，子事件被吸收。

声明侧因此已经能跟上带日期的新 type：`web_search_20260209`、`code_execution_20260521` 这类只要仍是带 `name` 的声明，就在 `definition` 里原样回转，不需要为每个日期加白名单（N-NT-2）。缺的是调用和结果。`web_search_call` 是唯一已经单开的结果块，而且规格禁止把它改写成 `server_tool_use`。

建议仍是一次一个角色，先同脸：

- 第一角色是 web search 的历史。Responses 已有 `WebSearchCallBlock`。Anthropic 侧是另一套块，不要合成。
- `computer_toolset_20260801` 若声明里没有 `name`，继续落在 spec/16 的排除项里，不要为了 Fable 5.2 的 400 行为去加模型能力表。spec/00 禁止模型知识；2026-11-09 的发布说明是服务器按模型拒绝旧工具，不是新的消息形状。
- Advisor 的结果块维持 spec/16 的排除，直到单独评审。2026-11-09 只是收紧了 advisor 的 `model` 配对，没有新的结果字段。

## 5. 同脸 native 保留，但不进前几刀

这些字段官方有，规格映射表没有，同脸回转会按未知字段丢掉。它们没有第二张脸的落点，也不是回放状态。做成 native annotation（与 `top_k` 同一模式）即可，不要造另一张脸的字段。

| 字段 | 脸 | beta | 为什么靠后 |
|---|---|---|---|
| `speed`: `"standard"` \| `"fast"` | Messages | 当前模型（含 2026-11-09 的 Fable 5.2）不需要 beta header。Opus 4.8 的 fast 将于 2026-11-30 移除，那是服务器行为。 | 生成速度，不是内容。上次清单已标成单脸。 |
| `output_config.task_budget` | Messages | `task-budgets-2026-03-13` | 给模型看的 token 预算。beta schema 写 `total` 最小 1024；另一份指南写最小 20000。本轮不裁定哪个是服务器真正执行的下限。 |
| `messages[].clear_at`、消息级 `output_config` | Messages | `clear_at` 对应 `mid-conversation-system-clear-at-2026-08-21`；消息级 effort 对应 `mid-conversation-output-config-2026-07-01` | 规格已经点名并记 `unmapped-field`（N-AN-22）。补的是存储，不是发现。 |
| `moderation`、`service_tier`、`safety_identifier` | CC 与 Responses | reference 未标 | 上次清单已列为低于写明 loss 的项。本轮没有新的跨脸角色。 |

`reasoning` 上除 `effort` 以外的属性（含 summary 偏好）仍按 N-R-13 记 `unmapped-field`。它和 `thinking.display` 是不是同一角色，上次标成待定。本轮没有新的一手证据，不推翻。

## 6. 明确不做

| 项 | 它是什么 | 为什么不进 |
|---|---|---|
| Messages `thread` | `{"type":"create"}` 或 `{"type":"continue","previous_message_id"}`。beta `message-threads-2026-08-12`。2026-11-02 发布。后续请求只送新消息，线程在最后一次请求 5 分钟后过期。 | 服务端会话续接，同类于 Responses 的 `previous_response_id`。续接失败时重放全文是调用方的事。 |
| Responses `conversation`、`previous_response_id` | 服务端把历史接到这次请求上。 | 同上。不是可搬运的消息内容。 |
| `fallbacks` | beta `server-side-fallback-2026-07-01`。被安全策略拒绝时按顺序换模型；`"default"` 用服务器推荐的替身。 | 换模型重试是路由。spec/00 §2 禁止 failover。已接通的是 `stop_reason: refusal` 与 `stop_details`，不是这条重试链。 |
| `image.context_compression` | Responses 请求 `image.context_compression`：`"auto"` \| `"enabled"` \| `"disabled"`。响应里只回 `"enabled"` \| `"disabled"`。 | 图像压缩策略，不是可搬运的图像字节。上次已列为单脸。 |
| `access_programs` | Responses create 的顶层对象，选择组织被授权使用的访问程序。 | 授权范围，不是消息语义。不进转换层，也不在本文展开其子字段。 |
| 2026-11-09 的 preserved thinking | 改写更早的 `system`、`tools` 或消息后再回放 Fable / Mythos 5.2 的 thinking 块，服务器回 400。换模型时服务器丢掉这些块。 | 服务器对回放的约束。oxa 已经原样搬运 thinking 块。不新增字段，也不要在转换层替调用方改写历史。 |
| Agents API、Managed Agents、Realtime、Live、Images、WebSocket | 另一套端点或传输。OpenAI 2026-11-02 至 11-05 的 changelog 是 Live SIP、图像模型和 GPT-6-Cyber，不是这三张脸的新内容字段。 | 项目契约已排除。模型名走 modelmap 的恒等透传，不加能力表。 |
| Claude Code auto mode、Codex reviewer | 上次清单已排除。本轮发布说明没有把它们做成 Messages 或 Responses 的字段。 | 权限策略与审批执行。 |

## 7. 建议的提交切分

只排序。每刀仍走规格 → 向量 → 实现。不写字节级映射，不预支版本号。

1. Responses `compaction` 项的同脸回转（请求、响应、`output_item.done`）。新 IR 变体，五语言一起。不碰 Anthropic `CompactionBlock` 的位置规则。
2. Anthropic `compact_20260112` 加入 spec/15 白名单。与第 1 刀分开：一张脸的触发器，另一张脸的历史项。
3. Responses `context_management` 数组的同脸 native 保留。不映射到第 2 刀。
4. 按需压缩的顶层 `compaction` 参数，同脸 native 保留。响应块已经有了，这一刀只补请求。
5. 缓存断点跨脸。TTL 不折算。同时改掉 spec/15 §3.2 那句过时的 detail。这是行为变更。
6. `logprobs` / `top_logprobs`。先把响应侧 schema 对完再写规格。
7. web search 调用/结果的同脸历史。跨脸仍记 loss。Advisor、无 name 的 toolset、computer 的模型配对不进这一刀。

`speed`、`task_budget`、`clear_at`、消息级 `output_config` 可以插在任意两刀之间，每项一个 commit。它们不阻塞上面七步。
