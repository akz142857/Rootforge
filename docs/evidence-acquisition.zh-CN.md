# L1 Evidence Acquisition 方案

> **Don't collect logs. Acquire evidence.**

## 决策摘要

Rootforge 不建设持续接收、传输、索引和长期保存全部日志的系统。

L1 的正式名称为：

> **Evidence Acquisition — 事故证据访问层**

它负责两件事：

1. 接收低流量、高信号的事故触发。
2. Case 建立后，从授权数据源中按时间窗、实体、调查假设和预算获取有限证据。

原始日志仍由 Loki、Elasticsearch、journald、Docker logging driver 等源系统保存。Rootforge Case 只保存查询方法、选中的证据、来源引用、校验信息和截断状态。

## 产品边界

| 能力 | 日志系统 | Rootforge L1 |
| --- | --- | --- |
| 持续接收全部日志 | 是 | 否 |
| 日志传输、缓冲和重试 | 是 | 否 |
| 长期存储与全文索引 | 是 | 否 |
| 搜索界面、仪表盘和保留策略 | 是 | 否 |
| 按事故范围查询 | 提供查询能力 | 编排查询 |
| 从大量记录中选择调查证据 | 通常不是核心 | 是 |
| 与 Heap、部署和代码关联 | 通常有限 | 核心能力 |
| 保存 RCA 使用的证据血缘 | 通常不是核心 | 是 |

Rootforge 应复用成熟的 telemetry 管道。OpenTelemetry Collector 已经负责接收、处理和转发 telemetry；Grafana Alloy 也提供 logs、metrics、traces 和 profiles 的持续采集能力：

- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [Grafana Alloy](https://grafana.com/docs/alloy/latest/introduction/)

## 两条数据路径

### 1. Trigger Intake：持续、低流量

Trigger 只携带创建 Case 所需的少量元数据，不携带完整日志流。

典型来源：

- Prometheus / Alertmanager / Grafana 告警 webhook
- Docker `oom`、`die`、`restart` 等事件
- JVM OOM 通知
- 服务健康检查和错误率告警
- 人工创建 Incident

```json
{
  "type": "container.oom",
  "occurred_at": "2026-09-10T07:03:45Z",
  "environment": "production",
  "service": "wand-saas",
  "container": "eab26338f957",
  "node": "ai-a2"
}
```

Trigger 的职责是回答“是否应该创建或更新一个 Case”，不分析根因。

### 2. Evidence Retrieval：按 Case、按需、有预算

Case 建立后，ClayHarness 只能通过受控 Tool Gateway 请求 Rootforge 向 L1 提交 `EvidenceQuery`：

```text
Case INC-001
  -> service=wand-saas
  -> container=eab26338f957
  -> time=事故前 10 分钟至后 2 分钟
  -> levels=ERROR,WARN
  -> max_records=2,000
  -> max_bytes=2 MiB
```

证据不足时，再根据明确假设扩大查询：

```text
初始高信号证据
   -> 形成 RabbitMQ 消息积压假设
   -> 查询相关 queue / consumer / routing key
   -> 获取必要上下文和正常基线
   -> 验证或排除假设
   -> 停止取证
```

这叫渐进式取证。它不是只看 `ERROR`，而是只获取与当前事故和调查假设相关的证据。OOM、背压和配置错误经常只出现在 kernel event、`WARN`、普通部署记录或运行时对象状态里。

## 运行模式

Rootforge 支持五种模式，共用同一套 Evidence 接口。

### 模式 A：已有 Loki

Rootforge 使用 Loki HTTP API，不部署新的日志采集器：

```text
Alert
  -> 创建 Case
  -> 生成 LogQL + 时间窗 + limit
  -> /loki/api/v1/query_range
  -> 预算控制、去重和脱敏
  -> EvidenceBatch
```

Loki 的 `/loki/api/v1/query_range` 支持 LogQL、`start`、`end` 和 `limit`；`/loki/api/v1/tail` 可以用于短期实时跟踪：

- [Loki HTTP API](https://grafana.com/docs/loki/latest/reference/loki-http-api/)

首轮查询示例：

```logql
{environment="production", service="wand-saas", node="ai-a2"}
  |~ "(?i)(oom|outofmemory|exception|error|warn)"
```

Rootforge 保存 LogQL、查询时间窗、选中记录和源引用，不复制整个结果集。

### 模式 B：已有 Elasticsearch / ELK

Rootforge 直接连接 Elasticsearch Search API；Kibana 和 Logstash 不作为查询入口。

```text
EvidenceQuery
  -> Elasticsearch Query DSL
  -> 时间字段 + 业务实体过滤
  -> search_after 分页
  -> 必要时使用 PIT 保持多页视图一致
  -> EvidenceBatch
```

大量结果使用 `search_after`；PIT 用于保证分页读取时视图一致；Async Search 只用于耗时查询，不被视为实时订阅机制：

- [Elasticsearch Search API](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-search-2)
- [Elasticsearch Point in Time](https://www.elastic.co/guide/en/elasticsearch/reference/current/point-in-time-api.html)
- [Elasticsearch Async Search](https://www.elastic.co/guide/en/elasticsearch/reference/current/async-search-intro.html/)

### 模式 C：没有集中日志系统

Rootforge 通过受限的 Node Retriever 从相关节点本地按需取证：

```text
Rootforge Case
      -> Restricted Node Retriever
           |- Docker Engine API
           |- journalctl
           |- local GC logs
           `- Heap / Thread Dump metadata
```

Node Retriever 是不运行 LLM 的小型 Go 程序，只暴露预定义操作：

```text
GetContainerInspect
GetImageInspect
GetContainerLogs
GetDockerEvents
GetJournalEntries
GetFileMetadata
StreamBoundedFile
```

它不暴露：

```text
RunShell(command)
DockerAPI(anyRequest)
ReadFile(anyPath)
```

节点本地日志充当短期环形缓冲。journald 可以通过 `SystemMaxUse` 和 `MaxRetentionSec` 控制容量与保留时间：

- [journald.conf](https://www.freedesktop.org/software/systemd/man/252/journald.conf.html)

这个模式适合节点不多、事故发现较快的 Docker / Swarm 环境。它的明确限制是：如果日志已经轮转、容器已经删除或节点已经损坏，证据可能无法恢复。

### 模式 D：没有集中日志系统，但需要可靠历史

Rootforge 不自行实现日志平台，而是提供可选的最小观测栈模板：

```text
每个节点
Grafana Alloy / OpenTelemetry Collector
                -> Loki 短期存储
                -> Rootforge Loki Connector
```

Rootforge 可以提供 Docker Compose、Swarm Stack、Helm 和标签规范示例，但 Collector 与 Loki 保持独立、可替换、可不安装。

建议初始场景只保留近期数据，例如 24～72 小时；实际容量和保留期由日志量、合规要求和节点资源决定，不写死在 Rootforge 核心中。

### 模式 E：离线文件导入

为隔离网络或人工调查保留文件入口：

```bash
rootforge evidence import \
  --case INC-001 \
  --file application.log \
  --from 2026-09-10T06:50:00Z \
  --to 2026-09-10T07:10:00Z
```

Heap dump、thread dump、GC log 和人工导出的应用日志都使用这一入口。

## Query 与 Follow

每个 Source 可以声明自己支持的能力：

```go
type SourceCapabilities struct {
	HistoricalQuery bool
	LiveFollow      bool
	StableReference bool
	ServerSideLimit bool
}

type EvidenceSource interface {
	Capabilities(context.Context) (SourceCapabilities, error)
	Query(context.Context, EvidenceQuery) (EvidenceStream, error)
	Follow(context.Context, FollowQuery) (EvidenceStream, error)
}
```

`Query` 用于有边界的历史取证。

`Follow` 只能用于活跃 Case 的短期观察，并且必须包含：

- `case_id`
- 服务、节点、容器等实体范围
- 查询条件
- 最大持续时间与空闲超时
- 最大记录数与最大字节数
- 明确的取消机制

Loki Source 可以用 `/tail`；Elasticsearch Source 可以按时间戳与稳定唯一键使用 `search_after` 做有限周期轮询。

禁止创建不属于任何 Case、没有 TTL 或没有容量预算的常驻 Follow。

## 统一数据契约

### EvidenceQuery

```go
type EvidenceQuery struct {
	CaseID     string
	SourceID   string
	From       time.Time
	To         time.Time

	Services   []string
	Nodes      []string
	Containers []string
	Severities []string
	Terms      []string

	MaxRecords int
	MaxBytes   int64
	Timeout    time.Duration
}
```

任何查询都必须绑定 Case、实体范围、时间范围和预算。Source Connector 还需要把 Rootforge 的通用条件编译成 LogQL、Elasticsearch Query DSL、journalctl 参数或 Docker API 参数。

### LogRecord

```go
type LogRecord struct {
	Timestamp   time.Time
	ObservedAt  time.Time
	Source      string
	Environment string
	Service     string
	Node        string
	Container   string
	Severity    string
	Stream      string
	Body        string
	Attributes  map[string]string
	SourceRef   string
	Fingerprint string
}
```

原始字段保存在受控属性中；跨 Source 的公共字段用于关联、排序、去重和调查。

### EvidenceBatch

```go
type EvidenceBatch struct {
	Records     []LogRecord
	Matched     int64
	Selected    int
	Bytes       int64
	Truncated   bool
	NextCursor  string
	SourceQuery string
	SourceRef   string
	Redactions  int
	Warnings    []string
}
```

`Truncated`、`Warnings` 和 `NextCursor` 不能省略。调查层必须知道证据不完整，不能把达到预算上限误认为“没有更多异常”。

## Case 保存策略

默认保存：

- 完整查询条件及其规范化形式
- 选中的关键记录和必要上下文
- 数据源、查询时间和 Connector 版本
- 原始结果引用或可继续查询的游标
- 内容指纹与去重信息
- 截断、缺失、错误和部分成功状态
- 脱敏规则版本和处理数量

默认不保存：

- 整个索引或整个日志文件
- 与 Case 无关的服务和时间段
- 不受容量限制的查询响应
- 数据源访问凭证
- 已被脱敏的原始 Secret

当 Source 不支持稳定引用，Rootforge 可以保存一份受预算和保留期限制的证据快照。

## 预算与安全

建议初始默认值：

| 项目 | 默认值 |
| --- | ---: |
| 初始时间窗 | 事故前 10 分钟至后 2 分钟 |
| 初始记录上限 | 2,000 条 |
| 初始字节上限 | 2 MiB |
| 单次查询硬上限 | 10,000 条或 10 MiB |
| 单 Case 日志证据上限 | 50 MiB，不含 Heap Dump |
| Follow 默认 TTL | 10 分钟 |

这些值允许按部署覆盖，但不能取消硬上限。

日志可能包含 token、密码和业务数据。Connector 应尽可能只请求必要字段；脱敏应在内容进入 Case 前完成：

- [Grafana Alloy access and permissions](https://grafana.com/docs/grafana-cloud/observe-and-act/send-data/alloy/access_permissions/)

访问 Loki、Elasticsearch 等系统时使用只读、最小范围的服务身份。Node Retriever 不将 Docker socket 或任意 Shell 能力暴露给 ClayHarness；这些凭证只存在于 Rootforge 管理的执行边界内。

## Go 包边界建议

```text
internal/evidence/
├── model/              # Query、Record、Batch、Capabilities
├── budget/             # 时间、记录、字节、超时限制
├── redact/             # Secret 和敏感字段处理
├── source/
│   ├── file/
│   ├── docker/
│   ├── journald/
│   ├── loki/
│   └── elasticsearch/
└── trigger/
    ├── manual/
    ├── webhook/
    └── docker/
```

Source 只负责访问、规范化和报告完整性。语义推理、根因排序和源码修改不进入 L1。

## 实现顺序

1. 定义 `EvidenceSource`、`EvidenceQuery`、`LogRecord` 和 `EvidenceBatch`。
2. 实现 Budget 与 Redaction 的公共包装器。
3. 实现 `FileSource`，使用现有 OOM 材料做测试。
4. 实现 `DockerSource` 和 `JournaldSource`。
5. 实现 `LokiSource`。
6. 实现 `ElasticsearchSource`。
7. 最后增加 Case 级、带 TTL 的 `Follow`。

## v0.1 验收标准

- 同一个 EvidenceQuery 能在 File、Docker、journald 和 Loki Source 上得到统一结构。
- 每次查询都必须有时间、实体、记录数、字节数和超时边界。
- 超过限制时停止读取并返回明确的 `Truncated` 状态。
- 原始证据或其稳定引用可以从 RCA 追溯。
- Evidence Source 失败可以产生部分结果和明确 Warning，不伪装成完整证据。
- 没有日志平台时，可以从一个 Docker Swarm 节点生成有界 OOM 证据包。
- 有 Loki 时，不安装 Rootforge 节点日志 Agent 也能完成同样的 Case 输入。
- Rootforge 不提供日志长期存储、全文索引、搜索 UI 或 telemetry 转发管道。

## 最终定位

> **有日志平台时，Rootforge 查询它；没有日志平台时，Rootforge 从节点本地按需取证；需要可靠历史时，Rootforge提供成熟日志基础设施的部署模板，但不自己成为日志基础设施。**
