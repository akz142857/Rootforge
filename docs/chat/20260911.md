# 解释 Mono infra 含义

> [Mono infra含义](chatgpt-conversation://6aa236a5-e1b4-83ec-8111-5f8f9c843d2e)

<details><summary>2 previous messages</summary>

> 我先读取这段对话的最近上下文，确认你说的 “Mono infra” 是命名、仓库结构，还是架构语境。
>
> <details><summary>Called a tool</summary>
>
> Tool call
>
> Tool: read_thread
> Status: completed
>
> </details>
</details>

已接上这段对话的上下文：

- `Mono infra`：将 Terraform、Docker、Kubernetes、监控等基础设施代码集中在一个 Monorepo 管理。
- 项目已定名 **Rootforge**。
- 定位：开源 AI SRE Agent，连接生产环境、可观测性、Git 和源代码。
- v0.1 聚焦：**自动调查 Docker/Swarm OOM，并生成 RCA 报告**。
- 暂不自动修改生产环境，后续再扩展为生成补丁、测试和 GitHub PR。

可以从当前 `Rootforge` 仓库继续，下一步落地 README、架构、目录结构和 OOM MVP。

> &#x20;OK，先写READER.md ,in Find the root. Forge the fix.

<details><summary>6 previous messages</summary>

> 我按 `README.md` 来处理，并把 **“Find the root. Forge the fix.”** 作为项目主标语。先检查现有仓库内容和约定，再直接完成初版。
>
> <details><summary>Ran 2 commands</summary>
>
> - Ran `pwd && rg --files -g 'AGENTS.md' -g 'README*' -g '!node_modules' -g '!vendor' && git status --short && find . -maxdepth 2 -type f | sort | head -80`
> - Ran `git status --short; git rev-parse --show-toplevel; find . -maxdepth 3 -type f | sort | head -120`
>
> </details>
>
> 当前目录是空的，也还没有初始化 Git。我会先建立一份英文开源 README：明确项目仍处于早期阶段，聚焦 OOM 调查闭环，避免把尚未实现的能力写成现有功能。
>
> <details><summary>Created a file, explored a file</summary>
>
> - Wrote `./README.md` (+187 -0)
>
> <details><summary>Explored a file</summary>
>
> - Read `./README.md`
>
> </details>
>
> </details>
>
> README 初版已经完成，主线是“生产事故 → 运行证据 → 部署版本 → Git 提交 → 源代码 → RCA”，并明确 v0.1 只承诺 OOM 调查与报告。现在做最后一次呈现检查。
>
> <details><summary>Called a tool</summary>
>
> Tool call
>
> Tool: open_in_codex
> Status: completed
>
> </details>
</details>

已完成 [README.md](./README.md)。

内容包括：

- 主标语：**Find the root. Forge the fix.**
- Rootforge 的项目定位
- v0.1 OOM Investigator 工作流
- Collector 与中央 Investigator 架构
- Runtime → Image → Git Commit → Source Code 映射
- 三阶段路线图
- 安全与设计原则

当前目录尚未初始化 Git，许可证也暂未选定。

> 目录下我加入了 heapdump 这个是早上分析OOM的 软件

<details><summary>2 previous messages</summary>

> 明白，`heapdump` 可以作为 Rootforge 的第一个真实案例和能力基线。我先只读检查它的结构与现有功能，确认哪些模块能直接复用到 v0.1，不会改动其中代码。
>
> <details><summary>Explored 5 files, ran a command</summary>
>
> - Ran `find . -maxdepth 3 -type f | sort | head -240; du -sh ./* 2>/dev/null; find heapdump -maxdepth 3 -type f \( -name 'README*' -o -name 'AGENTS.md' -o -name 'package.json' -o -name 'pyproject.toml' -o -name 'requirements*.txt' -o -name 'go.mod' -o -name 'Cargo.toml' -o -name 'pom.xml' -o -name 'build.gradle*' \) -print`
>
> <details><summary>Explored 5 files</summary>
>
> - Read `./heapdump/analysis.md`
> - Read `./heapdump/rabbitmq-workpool-oom-report.md`
> - Read `./heapdump/hprofx-开源评估.md`
> - Read `./heapdump/hprofx/README.md`
> - Read `./heapdump/hprofx/go.mod`
>
> </details>
>
> </details>
</details>

明白，我已经检查过 `heapdump`。其中真正的软件是 **`hprofx`**：一个用 Go 编写、零依赖的 Java HPROF 分析器。

它非常适合作为 Rootforge 的第一个真实诊断引擎：

```text
Rootforge 发现 OOM
        ↓
收集 JVM heap dump
        ↓
hprofx 分析对象、引用链、线程和框架配置
        ↓
Rootforge 关联部署/logs/部署版本/源代码
        ↓
生成 RCA 和修复建议
```

早上的真实案例已经证明其价值：

- RabbitMQ `ConsumerWorkService` retained 约 810 MB
- 客户端积压 47,584 条消息、约 749 MB
- `ackMode=NONE` 导致 prefetch 无法形成有效反压
- 另有一次加载 114,212 行数据的次要内存问题

需要特别注意：目录中的报告包含内部队列名、包名、业务数据等生产信息。Rootforge 开源前不能直接提交这些内容；原始 `.hprof` 也应加入 `.gitignore`。建议以后把 `hprofx` 放到 `analyzers/jvm/hprofx`，作为 Rootforge 的首个“框架感知型分析器”。

> OK，那我们先制定一个产品定位蓝图，也就是我们解决什么

<details><summary>4 previous messages</summary>

> 我会把蓝图写成一份独立的中文产品文档，并在 README 中链接它。核心会先钉死三件事：目标用户、真正解决的断层、明确不做什么；v0.1 则用今天的 JVM/RabbitMQ OOM 作为验证场景。
>
> <details><summary>Created a file, edited a file</summary>
>
> - Wrote `./README.md` (+3 -0)
> - Wrote `./docs/product-blueprint.zh-CN.md` (+282 -0)
>
> </details>
>
> 蓝图已经落盘。它把 Rootforge 的核心从“AI 看日志”收敛成了一个更明确的产品命题：**建立 Runtime-to-Code 调查链，并交付可审查的 Incident Case**。我正在做最后一遍措辞和 Markdown 检查。
>
> <details><summary>Edited a file, explored a file, a search, ran a command</summary>
>
> - Ran `wc -l README.md docs/product-blueprint.zh-CN.md; rg -n '^#|Rootforge|Runtime-to-Code|v0.1|明确不做|成功标准|北极星' docs/product-blueprint.zh-CN.md README.md; git diff --no-index /dev/null docs/product-blueprint.zh-CN.md | sed -n '1,260p'`
>
> <details><summary>Explored a file, a search</summary>
>
> - Searched for `^#|Runtime-to-Code|明确不做|成功标准|北极星` in `product-blueprint.zh-CN.md`
> - Read `./docs/product-blueprint.zh-CN.md`
>
> </details>
>
> - Wrote `./docs/product-blueprint.zh-CN.md` (+1 -2)
>
> </details>
</details>

产品定位蓝图已完成：[product-blueprint.zh-CN.md](./docs/product-blueprint.zh-CN.md)

核心定位已经收敛为：

> **Rootforge 是一个开源、证据驱动的 AI 生产事故调查员：连接生产异常、实际部署版本、Git 提交与源代码，生成可验证的根因分析，并协助工程师形成修复。**

蓝图明确了：

- 解决的核心断层：`Production Runtime → Git Commit → Source Code`
- 目标用户与最终交付物
- `Detect → Collect → Correlate → Investigate → Report → Forge → Learn` 产品闭环
- v0.1：JVM OOM Investigator
- 基于今天 RabbitMQ OOM 的真实验证案例
- 产品边界与明确不做的事情
- `Collector / Case / Analyzer / Investigator` 四个核心组件
- 分阶段发展路径与北极星指标

[README.md](./README.md) 也已加入蓝图入口。

一句最关键的产品判断是：

> **普通工具告诉你什么对象很大；Rootforge 告诉你为什么会变大、哪个配置或代码造成了它，以及应该去哪里修。**

> 这套系统分几层？每层的定义和边界是什么

<details><summary>4 previous messages</summary>

> 我会把它收敛成“6 个纵向层 + 1 个横切控制面”。关键不是按技术栈分层，而是按责任和信任边界分层：越靠近生产越确定、越少推理；越靠近修复权限越严格。随后我会把定义写入独立架构文档。
>
> <details><summary>Created a file, edited 2 files</summary>
>
> - Wrote `./README.md` (+2 -1)
> - Wrote `./docs/product-blueprint.zh-CN.md` (+2 -0)
> - Wrote `./docs/system-layers.zh-CN.md` (+397 -0)
>
> </details>
>
> 分层文档已写好。现在我在检查层间契约是否闭合，尤其是三条硬边界：Collector 不诊断、Investigator 不写生产、Forge 不绕过审批。
>
> <details><summary>Ran a command</summary>
>
> - Ran `wc -l docs/system-layers.zh-CN.md; rg -n '^## |^### |\*\*这一层|v0.1|system-layers' README.md docs/product-blueprint.zh-CN.md docs/system-layers.zh-CN.md; test -f docs/product-blueprint.zh-CN.md && test -f docs/system-layers.zh-CN.md && echo 'linked documents exist'`
>
> Tool call
>
> Tool: open_in_codex
> Status: completed
>
> </details>
</details>

Rootforge 建议分为 **6 个纵向层 + 1 个横切控制面**：

| 层 | 定义 | 核心边界 |
|---|---|---|
| L1 Collect | 接收告警，采集日志、Heap Dump、容器和部署信息 | 只取得事实，不判断根因，不修改生产 |
| L2 Case & Evidence | 将一次事故组织成可复现、可审计的证据案件 | 保存“知道什么、从哪里知道”，不解释含义 |
| L3 Analyze | 用 `hprofx` 等确定性工具解析 Heap、线程和框架状态 | 产出结构化 Finding，不做开放式猜测 |
| L4 Correlate | 建立 `Runtime → Image → Build → Commit → Code` 映射 | 负责关联，不把时间相关性直接宣布为因果 |
| L5 Investigate | 提出假设、补充证据、排除候选并形成 RCA | 可以推理，但不能修改代码或生产环境 |
| L6 Forge | 生成候选补丁，在隔离环境测试，审批后创建 PR | 不绕过审批，v0.1 不自动部署生产 |
| Control Plane | 权限、密钥、脱敏、审批、审计和成本控制 | 贯穿所有层，任何 Agent 都不能绕过 |

关键层间产物是：

```text
Collector
    ↓ EvidenceEnvelope
Incident Case
    ↓ Evidence Snapshot
Analyzer
    ↓ Finding
Correlator
    ↓ ContextGraph + Timeline
Investigator
    ↓ RCA
Forge
    ↓ ChangeProposal + Tests + PR
```

三个最重要的硬边界：

1. **Collector 不诊断**：它只能有限、只读地采集事故现场。
2. **Investigator 不执行生产变更**：模型只能基于授权证据调查。
3. **Forge 不绕过人工审批**：写代码、推送和 PR 是新的权限边界。

v0.1 不需要把六层全部自动化，先走最短纵向闭环：

```text
人工导入 JVM OOM 材料
    → 建立 Incident Case
    → hprofx 结构化分析
    → 映射部署 Commit 和源码
    → 生成可追溯 RCA
```

完整定义已经写入 [系统分层与边界](./docs/system-layers.zh-CN.md)，并更新了 [产品定位蓝图](./docs/product-blueprint.zh-CN.md) 和 [README](./README.md)。

> 整套系统使用Golang 还是 rust 比较合适？

<details><summary>2 previous messages</summary>

> 我倾向先给出一个明确结论：**整套 Rootforge 以 Go 为主，不做 Go/Rust 双主栈；Rust 只保留给未来确有性能或安全必要的独立 Analyzer。** 我会结合当前 `hprofx` 的实际表现和两边最新生态，再把取舍讲清楚。
>
> <details><summary>Ran 2 commands, searched the web</summary>
>
> Searched the web for `site:kubernetes.io client-go Go client library ...`
>
> Searched the web for `site:rust-lang.org learn build reliable efficient software memory safety concurrency ...`
>
> - Ran `go version; go test ./...; find . -name '*_test.go' -print; wc -l *.go`
>
> - Ran `go test ./...`
>
> </details>
</details>

结论：**Rootforge 核心系统选择 Go。v0.1 不引入 Rust，不做双主语言。**

Rust 保留为未来高性能、强隔离 Analyzer 的可选实现，但不承担控制面和主业务编排。

| 维度 | Go | Rust | Rootforge 选择 |
|---|---|---|---|
| Docker / Kubernetes 集成 | 官方生态非常完整 | 主要依赖社区库 | Go |
| Collector / Agent Server | 并发、网络、CLI 很适合 | 能做，但开发复杂度更高 | Go |
| 快速验证产品 | 编译快、代码直接、维护成本低 | 类型与生命周期成本更高 | Go |
| HPROF 大文件解析 | 当前性能已经足够 | 极限性能和内存控制更强 | 先 Go |
| 内存安全与无 GC | 有 GC | 编译期内存安全、无 GC | 特殊 Analyzer 可用 Rust |
| OpenTelemetry | Trace/Metric 稳定，Logs RC | 主要信号仍为 Beta | Go |
| 开源贡献门槛 | Infra/SRE 开发者更容易参与 | 受众相对更窄 | Go |

最关键的三个理由：

1. Rootforge 本质上是 Infra/SRE 编排系统

大多数工作是：

```text
调用 Docker / Kubernetes API
读取日志和运行时材料
处理并发采集任务
调用 Git / CI / LLM API
组织 Incident Case
运行 Analyzer
生成报告与 PR
```

这是典型的 I/O 密集型基础设施软件，不是极限计算引擎。Go 的标准库、并发模型、静态二进制和部署方式非常契合。Go 官方也把 DevOps/SRE、CLI、云服务列为主要适用场景。[Go DevOps/SRE](https://go.dev/solutions/devops)

2. Go 与目标基础设施生态更贴近

Docker 官方提供 Go SDK，并支持 Engine API 版本协商。[Docker Engine API](https://docs.docker.com/reference/api/engine/)

Kubernetes 将 `client-go` 列为官方维护客户端，而 Rust 客户端位于社区维护列表。[Kubernetes Client Libraries](https://kubernetes.io/docs/reference/using-api/client-libraries/)

OpenTelemetry 当前状态也更有利于 Go：Go 的 Trace 和 Metrics 已稳定、Logs 为 RC；Rust 的主要信号仍为 Beta。[OpenTelemetry status](https://opentelemetry.io/status/)

3. 我们已经拥有可用的 Go 资产

现有 `hprofx`：

- 2,563 行 Go
- 零第三方依赖
- 当前能够正常编译
- 根据现有事故报告，1.1 GB、751 万对象的 Heap Dump 完整分析约 5 秒
- 已经实现 retained size、GC root path、线程栈和 Spring AMQP 感知分析

这说明 Go 当前没有形成实际性能瓶颈。现在重写 Rust 不会增加产品价值。

我刚运行了：

```text
go test ./...
? hprofx [no test files]
```

代码能够编译，但完全没有测试。因此当前更重要的工程工作是补充解析器测试、错误处理和真实样本覆盖，而不是更换语言。

建议的技术边界：

```text
Go
├── rootforge CLI
├── Collector
├── Incident Case
├── Docker / Kubernetes / Git integrations
├── Analyzer orchestration
├── Runtime-to-Code correlation
├── Investigator
├── Forge workflow
└── hprofx JVM Analyzer

未来可选 Rust
├── 超大 Heap Dump 高性能解析器
├── 低内存边缘 Collector
├── eBPF / 系统级探针
└── 处理不可信二进制输入的隔离 Analyzer
```

即使未来引入 Rust，也建议采用独立进程协议：

```text
Rootforge Go
    ↓ JSON / protobuf / gRPC
Rust Analyzer
    ↓ Finding
Incident Case
```

不要使用 Go/Rust FFI，也不要让 Rust 类型渗透进 Rootforge 核心领域模型。

最终技术决策可以定为：

> **Go-first, analyzer-agnostic.**
> Rootforge 的控制面、Collector 和默认 Analyzer 使用 Go；Analyzer 通过稳定协议接入，只有经过真实性能和安全数据证明后，才为个别 Analyzer 引入 Rust。

> OK，那就采用Go ,现在先构建L1 Collect 是否合理？也就是先看看日志如何收集

<details><summary>2 previous messages</summary>

> 合理，但我建议不是先做“通用日志平台”，而是先做一个很窄的纵向切片：**给定容器和事故时间窗，把 Docker 日志、容器元数据、镜像/部署信息打包成一个可追溯的 Case**。L1 开工前只需要先定义 L2 的最小 `EvidenceEnvelope`，否则采到的数据没有稳定出口。
>
> <details><summary>Searched the web</summary>
>
> Searched the web for `site:docs.docker.com/reference/api/engine container logs API timestamps stdout stderr ...`
>
> Searched the web for `site:docs.docker.com engine security protect docker daemon socket root equivalent access ...`
>
> </details>
</details>

合理。**L1 Collect 是正确起点，但第一版应收集“事故证据”，而不只是应用日志。**

OOM 很可能不会在应用日志中留下完整原因。我们至少需要：

```text
应用日志
+ Docker 容器状态
+ Docker OOM/die/restart 事件
+ 主机 kernel / journald 日志
+ 镜像、服务、任务和部署元数据
+ JVM Heap Dump / Thread Dump
```

## 推荐的第一个纵向切片

先实现一个手动 CLI：

```bash
rootforge collect docker \
  --container <container-id> \
  --incident-at 2026-09-10T07:03:45+08:00 \
  --before 30m \
  --after 10m \
  --output ./cases/INC-20260910-001
```

产出：

```text
cases/INC-20260910-001/
├── case.json
├── evidence/
│   ├── container-inspect.json
│   ├── image-inspect.json
│   ├── container-logs.jsonl
│   ├── docker-events.jsonl
│   ├── host-journal.jsonl
│   └── heapdump.hprof.ref.json
├── checksums.json
└── collection-report.json
```

这里先保存 `.hprof` 的引用、大小和校验值，不一定复制 1 GB 文件进入 Case。

## 日志如何收集

建议按下面的优先级设计：

### 1. Docker Engine API

收集：

- `ContainerInspect`
- `ImageInspect`
- `ContainerLogs`
- Docker events
- 容器退出码、OOMKilled、启动/结束时间
- Labels、镜像 digest、Swarm service/task/node 信息

Docker Logs API 原生支持 `since`、`until` 和时间戳，适合限定事故窗口；但只适用于兼容的 logging driver。[Docker Logs API](https://docs.docker.com/reference/api/engine/version/v1.46/)

### 2. 主机 journald

收集事故窗口内：

```text
kernel OOM killer
dockerd
containerd
systemd service
主机内存压力
```

例如内部可以运行：

```bash
journalctl \
  --since "2026-09-10 06:30:00" \
  --until "2026-09-10 07:15:00" \
  --output=json
```

`journalctl` 支持时间窗和结构化 JSON 输出，适合作为 Evidence 输入。[journalctl](https://www.freedesktop.org/software/systemd/man/255/journalctl.html)

### 3. Docker Swarm 管理信息

Manager 节点负责解析：

```text
service
  -> task
  -> node
  -> container
  -> image digest
```

`docker service logs` 能聚合服务或任务日志，但只能在 Manager 上运行，并且仅适用于 `json-file` 或 `journald` 驱动。[Docker service logs](https://docs.docker.com/reference/cli/docker/service/logs/)

因此它可以作为方便入口，但不能成为唯一采集路径。

### 4. 已有日志平台

如果以后已有 Loki、Elasticsearch 或其他日志系统，Rootforge 应通过 Connector 查询事故窗口，而不是重新长期存储日志。

```text
Loki / Elasticsearch
        ↓ bounded query
EvidenceEnvelope
        ↓
Incident Case
```

## 一个重要限制

Docker events 只保留最近 256 条，因此不能作为可靠的长期事故历史。[Docker events](https://docs.docker.com/reference/cli/docker/system/events/)

第一版可以在事故发生后立即采集；后续需要二选一：

- Collector 持续订阅事件并保存到中央系统
- 从已有监控/日志平台查询历史事件

## 安全边界

不能简单地把 `/var/run/docker.sock` 挂载给包含 LLM 的 Rootforge 主进程。

Docker 官方明确说明，获得 Docker daemon 控制权通常相当于获得主机 root 权限。[Docker Engine security](https://docs.docker.com/engine/security/)

建议：

```text
Production Host
┌──────────────────────────────┐
│ Restricted Collector         │
│ - 只允许预定义读取操作        │
│ - 不运行 LLM                 │
│ - 不接受任意 Docker API 请求  │
└──────────────┬───────────────┘
               ↓ Evidence
       Rootforge Investigator
```

v0.1 可以先使用人工执行的本地 CLI，暂时不部署常驻 Agent，从而降低风险。

## 开发顺序

建议这样落地：

1. 定义最小 `Case` 和 `EvidenceEnvelope` JSON Schema。
2. 实现本地 Docker Collector：inspect、logs、events。
3. 增加 Swarm service/task/node 映射。
4. 增加 journald/kernel OOM Collector。
5. 用今天的 RabbitMQ OOM 验证采集包是否足够。
6. 再接入 `hprofx`，进入 L3 Analyze。

第一阶段验收标准可以定为：

> **给定一个容器和事故时间，Rootforge 能用一条命令生成限定时间窗、来源可追溯、包含 Docker 与主机 OOM 证据的 Incident Case。**

所以答案是：**从 L1 开始合理，但先做“有边界的 OOM Evidence Collector”，不要先造一个通用日志采集系统。**

> 我有个担心，collect 可能会收集到大量的日志，如果系统接收这些日志 那就变成了日志系统，而且目前应该有非常多的收集开源项目，所以collect 的定位是什么，我理解是只看错误 ，当然也可能收集一些数量 ，所以这个问题需要讨论清楚

<details><summary>2 previous messages</summary>

> 这个担心非常准确，也暴露了当前命名的问题：**L1 如果理解成持续 Collect 日志，Rootforge 会错误地演变成日志管道和存储系统。** 我建议把它重新定义成“事故证据访问层”，并区分低流量的持续触发与按事故、按需取证。
>
> <details><summary>Searched the web</summary>
>
> Searched the web for `site:opentelemetry.io docs collector receive process export telemetry official ...`
>
> </details>
</details>

你的担心是对的。**如果 Rootforge 持续接收、传输、索引和保存全部日志，它就会变成另一个 Loki/ELK，而这不应该是我们的产品。**

我建议修正 L1 的定义：

> **L1 不是日志采集层，而是事故证据访问层。**

甚至可以把名字从 `Collect` 改为：

> **L1 Acquire — Evidence Access & Retrieval**

## 与日志系统的边界

| 能力 | 日志系统 | Rootforge L1 |
|---|---|---|
| 持续接收所有日志 | 是 | 否 |
| 日志传输与缓冲 | 是 | 否 |
| 长期存储与索引 | 是 | 否 |
| 全文搜索和仪表盘 | 是 | 否 |
| 按事故时间窗查询 | 提供查询能力 | 调用查询能力 |
| 从大量日志选择证据 | 一般不负责 | 是 |
| 与部署、Heap、代码关联 | 通常有限 | 核心能力 |
| 保存 RCA 使用的证据 | 不是重点 | 是 |

OpenTelemetry Collector 和 Grafana Alloy 已经能够持续接收、处理、转发 logs、metrics、traces，没有必要重复建设。[OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)、[Grafana Alloy](https://grafana.com/docs/alloy/latest/introduction/)

因此整体关系应该是：

```text
现有日志管道
Alloy / Fluent Bit / OTel Collector
               ↓
日志存储
Loki / Elasticsearch / CloudWatch / Journald
               ↓ 按需查询
Rootforge Evidence Access
               ↓ 有限证据
Incident Case
```

如果用户没有日志平台，Rootforge 才从 Docker 或 journald 直接读取，但仍然必须按事故、按时间窗、按容量限制。

## “只看错误”接近，但还不够准确

只读取 `ERROR` 会漏掉很多根因线索。例如这次 OOM：

- Kernel OOM 事件不是应用 `ERROR`
- RabbitMQ 消费者积压可能没有抛异常
- `ackMode=NONE` 是运行时配置，不是错误日志
- 内存开始上涨前可能只有普通部署日志
- 消费变慢前可能出现 `WARN`
- 正常日志可以提供请求量和消息类型作为上下文

所以更准确的原则是：

> **不是只看错误，而是只获取与当前事故和调查假设相关的证据。**

## 两条完全不同的数据路径

L1 应拆成两个子部分：

### 1. Trigger Intake：持续，但数据量极小

只接收高信号事件：

```text
Prometheus Alert
Docker oom event
Container exit 137
Restart loop
JVM OOM notification
HTTP 5xx alert
人工创建 Incident
```

每条只有少量元数据：

```json
{
  "type": "container.oom",
  "time": "2026-09-10T07:03:45Z",
  "service": "wand-saas",
  "container": "eab26338f957",
  "node": "ai-a2"
}
```

它负责决定“是否建立一个 Case”，不传输应用日志正文。

### 2. Evidence Retrieval：按事故触发、按需查询

Case 建立后，再执行有界查询：

```text
service = wand-saas
container = eab26338f957
time = OOM 前 15 分钟到后 5 分钟
levels = ERROR / WARN / selected INFO
max_lines = 5,000
max_bytes = 5 MB
```

第一批证据不足时，Investigator 可以申请第二轮查询：

```text
发现 RabbitMQ 消息积压
        ↓
仅查询相关 consumer、queue 和 routing key
        ↓
仍不获取其他服务或全天日志
```

这是一种**渐进式取证**：

```text
先取最小证据
    ↓
形成假设
    ↓
针对假设补充查询
    ↓
验证或排除
    ↓
停止取证
```

## Case 中保存什么

Case 不应该默认保存完整日志查询结果，只保存：

```text
查询条件
数据来源
选中的关键日志片段
上下文前后若干行
内容校验值
原始日志引用
是否被截断
下一页游标
脱敏状态
```

例如：

```json
{
  "source": "loki",
  "query": "{service=\"wand-saas\"} |= \"OutOfMemoryError\"",
  "from": "2026-09-10T06:48:45Z",
  "to": "2026-09-10T07:08:45Z",
  "matched": 83,
  "selected": 21,
  "stored_bytes": 48192,
  "truncated": false,
  "source_reference": "loki://production/query/..."
}
```

原则是：

> **原始日志留在日志系统，Rootforge Case 只保存调查使用的证据、查询方法和来源引用。**

如果数据源不支持稳定引用，可以在 Case 中保存一份受容量和保留期限制的证据快照。

## 必须设置的硬限制

每次证据查询都必须包含：

```text
Case ID
目标服务/容器/节点
开始和结束时间
最大记录数
最大字节数
允许的日志级别
允许的字段
脱敏策略
超时
```

建议首版默认值：

```text
初始时间窗：事故前 10 分钟、后 2 分钟
初始上限：2 MB 或 2,000 条
单次最大：10 MB 或 10,000 条
单 Case 默认上限：50 MB，不含 Heap Dump
超长单行：截断并保留原始引用
```

日志可能包含 token、密码和业务数据，脱敏应发生在内容进入 Case 之前。Grafana 的权限文档也明确提醒日志中可能携带 API key、token 和 password。[Grafana Alloy access and permissions](https://grafana.com/docs/grafana-cloud/observe-and-act/send-data/alloy/access_permissions/)

## v0.1 应该怎么做

暂时不开发常驻日志 Agent，也不接收实时日志流。

第一版只做三个 Connector：

```text
Docker Evidence Retriever
├── container inspect
├── image inspect
├── bounded container logs
└── relevant Docker events

Journald Evidence Retriever
├── kernel OOM
├── dockerd
└── bounded time window

File Evidence Importer
├── heap dump reference
├── thread dump
├── GC log
└── manually exported application logs
```

统一输出：

```text
EvidenceQuery
    ↓
EvidenceBatch
├── selected records
├── source references
├── provenance
├── truncation information
└── redaction information
```

## 最终定义

我建议把边界正式确定为：

> **Rootforge 不持续采集和存储可观测性数据。L1 负责接收低流量事故触发，并从现有数据源中按 Case、按时间窗、按调查假设，有预算地获取证据。**

一句更短的内部原则：

> **Don’t collect logs. Acquire evidence.**

因此，下一步不应该先写“日志 Agent”，而应该先定义：

1. `Trigger`
2. `EvidenceQuery`
3. `EvidenceBatch`
4. 容量与时间窗预算
5. Docker/Journald 的按需 Retriever

这会从架构上避免 Rootforge 变成日志平台。

> 是不过我们需要考虑
> 1、有成熟的日志系统比如ELK/loki 是否可以接入他们的API 订阅一个查询条件 拉取对应的日志进行分析
> 2、没有成熟日志系统，那么应该怎么做？

<details><summary>2 previous messages</summary>

> 这两个场景需要同时支持，但产品行为要不同：**有日志系统时做联邦查询；没有时做节点本地、短保留、按需取证。** 另外，“订阅查询”应只用于某个活跃 Case 的短期追踪；常驻订阅所有错误会再次把 Rootforge推向日志处理平台。
>
> <details><summary>Searched the web</summary>
>
> Searched the web for `site:grafana.com/docs/loki/latest/reference/loki-http-api query_range tail websocket official ...`
>
> </details>
</details>

这两个场景都必须支持，而且应该统一在同一个抽象下：

> **Rootforge 不拥有日志，只拥有对证据源的访问能力。**

建议将 L1 定义为 `Evidence Acquisition`，下面包含多个 `Evidence Source Connector`。

```text
Evidence Acquisition
├── Loki Connector
├── Elasticsearch Connector
├── Journald Connector
├── Docker Connector
└── File Import Connector
```

无论底层有没有日志平台，上层看到的都是统一的：

```text
EvidenceQuery -> EvidenceBatch
```

## 场景一：已有 ELK / Loki

这是最理想的工作模式。Rootforge 不再部署日志采集器，而是：

```text
Alert / 人工事件
        ↓
创建 Incident Case
        ↓
生成有限查询条件
        ↓
调用 Loki / Elasticsearch API
        ↓
流式筛选、去重、截断
        ↓
保存关键证据和原始查询引用
```

### Loki 接入

Loki 原生提供：

- `/loki/api/v1/query_range`：查询指定时间范围
- `/loki/api/v1/query`：即时查询
- `/loki/api/v1/tail`：通过 WebSocket 实时跟踪

`query_range` 支持 LogQL、开始时间、结束时间和结果数量限制，非常适合事故取证。[Loki HTTP API](https://grafana.com/docs/loki/latest/reference/loki-http-api/)

例如：

```text
Case:
  service = wand-saas
  incident_at = 07:03:45

Rootforge 生成:
  query = {service=\"wand-saas\", node=\"ai-a2\"}
          |~ \"(?i)(oom|outofmemory|exception|error|warn)\"
  start = 06:53:45
  end   = 07:08:45
  limit = 2000
```

但 Rootforge 不应只执行一次大查询。更合理的是分阶段查询：

```text
第一轮：OOM / ERROR / WARN，高信号日志
第二轮：关键日志前后上下文
第三轮：根据调查假设查询 queue、consumer、traceId
第四轮：必要时查看正常基线
```

### Elasticsearch 接入

准确地说，Rootforge 接入的是 Elasticsearch，而不是 Logstash 或 Kibana。

使用：

- `_search` 执行 Query DSL
- 时间字段排序
- `search_after` 分页
- PIT 保证多页读取时视图一致
- Async Search 处理耗时查询

Elasticsearch 官方建议大量结果使用 `search_after`，PIT 可以避免翻页过程中索引刷新造成结果不一致。[Elasticsearch Search API](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-search-2)、[Point in Time](https://www.elastic.co/guide/en/elasticsearch/reference/current/point-in-time-api.html)

### “订阅查询”应该如何定义

我们需要两种操作：

```go
type EvidenceSource interface {
    Query(ctx context.Context, query EvidenceQuery) (EvidenceStream, error)
    Follow(ctx context.Context, query FollowQuery) (EvidenceStream, error)
}
```

`Query` 用于获取事故前后的历史记录。

`Follow` 用于事故正在发生时，短期观察新日志。

但是 `Follow` 必须有 Case 边界：

```text
case_id
目标服务
过滤条件
最长持续时间，例如 10 分钟
最大记录数
最大字节数
空闲超时
```

例如：

```bash
rootforge evidence follow \
  --case INC-001 \
  --source loki-prod \
  --service wand-saas \
  --for 10m \
  --max-bytes 5MB
```

不能允许这种常驻行为：

```text
永远订阅 production 的全部 ERROR
    ↓
所有日志不断流进 Rootforge
```

那会使 Rootforge重新成为日志处理系统。

Loki 可以使用 `/tail`。Elasticsearch 没有完全对应的日志 Tail 协议，可以按时间戳和唯一 ID 排序，使用 `search_after` 做有限周期轮询。Async Search 是长查询机制，不是实时日志订阅。[Elasticsearch Async Search](https://www.elastic.co/guide/en/elasticsearch/reference/current/async-search-intro.html/)

## 场景二：没有成熟日志系统

这里要区分两种需求。

### 2A：事故发生后立即调查

如果 Rootforge 能在事故发生后很快运行，可以直接从节点本地取证：

```text
Rootforge Case
      ↓
受限 Node Retriever
      ├── Docker Engine API
      ├── journalctl
      ├── 本地 GC 日志
      └── Heap / Thread Dump
```

Node Retriever 是一个小型 Go 程序，不运行 LLM，只暴露预定义操作：

```text
GetContainerInspect
GetContainerLogs
GetDockerEvents
GetJournalEntries
GetFileMetadata
StreamBoundedFile
```

它不提供：

```text
RunShell(command)
DockerAPI(anyRequest)
ReadFile(anyPath)
```

典型查询：

```json
{
  "case_id": "INC-001",
  "source": "journald",
  "units": ["docker.service"],
  "kernel": true,
  "from": "2026-09-10T06:53:45Z",
  "to": "2026-09-10T07:08:45Z",
  "max_records": 2000,
  "max_bytes": 2097152
}
```

在这种模式下：

- 日志继续保存在 Docker 或 journald 本地
- Rootforge 只按事故时间窗读取
- 只有选中的片段进入 Case
- Retriever 不持续向中心推送日志

### 2B：需要调查几小时前或几天前的事故

这时必须有某种历史保留能力，否则节点日志轮转、容器删除后，证据就无法恢复。

但这个能力仍然不应由 Rootforge 自己开发。建议提供一个可选的最小观测栈：

```text
每个节点
Grafana Alloy / OpenTelemetry Collector
             ↓
短期 Loki
保留 24～72 小时
             ↓
Rootforge Loki Connector
```

OpenTelemetry Collector 已经负责接收、处理和转发 telemetry；Grafana Alloy 也已经提供统一的日志、指标和 Trace 采集管道。[OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)、[Grafana Alloy](https://grafana.com/docs/alloy/latest/introduction/)

Rootforge 可以提供：

- Docker Compose 示例
- Swarm Stack 示例
- Alloy 配置模板
- Loki 短期保留建议
- 服务、节点、容器标签规范

但 Alloy 和 Loki 是独立组件：

```text
用户可以替换
用户可以不安装
Rootforge 不 fork
Rootforge 不把它们嵌入自己的存储层
```

### 更轻量的选择：使用 journald 作为节点本地环形缓冲

如果不想运行中央 Loki，可以在每个节点启用持久化 journald，并设置容量和时间限制：

```ini
[Journal]
Storage=persistent
SystemMaxUse=1G
MaxRetentionSec=3day
```

systemd 支持通过 `SystemMaxUse` 限制磁盘占用，通过 `MaxRetentionSec` 限制保留时间。[journald.conf](https://www.freedesktop.org/software/systemd/man/252/journald.conf.html)

然后 Rootforge 在事故发生时到相关节点按时间窗查询：

```text
Rootforge
   ├── query ai-a1 journald
   ├── query ai-a2 journald
   └── query ai-b1 journald
```

这个方案适合：

- 节点数量不多
- Docker Swarm 环境
- 不需要全局日志搜索
- 只需要保留最近几天
- 可以接受节点损坏时日志一起丢失

## 推荐的三种运行模式

| 环境 | Rootforge 行为 | 是否持续接收日志 |
|---|---|---:|
| 已有 Loki/ELK | 通过 API 按 Case 查询 | 否 |
| 没有平台、节点日志仍在 | 通过 Node Retriever 按需读取 | 否 |
| 没有平台、需要历史可靠性 | 建议部署 Alloy + Loki 短期存储 | Rootforge 本身不接收 |

还应该支持第四种离线模式：

```bash
rootforge evidence import \
  --case INC-001 \
  --file application.log \
  --from 06:50 \
  --to 07:10
```

适合人工从生产环境导出证据后离线分析。

## 统一数据模型

不管来自 Loki、Elasticsearch 还是 journald，都转换为：

```go
type LogRecord struct {
    Timestamp  time.Time
    ObservedAt time.Time

    Source     string
    Service    string
    Environment string
    Node       string
    Container  string

    Severity   string
    Stream     string
    Body       string
    Attributes map[string]string

    SourceRef  string
    Fingerprint string
}
```

查询模型必须自带预算：

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

返回结果必须明确说明是否完整：

```go
type EvidenceBatch struct {
    Records       []LogRecord
    Matched       int64
    Selected      int
    Bytes         int64

    Truncated     bool
    NextCursor    string
    SourceQuery   string
    SourceRef     string

    Redactions    int
    Warnings      []string
}
```

## 最终边界

可以把 L1 的产品承诺定为：

> **有日志平台时，Rootforge 查询它；没有日志平台时，Rootforge 从节点本地按需取证；需要历史保留时，Rootforge帮助部署成熟的短期日志基础设施，但不自己成为日志基础设施。**

对应架构：

```text
                    EvidenceSource
                   /      |       \
                Loki  Elasticsearch  Node
                                     /   \
                                  Docker Journald
                     \      |       /
                       EvidenceQuery
                             ↓
                    Budget + Redaction
                             ↓
                       EvidenceBatch
                             ↓
                       Incident Case
```

下一步最合理的实现顺序是：

1. 定义 `EvidenceSource`、`EvidenceQuery`、`EvidenceBatch`。
2. 先实现 `FileSource`，用现有 OOM 日志做测试。
3. 实现 `DockerSource` 和 `JournaldSource`。
4. 实现 `LokiSource`。
5. 再实现 `ElasticsearchSource`。
6. 最后增加 Case 级、带 TTL 的 `Follow` 能力。

这样既能服务没有日志系统的早期用户，也能无缝接入成熟生产环境，同时不会让 Rootforge 偏离自己的核心定位。

> 把方案更新到文档中

<details><summary>4 previous messages</summary>

> 我会把这次讨论落成一份独立的 L1 方案文档，并同步修正现有文档里的术语：将 `Collect` 收敛为 `Acquire / Evidence Access`，明确“联邦查询、节点按需取证、可选短期日志栈”三种模式。
>
> <details><summary>Created a file, edited 3 files, ran a command</summary>
>
> - Ran `rg -n 'Collect|Collector|采集层|采集|日志|Evidence' README.md docs/*.md | head -240; find docs -maxdepth 2 -type f -print | sort`
> - Wrote `./README.md` (+11 -9)
> - Wrote `./docs/evidence-acquisition.zh-CN.md` (+397 -0)
> - Wrote `./docs/product-blueprint.zh-CN.md` (+5 -5)
> - Wrote `./docs/system-layers.zh-CN.md` (+34 -25)
>
> </details>
>
> 方案文档已经加入，并完成了全局术语调整。现在我在做一致性检查，确保 README、产品蓝图和分层文档都不再把 L1 描述成持续日志 Collector。
>
> <details><summary>Edited 2 files, ran 2 commands</summary>
>
> - Ran `rg -n 'L1 Collect|采集层|Lightweight Collector|Collectors|Collector|持续.*日志|四种模式|五种模式|Evidence Acquisition|Evidence Access' README.md docs/*.md; wc -l README.md docs/*.md; sed -n '1,220p' docs/evidence-acquisition.zh-CN.md; sed -n '1,125p' docs/system-layers.zh-CN.md`
>
> - Wrote `./docs/evidence-acquisition.zh-CN.md` (+2 -3)
> - Wrote `./docs/system-layers.zh-CN.md` (+1 -1)
>
> - Ran ````for f in README.md docs/*.md; do n=$(rg -c '^```' "$f"); echo "$n $f"; done; rg -n '四种模式|采集层|L1 Collect|Lightweight Collector|持续接收全部日志|Evidence Acquisition 方案' README.md docs/*.md; test -f docs/evidence-acquisition.zh-CN.md && test -f docs/system-layers.zh-CN.md && test -f docs/product-blueprint.zh-CN.md && echo 'documentation links resolved'````
>
> Tool call
>
> Tool: open_in_codex
> Status: completed
>
> </details>
</details>

方案已更新到文档中，新增：

- [L1 Evidence Acquisition 方案](./docs/evidence-acquisition.zh-CN.md)

文档明确了：

- L1 从 `Collect` 调整为 **Evidence Acquisition**
- Loki、Elasticsearch 的联邦查询模式
- 无日志平台时的 Docker、journald 节点按需取证
- 需要可靠历史时采用 Alloy/OTel Collector + Loki
- 离线文件导入模式
- `Query` 与 Case 级短期 `Follow`
- Go 接口和统一数据模型
- 查询容量、时间窗、脱敏与权限边界
- 实现顺序和 v0.1 验收标准

同时同步更新了：

- [系统分层与边界](./docs/system-layers.zh-CN.md)
- [产品定位蓝图](./docs/product-blueprint.zh-CN.md)
- [README](./README.md)

现在 L1 的正式边界是：

> **有日志平台时查询它；没有日志平台时从节点按需取证；需要历史时提供成熟日志基础设施模板，但 Rootforge 本身不成为日志平台。**

> 我有个问题 难度没有类似的开源系统吗？

<details><summary>2 previous messages</summary>

> 有，而且这个问题必须先查清楚再继续造轮子。最接近的候选很可能是 **HolmesGPT / Robusta**；我会重点比较它是否已经覆盖“告警 → 证据查询 → RCA”，以及它与 Rootforge 的“运行时 → Git Commit → 源码 → 修复/PR”还差多少。
>
> <details><summary>Searched the web</summary>
>
> Searched the web for `HolmesGPT official GitHub open source AI SRE Kubernetes troubleshooting Loki Elasticsearch source code investigation ...`
>
> Searched the web for `site:holmesgpt.dev latest GitHub integration open pull requests source code fix operator mode ...`
>
> Searched the web for `site:holmesgpt.dev "open PR" GitHub integration fix ...`
>
> Searched the web for `site:holmesgpt.dev/latest operator mode HolmesGPT autonomous background GitHub PR ...`
>
> Searched the web for `site:holmesgpt.dev heap dump HPROF Java OOM Spring AMQP ...`
>
> Searched the web for `site:docs.coroot.com AI root cause analysis source code GitHub pull request open source ...`
>
> </details>
</details>

有，而且相似度已经很高。**我前面关于“市场上缺少完整开源系统”的判断需要修正。**

截至 2026 年 9 月，最接近 Rootforge 的是：

# HolmesGPT

[HolmesGPT](https://github.com/HolmesGPT/holmesgpt) 是 Apache-2.0 开源、CNCF Sandbox 的 SRE Agent。它的定位几乎就是：

> 调查生产事故、查询可观测性数据、寻找根因，并连接 GitHub 形成修复。

它目前已经具备：

- 查询 Docker images、logs、events
- 接入 Loki
- 接入 Elasticsearch / OpenSearch
- 查询 Prometheus、Datadog、Tempo、RabbitMQ
- 读取 Kubernetes、VM、云资源和数据库
- 接收 AlertManager、PagerDuty、Jira 等事故
- 读取 GitHub 仓库、提交、PR 和 Actions
- 使用 GitHub MCP 创建分支和 Pull Request
- 自定义 Toolset 和 Runbook
- 后台 Operator 模式

完整能力表甚至已经列出 Docker、Loki、Elasticsearch、GitHub、RabbitMQ 等我们计划中的连接器。[HolmesGPT README](https://github.com/HolmesGPT/holmesgpt)、[数据源配置](https://holmesgpt.dev/latest/data-sources/recommended-setup/)

它还已经处理了我们刚刚讨论的“大量日志”问题：

- 服务端过滤
- 大结果流式写盘
- Tool output transformation
- 输出容量预算
- 子进程内存限制

所以必须承认：

> **我们刚才设计的 L1 Evidence Acquisition，大部分并不是空白，HolmesGPT 已经做了。**

## 与 Rootforge 当前蓝图对比

| 能力 | HolmesGPT | Rootforge 计划 |
|---|---:|---:|
| 生产事故 AI 调查 | 已有 | 计划 |
| Loki / Elasticsearch 查询 | 已有 | 计划 |
| Docker logs/events | 已有 | 计划 |
| Prometheus / Trace / Cloud | 已有 | 计划 |
| GitHub 代码和提交分析 | 已有 | 计划 |
| 自动或定时调查 | Operator Alpha | 计划 |
| 创建 Pull Request | 已有 GitHub 集成 | 计划 |
| 自定义 Analyzer / Toolset | 已有 | 计划 |
| Runbook | 已有 | 计划 |
| JVM Heap Dump 深度分析 | 官方文档中未发现 | 已有 `hprofx` |
| Spring AMQP 对象级诊断 | 官方文档中未发现 | 已有真实案例 |
| 可复现 Incident Case | 部分具备 | 蓝图中的核心 |
| 严格 Fact/Inference/Unknown | 未见明确契约 | 蓝图中的核心 |
| 确定性 Runtime→Build→Commit 映射 | 能查近期变更，但未见强制证明链 | 蓝图中的核心 |

HolmesGPT 的 GitHub 集成已经支持代码搜索、提交查询、分支、推送和创建 PR；写权限是可选的。[HolmesGPT GitHub MCP](https://holmesgpt.dev/dev/data-sources/builtin-toolsets/github-mcp/)

它的 Operator 也能持续执行 HealthCheck 并建议连接 GitHub 自动开 PR，不过官方目前仍明确标为 **Alpha**。[Holmes Operator](https://holmesgpt.dev/latest/operator/)

## 其他相似项目

### K8sGPT

[K8sGPT](https://github.com/k8sgpt-ai/k8sgpt) 是 Go 编写的 Kubernetes 诊断工具，把 SRE 经验编码成 Analyzer，再用 LLM 解释结果。

它与我们设想的 Analyzer 思路相似，但主要关注：

```text
Kubernetes Resource
    -> Analyzer
    -> 问题解释与建议
```

不以跨 Docker/VM、Heap Dump、部署 Commit、源码和 PR 的完整链路为核心。

### Coroot

[Coroot](https://github.com/coroot/coroot) 是 Apache-2.0 的开源 Observability/APM 平台，通过 eBPF 自动采集 metrics、logs、traces 和 profiles，并提供 RCA、部署追踪和代码行级性能分析。

它支持 Docker Swarm，但它本身就是完整的观测平台，会持续采集和保存 telemetry。这与我们刚刚为 Rootforge 划定的“不成为日志系统”边界不同。

### Keep

[Keep](https://github.com/keephq/keep) 是开源 AIOps 和告警管理平台，强项是：

- 告警聚合
- 去重、关联和丰富
- 工作流自动化
- 双向集成

它更接近事故入口和流程编排，不是 Heap/源码级根因调查引擎。

## Rootforge 还有没有必要？

**有可能有，但不能再以“开源 AI SRE Agent”作为差异化定位。** HolmesGPT 已经占据了这个位置。

Rootforge 目前真正独特的部分只剩下：

1. **框架感知的深度取证**

   `hprofx` 不只发现 `byte[]` 很大，还能读出：

   ```text
   ConsumerWorkService retained 810 MB
   ackMode = NONE
   prefetch = 50
   47,584 条客户端积压消息
   consumersMonitor 锁竞争
   ```

2. **确定性的 Runtime-to-Code 证明链**

   ```text
   Container
      -> Image Digest
      -> Build Provenance
      -> Git Commit
      -> File / Symbol
   ```

   不是让模型根据部署时间猜测最近提交。

3. **证据型 Incident Case**

   每个结论必须区分：

   ```text
   Fact
   Inference
   Unknown
   ```

   并能追溯到 Heap 对象、日志、指标、部署记录或代码。

4. **可复现诊断**

   相同 Case、Analyzer 版本和规则能够重新运行，结果可比较、可审计。

这些能力可以成为 HolmesGPT 的增强，而不一定需要重造完整 Agent 平台。

## 推荐路线

现在先暂停实现 Rootforge L1。先做一次 HolmesGPT 验证：

```text
今天的 RabbitMQ OOM Case
        ↓
HolmesGPT
        +
自定义 hprofx Toolset
        +
GitHub 只读访问
        ↓
观察它能否形成同等质量 RCA
```

HolmesGPT 支持通过 YAML、脚本或 MCP 扩展自定义 Toolset，所以可以直接把 Go 编写的 `hprofx` 接进去。[Custom Toolsets](https://holmesgpt.dev/0.20.0/data-sources/custom-toolsets/)

验证重点：

- 能否自动选择正确的 Heap 分析命令
- 能否控制日志和 Heap 结果规模
- 能否明确引用证据
- 能否找到 `ackMode=NONE`
- 能否关联到实际代码和部署提交
- 能否生成合理补丁和测试
- 能否在 Docker Swarm、非 Kubernetes 场景顺畅运行
- 权限和数据外发是否满足要求

如果验证通过，最合理的架构可能变成：

```text
HolmesGPT
├── L1 数据源连接
├── L5 Agent 调查循环
└── 部分 L6 GitHub 操作

Rootforge
├── Incident Case 证据模型
├── Runtime-to-Code 映射
├── hprofx
├── JVM / Spring / RabbitMQ Analyzer
└── 可验证的修复与回归测试
```

也就是：

> **HolmesGPT 是通用 SRE Investigator；Rootforge 是框架感知、证据驱动的 Runtime-to-Code 取证引擎。**

还有一个生产评估注意点：HolmesGPT Operator 仍是 Alpha，项目在 2026 年 7 月披露过一个 Critical 级远程代码执行安全公告，因此试用时应选择已修复版本、限制网络入口和工具权限。[HolmesGPT Security](https://github.com/HolmesGPT/holmesgpt/security)

所以目前最理性的决定不是立刻继续造 L1，而是：

> **先用真实 OOM 案例验证 HolmesGPT，再决定 Rootforge 是其 Toolset、上游贡献，还是一个只聚焦深度取证的独立项目。**

> [https://holmesgpt.dev/latest/installation/cli-installation/#quick-start](https://holmesgpt.dev/latest/installation/cli-installation/#quick-start) 这是他们的文档，我看好像是需要ask ？

<details><summary>3 previous messages</summary>

> 我去核对最新版 CLI 文档和 Operator/告警入口。重点确认 `ask` 是不是唯一触发方式，以及自动调查哪些能力目前仍是 Alpha。
>
> <details><summary>Searched the web</summary>
>
> Searched the web for `https://holmesgpt.dev/latest/installation/cli-installation/#quick-start`
>
> Searched the web for `https://holmesgpt.dev/latest/walkthrough/investigating-prometheus-alerts/`
>
> </details>
>
> Context compaction
>
> Source: automatic
> Status: completed
</details>

对，你理解得对一半：**HolmesGPT 的 CLI 默认确实需要先执行 `holmes ask`，由人提供调查目标。**

例如：

```bash
holmes ask "为什么 user-profile-import Pod 异常？"
```

问题提交后，日志、指标和 Kubernetes 状态的查询与推理过程是自动的，但“什么时候调查、调查什么”由人发起。[CLI Quick Start](https://holmesgpt.dev/latest/installation/cli-installation/)

它还有另外几种模式：

| 模式 | 是否每次手动提问 | 特点 |
|---|---:|---|
| `holmes ask` | 是 | 人工指定调查目标 |
| `holmes investigate alertmanager` | 不需要写问题，但要执行命令 | 读取当前 Alertmanager 告警并逐个调查 |
| Scheduled HealthCheck | 否 | 按计划自动执行预设调查 |
| TriggeredHealthCheck | 否 | Kubernetes Deployment 发布时自动调查 |

Alertmanager 可以这样调查：

```bash
holmes investigate alertmanager \
  --alertmanager-url http://localhost:9093 \
  --alertmanager-label "severity=critical"
```

不过它仍然需要外部定时任务或者其他系统调用这条命令，才能真正实现无人值守。[Alertmanager 调查文档](https://holmesgpt.dev/latest/walkthrough/investigating-prometheus-alerts/)

Holmes Operator 能够 24×7 运行，但有两个重要限制：

- 目前属于 **Alpha**。
- 必须运行在 Kubernetes 中。
- 自动触发目前主要支持 Kubernetes Deployment rollout。
- Pod CrashLoop、Job 失败、通用告警等触发类型，文档仍标记为后续计划。
- 即使自动触发，也需要提前配置一段自然语言 `query`，告诉 Agent 调查目标。

详见 [Holmes Operator](https://holmesgpt.dev/latest/operator/) 和 [Triggered Health Checks](https://holmesgpt.dev/latest/operator/triggered-health-checks/)。

所以真正的区别不是“有没有 prompt”，而是：

```text
Holmes CLI
人发现问题
  → 人输入 ask
  → Holmes 自动调查

Rootforge 设想
监控系统发现事件
  → 自动创建 Incident Case
  → 根据事件生成调查目标
  → 自动收集限定范围证据
  → Agent 调查
  → RCA / Fix
```

对于我们此前的 Docker Swarm OOM 场景，HolmesGPT 目前没有完全现成的：

```text
Docker OOM Event
    → 自动确定 service/container/time window
    → 拉取 Loki/ELK 日志
    → 获取 heap dump
    → 调用 hprofx
    → 关联代码
    → 输出 RCA
```

但它已经很好地覆盖了中间的 Agent 调查能力。因此 Rootforge 不一定需要重新开发一个 HolmesGPT，更合理的定位可能是：

```text
Rootforge
├── Trigger：接收 OOM/告警/运行时事件
├── Case：建立事故、范围和证据链
├── Evidence：Loki/ELK/Docker/heap dump
├── Diagnostics：hprofx 等确定性分析工具
└── Investigator：复用或兼容 HolmesGPT
```

一句话概括：

> **HolmesGPT 更像“给我一个问题，我来调查”；Rootforge 可以做成“事故一发生，我就知道该调查什么”。**

这仍然给 Rootforge 留下了一个清晰而且有价值的边界：**事件驱动、自动建案、证据约束和专项诊断**。

> 我的本意是无人值守的情况下 把问题找到并且修复 或者通知开发者

明白。那 Rootforge 的核心就不是“AI 运维问答工具”，而是一个真正的**无人值守事故闭环系统**：

> **Rootforge continuously detects production incidents, finds the root cause, attempts a safe fix, and notifies developers when human intervention is required.**

中文可以定义为：

> **Rootforge 持续监控生产系统，在无人值守情况下发现故障、收集证据、定位代码根因、生成并验证修复；无法安全自动处理时，携带完整上下文通知开发者。**

## Rootforge 的核心闭环

```text
生产环境异常
     ↓
自动发现 Incident
     ↓
创建 Case，确定服务、时间和影响范围
     ↓
按需获取日志、指标、Trace、Dump、部署信息
     ↓
确定性工具分析 + AI 调查
     ↓
定位根因及相关代码
     ↓
生成候选修复
     ↓
自动测试和风险评估
     ↓
┌────────────────┬────────────────┐
│ 风险低、置信度高 │ 风险高或证据不足 │
│ 自动修复/回滚    │ 通知开发者       │
│ 验证生产结果     │ 提供 RCA 和 PR    │
└────────────────┴────────────────┘
```

这里不应该要求开发者先运行：

```bash
rootforge ask "为什么 OOM？"
```

事件本身就是输入：

```text
Docker OOMKilled
Prometheus Alert
Loki error spike
Process crash
Deployment regression
Queue backlog
Latency anomaly
```

Rootforge 根据事件自动生成调查目标，例如：

```yaml
incident:
  type: process_oom
  service: rabbitmq
  instance: rabbitmq-1
  detected_at: 2026-09-11T03:21:00Z

generated_objective:
  determine:
    - why memory continuously increased
    - which process or object retained memory
    - whether the cause originated in configuration, workload, or code
    - whether a safe remediation is available
```

这段调查目标是系统生成的，不需要人输入 `ask`。

## 无人值守不等于无条件修改生产环境

Rootforge 应当提供逐级自动化策略：

| 级别 | 系统行为 |
|---|---|
| Observe | 自动发现、调查并通知 |
| Recommend | 输出 RCA、修复方案和操作建议 |
| Prepare | 自动创建修复分支或 PR，等待审批 |
| Remediate | 执行白名单中的低风险操作 |
| Autonomous | 修复、验证、失败时自动回滚 |

第一阶段建议做到 **Prepare**：

```text
自动发现
→ 自动调查
→ 自动 RCA
→ 自动生成修复
→ 自动运行测试
→ 创建 PR
→ 通知开发者审批
```

只有明确安全、幂等、可回滚的动作可以直接执行，例如：

- 重启已经失去服务能力的单个副本
- 扩容满足预设策略的实例
- 回滚刚刚发布且已验证引发故障的版本
- 清理明确过期的临时数据
- 熔断发生异常的非核心依赖

修改代码、数据库结构、删除数据等操作，默认只生成 PR 或建议。

## 与 HolmesGPT 的真正关系

HolmesGPT 可以作为 Rootforge 的一个调查引擎，但不能代表完整产品：

```text
Rootforge Controller
├── 事件检测
├── Incident 状态机
├── 自动生成调查目标
├── 证据收集与保存
├── HolmesGPT / 其他 Agent
├── hprofx 等诊断工具
├── 修复策略与安全边界
├── 测试、发布和回滚
└── 开发者通知
```

HolmesGPT 主要回答：

> 给定一个调查目标，如何利用各种工具寻找原因？

Rootforge 要回答的是：

> 何时需要调查、调查什么、证据是否充分、是否可以修复、如何验证，以及什么时候必须找人。

因此 Rootforge 的准确定位不是另一个 observability 平台，也不是另一个 `ask` 式 AI CLI，而是：

> **An autonomous incident investigation and remediation system.**

第一期 OOM MVP 的完成标准也应该调整为：

```text
无人执行任何命令
    ↓
自动收到 OOM 事件
    ↓
自动建立 Case
    ↓
自动获取相关日志、指标和 Dump
    ↓
自动调用 hprofx
    ↓
生成可信的根因报告
    ↓
能够修复则创建 PR
否则携带证据通知开发者
```

这才是 Rootforge 最核心的产品方向：**不是帮助开发者调查，而是在开发者还没介入之前，先把调查和可安全执行的修复做完。**

> ```
> ├── HolmesGPT / 其他 Agent
> ```
>
> 这部分我是想开发一个自己的Harness Agent

明白。那 Rootforge 不依赖 HolmesGPT，而是开发自己的 **Harness Agent**。HolmesGPT 只作为竞品、能力参考和测试基线。

这里的 Harness 不是大模型本身，而是 Rootforge 的 **Agent 执行与推理框架**：

> 给它一个 Incident Case，它能够自主制定调查计划、调用诊断工具、验证假设、收集证据，并产出可审计的结论和修复方案。

整体关系应改成：

```text
Rootforge
├── Event Gateway
│   └── 接收 Alertmanager、Docker、Kubernetes、Webhook 事件
│
├── Incident Controller
│   └── 去重、聚合、建案、状态管理、调度
│
├── Evidence Layer
│   └── 日志、指标、Trace、Dump、部署与代码上下文
│
├── Harness Agent                 ← 自研核心
│   ├── Planner
│   ├── Tool Selector
│   ├── Investigation Loop
│   ├── Hypothesis Manager
│   ├── Evidence Evaluator
│   └── Conclusion Builder
│
├── Diagnostic Toolsets
│   ├── Loki / Elasticsearch
│   ├── Prometheus
│   ├── Docker / Kubernetes
│   ├── Git
│   └── hprofx
│
├── Forge
│   └── 生成修复、运行测试、创建 PR
│
└── Policy & Action Executor
    └── 审批、执行、验证、回滚、通知
```

## Harness Agent 的边界

Harness Agent 负责：

- 理解 Incident 和调查目标
- 制定动态调查计划
- 选择和调用工具
- 根据新证据调整计划
- 建立、验证和排除假设
- 判断证据是否足够
- 输出根因、置信度和修复建议
- 证据不足时明确升级给开发者

Harness Agent 不负责：

- 长期存储全部日志
- 判断是否产生新 Incident
- 直接拥有生产环境凭证
- 绕过策略修改生产环境
- 自己决定高风险操作是否可以执行
- 直接部署未经测试的代码

也就是说：

```text
Incident Controller 决定：
什么时候启动调查、调查哪个事故

Harness Agent 决定：
下一步查什么、调用什么工具、结论是否成立

Policy Engine 决定：
这个修复是否允许执行
```

这个边界非常重要。否则 Agent 既负责推理又拥有无限生产权限，会很难保证安全性。

## Agent 的核心循环

```text
Incident Case
     ↓
读取已有证据
     ↓
生成候选假设
     ↓
选择当前信息增益最高的操作
     ↓
调用受控 Tool
     ↓
记录原始结果和证据来源
     ↓
更新假设及置信度
     ↓
┌───────────────┬──────────────────┐
│ 证据不足       │ 根因已经被验证    │
│ 继续调查       │ 生成结论与修复方案 │
└───────────────┴──────────────────┘
```

伪代码大致是：

```go
for step := 0; step < budget.MaxSteps; step++ {
    state := caseStore.Load(caseID)

    decision := agent.Decide(DecisionInput{
        Incident:   state.Incident,
        Evidence:   state.Evidence,
        Hypotheses: state.Hypotheses,
        Tools:      toolRegistry.AllowedTools(state),
        Budget:     budget,
    })

    switch decision.Action {
    case CallTool:
        result := toolRunner.Execute(decision.ToolCall)
        caseStore.AppendEvidence(caseID, result)

    case Conclude:
        return decision.Conclusion

    case Escalate:
        return decision.Escalation
    }
}
```

## Agent 与工具应该使用结构化协议

不要让 Agent 通过任意 Shell 命令完成一切，而是提供受控工具：

```go
type Tool interface {
    Name() string
    Schema() JSONSchema
    Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
}
```

例如：

```text
logs.search
metrics.query_range
runtime.inspect_container
runtime.get_oom_event
heapdump.analyze
git.find_deployment_commit
git.search_source
test.run
notification.send
```

每次调用都要保存：

```yaml
tool: heapdump.analyze
input:
  artifact_id: artifact-123
started_at: ...
finished_at: ...
result_digest: ...
evidence_ids:
  - evidence-456
agent_reason:
  "The OOM event indicates beam.smp memory growth; heap analysis
   can distinguish binary retention from process heap growth."
```

这样 Harness Agent 的结论才可复现、可审计，而不只是生成一段看似合理的文字。

## 建议的内部模块

Go 工程可以按照以下方向发展：

```text
internal/
├── harness/
│   ├── agent.go
│   ├── loop.go
│   ├── planner.go
│   ├── hypothesis.go
│   ├── context.go
│   ├── budget.go
│   └── conclusion.go
│
├── model/
│   ├── provider.go
│   ├── openai.go
│   └── anthropic.go
│
├── tools/
│   ├── registry.go
│   ├── runner.go
│   ├── logs/
│   ├── metrics/
│   ├── runtime/
│   ├── heapdump/
│   └── git/
│
├── incident/
├── evidence/
├── policy/
└── forge/
```

其中最值得自研、也是 Rootforge 的技术核心是：

1. **Case-aware context**：Agent 始终围绕一个事故工作。
2. **Evidence ledger**：任何结论必须指向证据。
3. **Hypothesis-driven investigation**：不是无限查询日志，而是用假设驱动取证。
4. **Budget control**：限制时间、token、工具次数和查询范围。
5. **Deterministic verification**：关键判断尽量由 `hprofx` 等确定性工具验证。
6. **Safe execution boundary**：Agent 提议操作，Policy Engine 决定能否执行。

因此产品表述可以进一步收敛为：

> **Rootforge is an autonomous incident-response system powered by its own evidence-driven Harness Agent.**

而 HolmesGPT 对我们最有价值的地方，是拿来做对照测试：

```text
同一个 OOM Incident
├── HolmesGPT 能找到什么
├── Rootforge Harness 能找到什么
├── 各自用了多少步骤和 token
├── 是否引用了可验证证据
└── 是否能够形成安全的修复
```

这条路线的差异化是成立的：**不是重新包装一个通用聊天 Agent，而是构建一个专门面向生产事故、以证据和安全边界为核心的 Agent Harness。**

> OK，开始构建整个项目结构，先不要具体写逻辑代码，

<details><summary>8 previous messages</summary>

> 我先检查现有仓库、文档和 `heapdump` 目录，再按刚才确定的边界搭建 Go 项目骨架。只建立模块、接口占位和说明，不实现调查逻辑。
>
> <details><summary>Explored 5 files, a list, ran a command</summary>
>
> - Ran `pwd && rg --files -g 'AGENTS.md' -g '!heapdump/**/vendor/**' -g '!heapdump/**/.git/**' && rg --files -g '!heapdump/**/vendor/**' -g '!heapdump/**/.git/**' | sed -n '1,240p'`
>
> <details><summary>Explored 5 files, a list</summary>
>
> - Listed files in `.`
> - Read `./README.md`
> - Read `./docs/system-layers.zh-CN.md`
> - Read `./docs/product-blueprint.zh-CN.md`
> - Read `./docs/evidence-acquisition.zh-CN.md`
> - Read `./heapdump/hprofx/go.mod`
>
> </details>
>
> </details>
>
> 现有文档仍把产品描述成“人工触发调查、人工批准修复”，与刚确认的无人值守目标有冲突。我会在搭骨架时同步校正定位：自动发现和调查是默认能力；生产动作由策略分级，不能由 Harness Agent 自行越权。现有 `heapdump/hprofx` 保持独立 Go 模块，先作为确定性工具接入点保留。
>
> <details><summary>Created 43 files, edited 3 files, explored 2 files, a list, ran 3 commands</summary>
>
> - Ran `git status --short && find . -maxdepth 2 -type d | sort && sed -n '240,520p' docs/system-layers.zh-CN.md && sed -n '320,720p' docs/evidence-acquisition.zh-CN.md`
>
> <details><summary>Explored 2 files, a list</summary>
>
> - Listed files in `.`
> - Read `./docs/system-layers.zh-CN.md`
> - Read `./docs/evidence-acquisition.zh-CN.md`
>
> </details>
>
> - Ran `go version`
>
> - Ran `mkdir -p api/v1alpha1 cmd/rootforge cmd/rootforged cmd/rootforge-retriever configs/examples deployments/docker-compose deployments/swarm deployments/kubernetes docs/adr internal/trigger internal/incident internal/casefile internal/evidence/source/file internal/evidence/source/docker internal/evidence/source/journald internal/evidence/source/loki internal/evidence/source/elasticsearch internal/analyzer/heapdump internal/analyzer/jvm internal/analyzer/logs internal/analyzer/deployment internal/analyzer/git internal/correlation internal/harness internal/llm internal/tool internal/forge internal/policy internal/action internal/notification internal/audit internal/storage internal/config testdata/cases/rabbitmq-oom`
>
> - Wrote `./.gitignore` (+29 -0)
> - Wrote `./ARCHITECTURE.md` (+139 -0)
> - Wrote `./CONTRIBUTING.md` (+20 -0)
> - Wrote `./Makefile` (+14 -0)
> - Wrote `./api/v1alpha1/README.md` (+8 -0)
> - Wrote `./cmd/rootforge-retriever/README.md` (+6 -0)
> - Wrote `./cmd/rootforge/README.md` (+6 -0)
> - Wrote `./cmd/rootforged/README.md` (+6 -0)
> - Wrote `./configs/examples/README.md` (+6 -0)
> - Wrote `./deployments/docker-compose/README.md` (+5 -0)
> - Wrote `./deployments/kubernetes/README.md` (+5 -0)
> - Wrote `./deployments/swarm/README.md` (+5 -0)
> - Wrote `./docs/adr/0001-unattended-event-driven-core.md` (+25 -0)
> - Wrote `./docs/adr/README.md` (+6 -0)
> - Wrote `./go.mod` (+4 -0)
> - Wrote `./go.work` (+7 -0)
>
> - Wrote `./internal/action/README.md` (+8 -0)
> - Wrote `./internal/analyzer/README.md` (+8 -0)
> - Wrote `./internal/analyzer/deployment/README.md` (+5 -0)
> - Wrote `./internal/analyzer/git/README.md` (+5 -0)
> - Wrote `./internal/analyzer/heapdump/README.md` (+6 -0)
> - Wrote `./internal/analyzer/jvm/README.md` (+5 -0)
> - Wrote `./internal/analyzer/logs/README.md` (+5 -0)
> - Wrote `./internal/audit/README.md` (+8 -0)
> - Wrote `./internal/casefile/README.md` (+9 -0)
> - Wrote `./internal/config/README.md` (+7 -0)
> - Wrote `./internal/correlation/README.md` (+11 -0)
> - Wrote `./internal/evidence/README.md` (+19 -0)
> - Wrote `./internal/evidence/source/docker/README.md` (+6 -0)
> - Wrote `./internal/evidence/source/elasticsearch/README.md` (+6 -0)
> - Wrote `./internal/evidence/source/file/README.md` (+5 -0)
> - Wrote `./internal/evidence/source/journald/README.md` (+5 -0)
> - Wrote `./internal/evidence/source/loki/README.md` (+6 -0)
> - Wrote `./internal/forge/README.md` (+9 -0)
> - Wrote `./internal/harness/README.md` (+24 -0)
> - Wrote `./internal/incident/README.md` (+8 -0)
> - Wrote `./internal/llm/README.md` (+8 -0)
> - Wrote `./internal/notification/README.md` (+8 -0)
> - Wrote `./internal/policy/README.md` (+8 -0)
> - Wrote `./internal/storage/README.md` (+8 -0)
> - Wrote `./internal/tool/README.md` (+9 -0)
> - Wrote `./internal/trigger/README.md` (+8 -0)
> - Wrote `./testdata/cases/rabbitmq-oom/README.md` (+9 -0)
>
> - Wrote `./README.md` (+47 -25)
>
> - Wrote `./docs/product-blueprint.zh-CN.md` (+41 -21)
>
> - Wrote `./docs/system-layers.zh-CN.md` (+34 -23)
>
> </details>
>
> 项目边界已经落成：三个可执行入口、事件与 Case 主干、自研 Harness Agent、确定性工具、Forge、Policy/Action 和通知层都已独立。现在我在做一致性检查，避免旧文档里的“人工 ask / 人工启动”与新架构互相矛盾。
>
> <details><summary>Edited 2 files, explored a search, a list, ran a command</summary>
>
> <details><summary>Explored a search, a list</summary>
>
> - Searched for `Holmes|Investigator|人工|Human|ask|审批|自主|自动` in `README.md`
> - Listed files in `.`
>
> </details>
>
> - Wrote `./docs/evidence-acquisition.zh-CN.md` (+2 -2)
> - Wrote `./docs/system-layers.zh-CN.md` (+4 -4)
>
> - Ran `make test && make vet`
>
> </details>
>
> 验证发现根模块目前只有目录说明，没有任何 Go package，`go test ./...` 会把“没有包”视为失败。我会给每个边界补一个仅含包说明的 `doc.go`；这样仍然没有业务逻辑，但项目骨架已经是可被 Go 工具识别、可持续扩展的真实包结构。
>
> <details><summary>Created 27 files, edited 2 files, explored 2 files, a search, ran 3 commands</summary>
>
> - Wrote `./api/v1alpha1/doc.go` (+3 -0)
> - Wrote `./internal/action/doc.go` (+3 -0)
> - Wrote `./internal/analyzer/deployment/doc.go` (+3 -0)
> - Wrote `./internal/analyzer/doc.go` (+3 -0)
> - Wrote `./internal/analyzer/git/doc.go` (+3 -0)
> - Wrote `./internal/analyzer/heapdump/doc.go` (+3 -0)
> - Wrote `./internal/analyzer/jvm/doc.go` (+3 -0)
> - Wrote `./internal/analyzer/logs/doc.go` (+3 -0)
> - Wrote `./internal/audit/doc.go` (+3 -0)
> - Wrote `./internal/casefile/doc.go` (+3 -0)
> - Wrote `./internal/config/doc.go` (+3 -0)
> - Wrote `./internal/correlation/doc.go` (+3 -0)
> - Wrote `./internal/evidence/doc.go` (+3 -0)
> - Wrote `./internal/evidence/source/docker/doc.go` (+3 -0)
> - Wrote `./internal/evidence/source/elasticsearch/doc.go` (+3 -0)
> - Wrote `./internal/evidence/source/file/doc.go` (+3 -0)
> - Wrote `./internal/evidence/source/journald/doc.go` (+3 -0)
> - Wrote `./internal/evidence/source/loki/doc.go` (+3 -0)
> - Wrote `./internal/forge/doc.go` (+3 -0)
> - Wrote `./internal/harness/doc.go` (+3 -0)
> - Wrote `./internal/incident/doc.go` (+3 -0)
> - Wrote `./internal/llm/doc.go` (+3 -0)
> - Wrote `./internal/notification/doc.go` (+3 -0)
> - Wrote `./internal/policy/doc.go` (+3 -0)
> - Wrote `./internal/storage/doc.go` (+3 -0)
> - Wrote `./internal/tool/doc.go` (+3 -0)
> - Wrote `./internal/trigger/doc.go` (+3 -0)
>
> - Wrote `./Makefile` (+3 -1)
>
> - Ran `make fmt && make test && make vet`
>
> - Wrote `./Makefile` (+3 -3)
>
> - Ran `make fmt && make test && make vet`
>
> <details><summary>Explored 2 files, a search</summary>
>
> - Read `./README.md`
> - Read `./ARCHITECTURE.md`
> - Searched for `doc.go` in `internal`
>
> </details>
>
> - Wrote `./README.md` (+4 -2)
>
> - Ran `make test && find internal api -name '*.go' ! -name 'doc.go' -print && find . -maxdepth 3 -type d | sort`
>
> Tool call
>
> Tool: open_in_codex
> Status: completed
>
> </details>
</details>

项目骨架已经完成，并按照“无人值守 + 自研 Harness Agent”重新统一了定位。

核心成果：

- 三个入口：`rootforge` CLI、`rootforged` 控制面、`rootforge-retriever` 只读取证。
- 建立 Trigger → Incident → Case → Harness → Forge → Policy/Action → Notification 全链路。
- Harness Agent 与 LLM Provider、Tool Gateway、确定性 Analyzer 解耦。
- Loki、Elasticsearch、Docker、journald、文件取证均预留独立边界。
- `hprofx` 保持独立 Go module，通过 `go.work` 纳入工作区。
- 建立 Docker Compose、Swarm、Kubernetes 部署目录。
- 建立 RabbitMQ OOM 验收案例目录。
- 新增 ADR，明确 HolmesGPT 只是评估基线，不是运行依赖。
- 当前 26 个 Go package 只有 `doc.go` 包说明，没有业务逻辑。

主要文档：

- [项目架构与目录边界](./ARCHITECTURE.md)
- [README](./README.md)
- [产品定位蓝图](./docs/product-blueprint.zh-CN.md)
- [系统分层与边界](./docs/system-layers.zh-CN.md)
- [无人值守核心 ADR](./docs/adr/0001-unattended-event-driven-core.md)
- [Harness Agent 边界](./internal/harness/README.md)

验证结果：

```text
make fmt  通过
make test 通过
make vet  通过
hprofx    编译、测试和 vet 通过
```

目前根模块名暂定为 `rootforge`；确定最终 GitHub organization 后，再替换成正式 module path。

> 把代码提交到远程仓库
> git remote add origin git\@github.com:akz142857/Rootforge.git
> git branch -M main
> git push -u origin main

<details><summary>10 previous messages</summary>

> 我会初始化 Git 仓库，检查待提交文件中是否包含大文件或生产敏感材料，然后创建首个提交并推送到你给出的 GitHub 仓库。现有 heapdump 报告会先确认大小和内容类型，避免误提交实际 dump。
>
> <details><summary>Ran 2 commands</summary>
>
> - Ran `git init`
> - Ran `git status --short && find . -type f -not -path './.rootforge/*' -not -path './.git/*' -exec du -h {} + | sort -h | tail -30 && file heapdump/hprofx-sh heapdump/* 2>/dev/null`
>
> </details>
>
> 检查发现 `heapdump/hprofx-sh` 是 2.2 MB 的 macOS ARM 编译产物，不适合进入源码仓库；`hprofx` 源码会正常提交。我会把该二进制加入忽略规则，并对报告和配置做一次仅返回文件名的敏感信息扫描。
>
> <details><summary>Edited 2 files, explored a file, 4 searches, ran a command</summary>
>
> - Wrote `./.gitignore` (+1 -1)
>
> - Ran `rg -l -i --hidden -g '!.git/**' -g '!.rootforge/**' -g '!heapdump/hprofx-sh' '(api[_-]?key|access[_-]?token|secret[_-]?key|private[_-]?key|password\s*[:=]|authorization:\s*bearer|BEGIN [A-Z ]*PRIVATE KEY|github_pat_)' . || true
> rg -l --hidden -g '!.git/**' -g '!.rootforge/**' -g '!heapdump/hprofx-sh' '(10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3}|192\.168\.[0-9]{1,3}\.[0-9]{1,3})' . || true
> git status --short --ignored | sed -n '1,240p'`
>
> <details><summary>Explored a file, 2 searches</summary>
>
> - Searched for `(api[_-]?key|access[_-]?token|secret[_-]?key|private[_-]?key|password\s*[:=]|authorization:\s*bearer|BEGIN [A-Z ]*PRIVATE KEY|github_pat_)` in `analysis.md`
> - Searched for `(10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3}|192\.168\.[0-9]{1,3}\.[0-9]{1,3})` in `analysis.md`
> - Read `./heapdump/analysis.md`
>
> </details>
>
> - Wrote `./heapdump/analysis.md` (+1 -2)
>
> <details><summary>Explored 2 searches</summary>
>
> - Searched for `(/Users/|/home/[^ /]+|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,})` in `.`
> - Searched for `(10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3}|192\.168\.[0-9]{1,3}\.[0-9]{1,3})` in `.`
>
> </details>
>
> </details>
>
> 敏感扫描已通过；唯一发现的生产内网地址已替换为 `<rabbitmq-host>`，本地编译二进制和运行缓存都已忽略。接下来我会设置远程、暂存全部源码与文档，检查提交清单后创建初始提交。
>
> <details><summary>Ran 2 commands</summary>
>
> - Ran `git remote add origin git@github.com:akz142857/Rootforge.git && git branch -M main && git add . && git status --short && git diff --cached --stat`
> - Ran `git diff --cached --check && git commit -m "Initial Rootforge architecture scaffold"`
>
> </details>
>
> 提交检查发现新建说明文件末尾多了一行空白，Git 将其视为 whitespace 警告；提交尚未产生。我会机械清理这些文件末尾的多余空行，重新暂存并提交。
>
> <details><summary>Ran 2 commands</summary>
>
> - Ran `git status --short | sed -n '1,120p' && git diff --check`
> - Ran `git add . && git diff --cached --check && git commit -m "Initial Rootforge architecture scaffold"`
>
> </details>
>
> 初始提交已创建：`2b2a3a6 Initial Rootforge architecture scaffold`。现在推送 `main` 并设置上游分支。
>
> <details><summary>Ran 2 commands</summary>
>
> - Ran `git push -u origin main`
> - Ran `git status --short && git branch -vv && git remote -v && git log -1 --oneline`
>
> </details>
</details>

代码已提交并成功推送：

- 仓库：[akz142857/Rootforge](https://github.com/akz142857/Rootforge)
- 分支：`main`
- 提交：`2b2a3a6 Initial Rootforge architecture scaffold`
- `main` 已跟踪 `origin/main`
- 工作区干净

另外，编译产物 `heapdump/hprofx-sh` 未提交，报告中的生产内网 IP 已脱敏。
