# OpenAI Responses WebSocket 接入与 oxa 边界

**调查日期：** 2026-10-25

## TL;DR

- **有。** OpenAI 现在同时提供标准 Responses HTTP/SSE 流和 **Responses WebSocket mode**。WebSocket mode 连接到 `/v1/responses`，用 `response.create` 启动生成；连接内的每个 response 仍沿用现有 Responses streaming event model。它不是 Realtime API。
- **Realtime 是另一套接口。** 它连接 `/v1/realtime?model=...`，交换以 session、conversation、音频缓冲区等为核心的双向 client/server events；不能因为两边都传 JSON，或个别事件名相似，就把 Realtime 事件当成 Responses 事件。
- **oxa 做协议语义转换，不做 WebSocket/HTTP 接入。** 对 Responses WebSocket 消息，只要拿到的是兼容 oxa 支持范围的 **Responses server stream event JSON**，就可以在 WebSocket 库完成收包后交给 Responses decoder；连接、鉴权、WebSocket 帧、`response.create` / `response.steer`、lane 和控制事件由调用方处理。Realtime 事件不能直接交给该 decoder。
- **差异大小：** Responses WebSocket 相对 Responses SSE，核心 response 事件语义大体复用，主要增加持久双向连接、增量 continuation/steering 与并发 lane 管理，适配边界可控但不是零工作；Realtime 则是显著不同的语义协议，不是换个 framing 就能兼容。

## 官方一手证据：三种接入不要混为一谈

| 接入 | 官方端点 / 触发方式 | 事件与生命周期 |
|---|---|---|
| Responses HTTP | `POST https://api.openai.com/v1/responses`；设置 `stream: true` 使用 HTTP streaming / SSE。 | 一个 HTTP 请求产生一个 SSE response stream；官方示例包含 `response.created`、`response.output_text.delta`、`response.completed` 等 Responses 事件。[Create a model response](https://developers.openai.com/api/reference/resources/responses/methods/create)；[Streaming API responses](https://developers.openai.com/api/docs/guides/streaming-responses) |
| Responses WebSocket mode | 官方称其为 persistent Responses WebSocket，连接 `/v1/responses`；reference 中的 SDK 入口为 `client.responses.connect()`。官方 API reference 索引给出的完整地址为 `wss://api.openai.com/v1/responses`。[WebSocket mode](https://developers.openai.com/api/docs/guides/websocket-mode)；[Responses `connect()` reference](https://developers.openai.com/api/reference/python/resources/responses/methods/connect) | 客户端发送 `response.create` 开始一次 response；新输入可用 `previous_response_id` 继续，支持 `response.steer` 中途追加指令。**每个 response 内**沿用现有 Responses streaming event model；多 lane 的事件可能交错。连接最长 60 分钟，重连会清除连接本地缓存。WebSocket mode guide 没有规定原始帧与 JSON event 的一一对应/批量规则。 |
| Realtime WebSocket | `wss://api.openai.com/v1/realtime?model=...`。这是独立 Realtime endpoint，不是 Responses endpoint。[WebSockets](https://developers.openai.com/api/docs/guides/voice-websockets)；[Realtime API reference](https://developers.openai.com/api/reference/resources/realtime) | 双向 session/event 协议：client events 包括 `session.update`、`conversation.item.create`、`input_audio_buffer.append`、`response.create`；新连接自动收到 `session.created`，server events 还包含 session、conversation、response/audio 等生命周期。参见 [Realtime client events](https://developers.openai.com/api/reference/resources/realtime/client-events) 与 [server events](https://developers.openai.com/api/reference/resources/realtime/server-events)。Realtime guide 明确将“reasoning and tool backend to the Responses API”引向独立的 Responses WebSocket mode。 |

**重要区分：** OpenAI 的 Responses WebSocket mode 改的是接入与连接生命周期，官方明确说 response 内仍用既有 Responses streaming event model；HTTP `stream=true` 则是 HTTP + SSE。Realtime 不属于该 Responses streaming event model。Responses 和 Realtime 都可能出现 `response.create` 等看似相近的名字，但发送方向、事件 envelope、session 状态和含义不同，应按所属 API 的事件 schema 解释。[Responses WebSocket mode](https://developers.openai.com/api/docs/guides/websocket-mode)；[Realtime WebSockets](https://developers.openai.com/api/docs/guides/voice-websockets)

## oxa 当前处理的层级

用户指定的 `spec/00-overview.md`、`spec/11-openai-responses.md`、`spec/20-streaming.md` 在当前树中已分别演进/改名为 `spec/00-scope-and-architecture.md`、`spec/11-mapping-openai-responses.md`、`spec/20-streaming-semantics.md`。以下按当前文件引用。

- 项目契约把 oxa 定义为纯的、进程内协议转换库，明确不是 proxy、HTTP client、router，也不负责 socket 或上游请求。[`CLAUDE.md`](../../CLAUDE.md#L7-L10)；[`spec/00-scope-and-architecture.md`](../../spec/00-scope-and-architecture.md#L38-L49)
- 当前规范列出 CC、Responses、Anthropic 三个 face；转换路径为 face → IR → face。它支持 parsed provider event objects 的流式转换，而不是自行建立网络连接。[`spec/00-scope-and-architecture.md`](../../spec/00-scope-and-architecture.md#L12-L30)；[`spec/00-scope-and-architecture.md`](../../spec/00-scope-and-architecture.md#L78-L87)
- Streaming 规范要求 provider adapter 先把原生 payload 解码成 face 的 typed event，再调用 `Feed`；不得把 raw SSE bytes 直接当 IR event。`go/openai/responses` 的 `StreamDecoder.Feed` 接受 `StreamEvent`；`FeedJSON` 会解析 event JSON、审计字段后调用 `Feed`。[`spec/20-streaming-semantics.md`](../../spec/20-streaming-semantics.md#L20-L49)；[`go/openai/responses/streamin.go`](../../go/openai/responses/streamin.go#L70-L83)；[`go/openai/responses/rawjson.go`](../../go/openai/responses/rawjson.go#L85-L117)
- `go/sse` 是可选的 opaque SSE byte-framing adapter，只提取 `event` / `data`；不理解 JSON、provider semantics 或 IR。它不是 Responses face 本身，也不是 WebSocket adapter。[`go/sse/sse.go`](../../go/sse/sse.go#L1-L9)；[`spec/20-streaming-semantics.md`](../../spec/20-streaming-semantics.md#L287-L303)
- Go Responses event 类型涵盖 `response.created`、output item / content part、text / reasoning / function-call delta，以及 `response.completed` / `incomplete` / `failed` 等；decoder 按原生 item 生命周期校验并在终态后要求 `Flush`。当前代码树和规范没有 Realtime face。[`go/openai/responses/types.go`](../../go/openai/responses/types.go#L58-L79)；[`go/openai/responses/streamin.go`](../../go/openai/responses/streamin.go#L91-L120)；[`go/openai/responses/streamin.go`](../../go/openai/responses/streamin.go#L458-L465)；[`spec/00-scope-and-architecture.md`](../../spec/00-scope-and-architecture.md#L12-L19)

## 收到 WebSocket JSON 后能不能直接交给 oxa？

### Responses WebSocket mode：有条件地可以

**可以直接交给 JSON/event 解码层，但不是把 WebSocket 连接或任意消息交给 oxa。** WebSocket 实现先完成连接与消息接收；对服务端发出的、属于 Responses response stream 且其 JSON shape/lifecycle 落在 oxa 支持范围内的事件，可将 JSON bytes 交给 `responses.StreamDecoder.FeedJSON`，或先 decode 为 `responses.StreamEvent` 再调用 `Feed`。理由是 OpenAI 说明 WebSocket mode 中“events within each response follow the existing Responses streaming event model”，而 oxa 解码器正是接收 Responses streaming events。[OpenAI WebSocket mode](https://developers.openai.com/api/docs/guides/websocket-mode)；[`FeedJSON`](../../go/openai/responses/rawjson.go#L85-L117)；[`Feed`](../../go/openai/responses/streamin.go#L70-L83)

限制：

1. `response.create` / `response.steer` 是 WebSocket 客户端命令，不是要交给 Responses 输出流 decoder 的 server response event。lane、连接、鉴权、断线恢复及 steering 状态也不是 oxa 负责的语义转换层。
2. `FeedJSON` 不是任意 JSON 消费器：只有符合 Responses event schema 和当前 converter 支持的生命周期才会正常转换。未知事件可能成为 loss；生命周期错序、缺终态或标识不匹配会报结构错误。终态 `Feed` 后还需按 API 契约调用一次 `Flush`。[`StreamDecoder.Feed`](../../go/openai/responses/streamin.go#L74-L83)；[`StreamDecoder.Flush`](../../go/openai/responses/streamin.go#L654-L665)
3. 该结论适用于 **Responses WebSocket mode 每个 response 的服务端 stream event**，不代表 `go/sse` 能解 WebSocket 帧，也不代表 oxa 已实现 WebSocket 客户端。

### Realtime：不能直接交给当前 Responses decoder

不能。Realtime WebSocket 的 `session.created` / `session.update`、`conversation.item.*`、`input_audio_buffer.*`、音频/转录及其 response 生命周期属于 Realtime session/conversation schema；而 oxa Responses decoder 要求诸如 `response.created`、`response.output_item.added`、`response.output_text.delta`、`response.output_item.done` 和 Responses terminal events 的生命周期与字段。JSON 只是序列化格式相同，不会把这些语义、事件方向和状态机变成 Responses。若要支持 Realtime，需要单独定义 Realtime → IR / IR → Realtime 的映射与损失语义，或实现专门适配层；当前仓库没有这项 face 或映射规范。[Realtime client events](https://developers.openai.com/api/reference/resources/realtime/client-events)；[Realtime server events](https://developers.openai.com/api/reference/resources/realtime/server-events)；[`spec/20-streaming-semantics.md`](../../spec/20-streaming-semantics.md#L186-L220)

## HTTP/SSE 与 WebSocket 的差异究竟多大？

- **同一个 Responses API 的两个传输方式：差异主要在传输和会话编排，不在每个 response 的核心输出语义。** SSE 是 `stream=true` 的 HTTP 响应事件流；Responses WebSocket 是持久双向连接，可连续发送 `response.create`、以 `previous_response_id` 增量续接、使用 steering 和并发 lanes，事件因此可能交错。对 oxa 而言，若已把服务端消息边界解成 JSON event，已有 Responses semantic decoder 可复用；但外层需要调用方实现连接、命令、lane 路由、交错流隔离与生命周期管理。尤其多个 lane 的事件不可混在同一个 `StreamDecoder` 状态机里处理。[Streaming API responses](https://developers.openai.com/api/docs/guides/streaming-responses)；[WebSocket mode](https://developers.openai.com/api/docs/guides/websocket-mode)；[`StreamDecoder` 状态](../../go/openai/responses/streamin.go#L29-L53)
- **Responses 与 Realtime 的差异是语义层面的。** 即使二者都使用 WebSocket 文本 JSON，Realtime 是独立的 session/conversation、音频和双向控制事件模型，不是把 Responses SSE 的 event/data 包装成 WebSocket message。oxa 目前能复用的是 Responses WebSocket mode 的每-response 事件语义；不是 Realtime 语义。
- ** framing 层也不同，但 OpenAI Responses WebSocket guide 没有指定 JSON event 与 WebSocket frame/message 的精确一一对应或批量规则。** 不要据此臆造“一个 WebSocket frame 等于一个 SSE frame”。由 WebSocket 客户端读取文本消息，再按实际 SDK/API 消息定义处理 JSON；SSE 则由 SSE framing adapter 读 `event:` / `data:` 字段与空行边界，然后由 provider JSON decoder 解释数据。[`go/sse/sse.go`](../../go/sse/sse.go#L1-L9)；[Streaming semantics §6](../../spec/20-streaming-semantics.md#L287-L303)

## 取证边界与访问说明

- 官方 WebSocket mode guide 与 HTTP streaming guide 可直接读取，分别确认 `/v1/responses` 持久 WebSocket / response event model，以及 `stream=true` 的 HTTP+SSE。
- 官方 `client.responses.connect()` Python reference 可读取并确认它是 persistent Responses API WebSocket，但其页面摘录未展示完整 URL。resource-level `responses/methods/connect` reference 的直接抓取返回 404；OpenAI 官方 reference 搜索索引显示完整地址 `wss://api.openai.com/v1/responses`。因此完整 scheme/host 是按该一手 API reference 索引加上可直接读取 guide 的 `/v1/responses` path 记录；guide **没有**给出 WebSocket 原始 frame / message batching 细节。未用二手文章补足这些边界。
- `docs/research/` 现有调研文档采用日期前缀（例如 [`2026-09-04-next-stage-review.md`](2026-09-04-next-stage-review.md#L1-L5)），故本报告沿用该命名方式。
