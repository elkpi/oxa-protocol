# 2026-11-08 控制面向量覆盖盘点（v3.1.0 实测）

背景：一份外部 interoperability 提议请求 OXA 提供显式的 control-plane
fidelity contract（3×3 能力矩阵、loss→caller-action 映射、流生命周期文档、
6 组最小 fixture）。评估结论（已确认）：不引入 control-plane 专用机制，
重构为「保真度文档 + 向量锁定补缺」——spec/21（since 3.2.0）承载矩阵与
承诺，本文档记录定级证据与向量缺口，作为 Phase 2 补缺的验收对照。

## 1. 探针方法

临时 Go 探针（工作单元内已删除仓库引用，代码快照见本文末尾附录）对两个
合成控制面 fixture 跑全部 9 个请求方向（decode(source) → IR →
encode(target)），逐方向收集结构错误、`(reason | path | field)` loss 列表、
以及目标输出中关键字段的存活探针（指令位置、schema name/strict、
tool_choice、预算、工具调用/结果深度搜索）。

- Fixture `structured`：特权指令 + JSON Schema（name+strict）+ `tool_choice=none` + 预算 256。
- Fixture `tool-loop`：特权指令 + 一个 function 工具声明 + 一轮完成的调用与结果 + 预算 256。
- 语义清单、定级与 loss 锚点的规范落地见 `spec/21-capability-matrix.md`（§3/§4）。

## 2. 请求方向实测（v3.1.0 Go）

### structured fixture

| 方向 | 结构错误 | loss（encode 侧） | 关键字段 |
|---|---|---|---|
| R→R | 无 | 0 | instructions ✓、name/strict ✓、none ✓、预算 ✓ |
| R→C | 无 | 0 | instructions→system ✓、name/strict ✓、none ✓ |
| R→A | 无 | `unmapped-field \| params.response_format.name`；`unmapped-field \| params.response_format.strict` | system ✓、schema 存活但无 name/strict |
| C→R | 无 | 0 | developer item ✓、name/strict ✓、none ✓ |
| C→C | 无 | 0 | developer 保留 ✓ |
| C→A | 无 | `degraded \| system[0].role \| role`；两条 name/strict `unmapped-field` | system ✓、无 name/strict |
| A→R | 无 | `unsupported-semantic \| params.response_format \| name` | **format 整格省略**（N-R-14） |
| A→C | 无 | `unsupported-semantic \| params.response_format \| name` | **format 整格省略**（N-CC-13） |
| A→A | 无 | 0 | 全保留 |

R→C、C→R 的 0 loss 与提议观察一致；R→A、C→A 的 name/strict 报损与提议
观察一致。A→{R,C} 的整格式 unsupported 为提议未覆盖的实测确认，与
`cross/nonstream/anthropic-to-{responses,chatcompletions}-unnamed-schema-request.json`
两个既有向量完全一致（已有锁定）。

### tool-loop fixture

9 个方向全部无结构错误；工具调用与结果、预算在全部方向存活（含
type 值深度搜索）。唯一 loss：C→A 的 `degraded | system[0].role | role`
（Chat developer 消息无 Anthropic tier）。其余 8 方向 0 loss。

### stream 标志实测（对照提议）

- CC：`stream` 经 N-NAT-2 `kind:"stream"` native 保留（同面还原、跨面 0 loss）。
- RE / AN：decode 报 `unmapped-field | stream | stream`（含同面 round-trip），
  由 `anthropic/nonstream/output-format-streaming-request-to-ir.json` 锁定。
- 三面不对称已在 spec/21 §7 声明为有意行为；streaming 与否由调用方选择的
  转换器 API 决定。

## 3. 向量覆盖盘点（对照提议 6 组 fixture）

| # | 提议 fixture | 现有覆盖 | 结论 |
|---|---|---|---|
| 1 | 特权指令 + JSON Schema + `tool_choice=none`（每源面） | 指令与 schema 各自覆盖（`responses-developer-*`、`*-json-schema-request`、`*-unnamed-schema-request`）；`tool_choice=none` 仅 CC/AN face 内部（`tool-choice-none-{to,from}-ir`），RE face 内部与全部跨面无 | **缺口：跨面组合向量；RE 面内部 none 向量** |
| 2 | function 工具循环（一次调用+结果） | 7 个 cross 请求向量含工具历史（`*-request.json` 系列） | 已覆盖；C→A developer 降级锚点缺向量 |
| 3 | 流式文本分片跨面 | `cross/stream` 为空；face 内部流向量覆盖本面 | **缺口：跨面流向量（需 vectest 框架支持）** |
| 4 | 流式工具参数分片跨面 | 同上；face 内部 M7 向量已有（`m7-*`） | **缺口：跨面流向量** |
| 5 | usage-only 终止（CC） | CC 流向量含 `finish_reason:null` 中间块与 usage（`m7-tool-calls-*`、`usage-int64-to-ir`） | 部分覆盖；补专用 usage-only 尾块向量 |
| 6 | 故意 unsupported 语义不静默丢弃 | `documents`、`native-context-*`、`loss-unknown-*`、`unnamed-schema-request` ×2 等 | 已覆盖 |

### degraded/unsupported 格锁定状态（spec/21 §4 锚点）

- C→A 结构化 degraded（name/strict）：`chatcompletions-to-anthropic-json-schema-request.json` ✓
- R→A 结构化 degraded（name/strict）：**缺**
- C→A privileged instruction degraded（`system[0].role`，developer 消息）：**缺**
- A→{R,C} unsupported：`anthropic-to-{responses,chatcompletions}-unnamed-schema-request.json` ✓
- 其余 D/U 格：无（矩阵中不存在其他 D/U 格）
- `none` 跨面（P 格但无锁定）：**缺**
- 跨面流（§6 契约）：**全缺**（`cross/stream` 目录不存在）

## 4. Phase 2 工作清单（已完成，2026-11-08）

按 spec/21 §9 的完成定义，全部落地于本分支（Go 参考实现全套测试通过，manifest 609 向量）：

1. **框架**：vectest `RunCrossStream`（TDD：先失败测试后实现；框架自带向量间 `ResetStreamVector` 隔离——首个 red 即暴露此真实缺陷）；`cross/stream/` 由 `TestCrossVectorsStream` 九对执行。
2. **请求缺口向量（10 个）**：`responses-to-anthropic-json-schema-request`（R→A name/description/strict degraded）、`chatcompletions-to-anthropic-developer-json-schema-request`（C→A `system[0].role` + schema 降级）、6 个 `tool-choice-none` 跨面向量、RE 面内部 `tool-choice-none-{to,from}-ir`。
3. **跨面流向量（10 个）**：9 对 `*-to-*-multi-delta-text`（含同面 round-trip 的信封保真断言与跨面 item id 重建断言）、`chatcompletions-to-anthropic-tool-arguments`（交错分片）、`chatcompletions-to-anthropic-usage-only-terminal`。
4. **实现侧结论**：补向量过程中无一处需要修改转换实现——所有差异均为期望值构造错误（RE/AN/CC 的 wire 渲染默认：单文本消息 content 字符串简写、model 恒等透传、AN 必填 max_tokens 缺省合成 4096 + degraded loss），与 spec/21 矩阵定级一致。补向量新锁定的规范细节已由向量本身承载，未新增 spec 规则。

### v3.1.0 → 3.2.0 向量增量

- `vectors/cross/stream/`：0 → 10（新目录、新执行框架）。
- `vectors/cross/nonstream/`：+8。
- `vectors/responses/nonstream/`：+2。
- 合计 589 → 609。

## 附录：探针运行方式

探针为一次性 Go 程序（git 忽略目录中的临时工件，运行后已清理）：
`go.mod` replace 至仓库 `go/`，对两个 fixture 逐一跑
`Decode(source) → Encode(target)` 并输出 JSON 结果。fixture 语义为合成
policy 字符串，无厂商提示词、无凭据、无网络访问。探针结论已全部转录进
§2 的表格与 spec/21 §4 的定级；等价的可复现断言由本分支新增的 golden
向量承载（见 §4）。
