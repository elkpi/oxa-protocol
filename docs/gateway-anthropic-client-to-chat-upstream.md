# 网关接入指南：Anthropic Messages 客户端 → Chat Completions 上游

Status: informational（非规范性）。本文把 [spec/21](../spec/21-capability-matrix.md) 的控制面保真度矩阵翻译成一个具体场景的操作指南：客户端说 Anthropic Messages 协议（Claude Code 类自动审批控制面），上游说 OpenAI Chat Completions 协议（如 DeepSeek）。oxa 只负责协议转换与保真度报告；客户端识别、路由、授权与审批决策全部是网关的职责。

## 1. 数据流全景

```text
客户端 (Messages JSON)
   │ 请求：anthropic.DecodeRequestJSON   [AN decode → IR]
   ▼
IR Request                            ← 检查 losses，做放行/降级/拒绝决策
   │ 请求：chatcompletions.EncodeRequest [IR → CC encode]
   ▼
上游 (Chat Completions JSON)
   ┆ HTTP + 上游流式 SSE
   ▼
上游响应流 (CC chunks)
   │ 响应流：chatcompletions StreamDecoder.Feed/Flush [CC → IR events]
   ▼
IR EventStream
   │ 响应流：anthropic StreamEncoder.Apply [IR events → AN events]
   ▼
客户端 (message_start / content_block_delta / message_delta / message_stop)
```

请求方向是非流式负载（`stream` 标志是传输意图，见 spec/21 §7）；流式转换发生在响应侧，两个流转换器都是增量式的，网关逐 chunk 转发即可。

## 2. 控制面语义映射（A→C 方向）

下表摘自 spec/21 §4 的 A→C 列，即「Messages 源 → Chat Completions 目标」：

| 控制面语义 | 定级 | 网关动作 |
|---|---|---|
| 特权指令（`system` / developer tier） | P（降级例外：C→A 方向 developer → Anthropic 是 D，见 §5） | 放行 |
| 强制工具选择 / `tool_choice` 具名 | P | 放行 |
| 禁用工具（`tool_choice: {"type": "none"}`） | P | 放行 |
| 工具循环历史（`tool_use` / `tool_result`） | P | 放行 |
| 输出预算（`max_tokens`） | P | 放行 |
| 结构化输出（`output_config.format`） | **U**：`unsupported-semantic \| params.response_format \| name` | **拒绝或改道**：Anthropic 的 format 无 name，Chat 的 `json_schema` 必须有非空 name 且禁止合成（N-CC-13）。格式会被整体省略——客户端将收到自由文本而非受 schema 约束的 JSON |
| `stream` 请求标志 | AN 侧 decode 报 `unmapped-field \| stream \| stream` | 忽略该条 loss；流式与否由网关调用哪个转换 API 决定 |

决策规则（spec/21 §2）：`degraded` 允许继续（调用方决定是否接受降级），`unsupported-semantic` 表示目标面无法承载该语义（通常应拒绝或改道）。判定只看 `reason + path + field`，不要解析 `detail` 文本。

## 3. 请求方向代码（Go 参考实现）

```go
import (
    "github.com/elkpi/oxa-protocol/go/v3/anthropic/messages"
    "github.com/elkpi/oxa-protocol/go/v3/ir"
    "github.com/elkpi/oxa-protocol/go/v3/openai/chatcompletions"
)

// AN wire → IR → CC wire；两条 loss 列表拼接后按 (path, field, reason) 决策。
func convertRequest(anWire []byte) ([]byte, []ir.Loss, error) {
    req, decodeLosses, err := messages.DecodeRequestJSON(anWire)
    if err != nil {
        return nil, nil, err // 结构性错误：请求本身不合法
    }
    out, encodeLosses, err := chatcompletions.EncodeRequest(req)
    if err != nil {
        return nil, nil, err
    }
    raw, err := json.Marshal(out)
    if err != nil {
        return nil, nil, err
    }
    return raw, append(append([]ir.Loss{}, decodeLosses...), encodeLosses...), nil
}

// 网关策略示例：结构化输出在 A→C 方向不可表示，直接拒绝。
func containsUnsupportedFormat(losses []ir.Loss) bool {
    for _, l := range losses {
        if l.Reason == ir.LossUnsupportedSemantic && l.Path == "params.response_format" {
            return true
        }
    }
    return false
}
```

## 4. 响应流方向代码

客户端自己解析流并依据文本做审批判断，因此 delta 顺序、文本拼接、usage 与终端事件必须保真——这些由 `cross/stream` 黄金向量锁定（`anthropic-to-chatcompletions-multi-delta-text` 等，共 9 个方向全覆盖，spec/21 §6）。

```go
func convertStream(ccChunk <-chan []byte, emit func(anEvent []byte) error) error {
    dec := chatcompletions.NewStreamDecoder()
    enc := messages.NewStreamEncoder()
    for {
        chunk, ok := <-ccChunk
        if !ok {
            break
        }
        events, err := dec.FeedJSON(chunk) // CC chunk → IR events
        if err != nil {
            return err
        }
        if err := emitIREvents(events, enc, emit); err != nil {
            return err
        }
    }
    flushed, err := dec.Flush() // 终端：MessageDelta + MessageDone
    if err != nil {
        return err
    }
    return emitIREvents(flushed, enc, emit)
}

func emitIREvents(events []ir.Event, enc *messages.StreamEncoder, emit func([]byte) error) error {
    for _, ev := range events {
        native, _, err := enc.Apply(ev) // IR event → AN events
        if err != nil {
            return err
        }
        for _, e := range native {
            if err := emit(e); err != nil {
                return err
            }
        }
    }
    return nil
}
```

流生命周期保真要点（spec/20 N-S-1，均由向量锁定）：

- 上游 `finish_reason` 决定 IR 终端；usage-only 尾块不会产生第二个终端。
- 客户端侧看到 `message_delta`（携带最终 usage 与 stop_reason）后紧跟 `message_stop`。
- chunk/item ID 由编码器重建，**不承诺跨面恒等**——网关不得向客户端转述上游 chunk id 作为关联标识。
- 解码器不会在上游流失败时虚构成功终端；上游中断时网关收到的是错误而非伪造的 `message_stop`。

## 5. 容易踩的三个坑

1. **反向方向（C→A）的 developer 指令是降级而非保真**：Chat `developer` 消息转 Anthropic 会落为 system 内容并报 `degraded | system[i].role`——特权指令的"developer 层级"在 Anthropic 侧不存在。反向接入时网关要按 D 决策，不要照搬本文的 P 结论。
2. **结构化输出方向不对称**：R→C 与 C→R 是 P（name/strict 全保真）；C→A 与 R→A 是 D（name/strict 被丢弃并报 `unmapped-field`）；A→C 与 A→R 是 U（Anthropic format 无 name，目标面禁止合成，整格式省略）。同一条业务请求换一个上游面，定级可能完全不同——每次接新上游先查 spec/21 §4 对应列。
3. **`stream` 标志的 loss 不代表语义丢失**：RE/AN 面对请求体 `stream` 标志按设计报 `unmapped-field`（spec/21 §7），流式与否只取决于网关调用 streaming 还是 non-streaming 转换器。

## 6. 用黄金向量做回归

网关升级 oxa 版本后，最便宜的回归是把本文场景对应的向量当作行为快照：

- 请求方向：`vectors/cross/nonstream/anthropic-to-chatcompletions-*.json`（含 tool-choice-none、budget、native-tools 等）；
- 响应流方向：`vectors/cross/stream/anthropic-to-chatcompletions-multi-delta-text.json`；
- 反向验证：`vectors/cross/nonstream/chatcompletions-to-anthropic-*.json` 与 `vectors/cross/stream/chatcompletions-to-anthropic-tool-arguments.json`。

`vectors/` 是行为唯一权威（spec/README source-of-truth 优先级第一条）；实现与向量不一致是向量赢。
