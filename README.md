# 磁带载体退化事件处置服务

本项目为声像档案保存机构提供版本化 HTTP JSON API，将磁带载体退化事件从登记、原子批次抽样与圈定预检、稳定化方案双人审批、逐盘处理及中止恢复、可读性复验推进到跨轮次独立裁定与证据封存。每次业务变更均要求 `request_id`、操作者身份和 `expected_revision`，并在同一次原子提交中保存聚合修订、幂等结果及只追加摘要链。

服务不依赖外部系统。数据以单事件 JSON 文件保存在本地目录，写入采用临时文件同步和原子替换；每次加载都会校验修订连续性、审计前序摘要和当前快照摘要。事件封存后，业务写入会被拒绝，但事件查询、时间线和档案校验保持可用。

## 构建、运行和测试

要求 Go 1.22 或更高版本。

```text
go build ./...
go test ./...
go run ./cmd/server -addr=127.0.0.1:19091 -data-dir=./data
```

默认监听 `127.0.0.1:19091`，不会绑定通配地址。可通过 `-addr=127.0.0.1:<port>` 指定地址；也可设置 `PORT`，此时绑定 `127.0.0.1:<PORT>`。`-addr` 优先于 `PORT`。

运行有界全流程自检：

```text
go run ./cmd/server -self-check -addr=127.0.0.1:19091
```

自检会使用临时数据目录启动真实 HTTP 服务，完成建档、幂等重放、修订冲突、批量抽样、圈定预检、方案审批、处理中止与恢复、复验、独立放行、封存清单和摘要校验，然后主动关闭服务并删除临时数据。

## API 约定

所有变更请求使用 `Content-Type: application/json`，请求体上限为 1 MiB。创建请求包含 `request_id` 和 `actor_id`；后续变更还包含 `expected_revision`。同一 `request_id` 在同一操作上重试会返回原命令结果，不会重复追加审计事件；将其用于不同操作会被拒绝。

错误响应采用以下结构，`correlation_id` 同时写入 `X-Correlation-ID` 响应头：

```json
{
  "error": {
    "code": "revision_conflict",
    "message": "expected_revision 与当前修订不一致",
    "correlation_id": "corr-...",
    "current_revision": 3
  }
}
```

主要路由如下：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `POST` | `/api/v1/incidents` | 登记退化事件并生成确定性唯一事件编号 |
| `GET` | `/api/v1/incidents/{incident_id}` | 查询当前事件快照 |
| `POST` | `/api/v1/incidents/{incident_id}/assessments` | 登记单盘抽样观察 |
| `POST` | `/api/v1/incidents/{incident_id}/assessments/batch` | 原子登记最多 100 条抽样观察，整批仅增加一次修订 |
| `POST` | `/api/v1/incidents/{incident_id}/boundary/preflight` | 只读检查候选清单的角色、未观察介质和预计影响边界 |
| `POST` | `/api/v1/incidents/{incident_id}/boundary` | 校验抽样完整性并冻结影响边界 |
| `POST` | `/api/v1/incidents/{incident_id}/plans` | 提交稳定化方案 |
| `POST` | `/api/v1/incidents/{incident_id}/plans/approval` | 由第二人员审批方案 |
| `POST` | `/api/v1/incidents/{incident_id}/treatments` | 创建逐盘进行中处理；仍兼容直接登记完整结果 |
| `POST` | `/api/v1/incidents/{incident_id}/treatments/{record_id}/interruptions` | 登记命中审批方案停止条件的中止事件 |
| `POST` | `/api/v1/incidents/{incident_id}/treatments/{record_id}/resume` | 闭环指定中止事件并恢复处理 |
| `POST` | `/api/v1/incidents/{incident_id}/treatments/{record_id}/complete` | 补齐偏差解释并完成处理 |
| `POST` | `/api/v1/incidents/{incident_id}/verifications` | 登记设备、校准与可读性指标并形成建议 |
| `POST` | `/api/v1/incidents/{incident_id}/decisions` | 签发独立放行、补治或报废裁定 |
| `POST` | `/api/v1/incidents/{incident_id}/seal` | 生成档案摘要并封存事件 |
| `GET` | `/api/v1/incidents/{incident_id}/timeline?cursor=0&limit=50` | 分页查询完整审计时间线 |
| `GET` | `/api/v1/incidents/{incident_id}/archive/verify` | 复核审计链和封存档案摘要 |
| `GET` | `/api/v1/incidents/{incident_id}/rounds/{round_number}` | 查询按介质编号排序的只读轮次证据 |
| `GET` | `/api/v1/incidents/{incident_id}/archive/manifest` | 获取已验证的规范封存清单，支持 `ETag` 和期望摘要 |

## 固定业务规则

抽样等级为 `0` 至 `3`。任一外观、气味、黏连或脱粉等级达到 `2`，四项累计达到 `4`，或出现驱动器污染时，该介质被判定为受影响。冻结边界前必须同时存在 `target` 和 `control` 样本。

复验在校准标识存在时按固定阈值形成建议：错误率不超过 `0.01` 且可读时长不少于 `60` 秒为 `pass`；错误率不超过 `0.05` 且存在可读时长为 `retreat`；其余为 `irrecoverable`。批次放行要求全部介质为 `pass`，报废要求存在 `irrecoverable`，其他不通过结果进入补治。

方案审批人必须不同于编制人。处理中止必须引用已审批停止条件；在风险处置和参数调整闭环前不得完成或复验。补治新方案通过 `retreat_decision_id` 关联上一轮补治裁定，提交时固化上一轮方案、处理、复验和裁定证据。最终复核员不得参与当前或历史任一轮处理事件，并必须确认职责分离、处理完整性、校准证据、复验完整性和审计复核五项证据。

规范档案清单只对 `sealed` 事件开放。响应的 `ETag` 等于 `archive_digest`；`If-None-Match` 匹配时返回 `304`。调用方可通过 `expected_digest` 查询参数或 `X-Expected-Archive-Digest` 请求头提供期望摘要，不匹配时冲突响应包含 `actual_digest`。
