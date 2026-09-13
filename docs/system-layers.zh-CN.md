# Rootforge 系统分层与边界

> **设计原则：越靠近生产环境，行为越确定、权限越小；越靠近修复动作，审批越严格。**

## 总览

Rootforge 分为六个纵向层，以及一个贯穿所有层的控制面：

```text
外部系统
监控 / 主机 / Docker / JVM / Deploy / Git / CI
                         |
                         v
+------------------------------------------------------+
| L1 证据访问层    Acquire                             |
| 事故触发、联邦查询、节点按需取证                       |
+---------------------------+--------------------------+
                            v
+------------------------------------------------------+
| L2 案件与证据层  Case & Evidence                     |
| 标准化、溯源、快照、证据保存                           |
+---------------------------+--------------------------+
                            v
+------------------------------------------------------+
| L3 确定性分析层  Analyze                             |
| Heap、线程、日志、框架状态等可重复分析                 |
+---------------------------+--------------------------+
                            v
+------------------------------------------------------+
| L4 关联层        Correlate                           |
| Runtime -> Image -> Build -> Commit -> Code          |
+---------------------------+--------------------------+
                            v
+------------------------------------------------------+
| L5 ClayHarness 调查运行时                            |
| 通用 Agent 提出假设、请求工具、验证证据、形成结构化输出   |
+---------------------------+--------------------------+
                            v
+------------------------------------------------------+
| L6 修复与行动层  Forge & Act                         |
| 候选修复、隔离验证、策略审批、执行、回滚或通知           |
+------------------------------------------------------+

  横切控制面：身份 / 权限 / 密钥 / 脱敏 / 审批 / 审计 / 预算
```

监控、日志存储、Git 和 CI 仍是各自事实的源系统。Rootforge 连接它们，但不取代它们。

## L1：证据访问层（Evidence Acquisition）

### 定义

负责接收低流量的事故触发，并在 Case 建立后，从生产环境及外部系统按时间窗、实体、调查假设和预算获取有限证据。

L1 不建设持续接收、传输、索引和长期保存全部日志的系统。已有 Loki 或 Elasticsearch 时，通过 API 做联邦查询；没有集中日志系统时，通过受限 Node Retriever 从 Docker、journald 和本地文件按需取证；需要可靠历史时，推荐部署独立的成熟日志基础设施。

典型输入：

- Prometheus、Grafana、APM 或自定义告警
- Docker / Swarm / Kubernetes 事件
- 主机内核日志、容器日志和应用日志
- JVM heap dump、thread dump、GC 日志
- 镜像标签、部署记录、构建元数据
- Git 和 CI 系统的只读信息

典型输出是带来源信息和完整性状态的 `EvidenceBatch`：

```text
case_id
source
query
incident_time_range
records_or_artifacts
source_reference
matched_and_selected
bytes
truncated
next_cursor
connector_version
redaction_status
```

### 边界

证据访问层可以：

- 接收低流量、高信号的告警与运行时事件
- 调用 Loki、Elasticsearch 等源系统的只读查询 API
- 执行预先允许的节点本地只读取证操作
- 按事故、服务、节点、时间窗和容量预算限制范围
- 对敏感字段进行初步脱敏
- 保存选中的证据、查询方法和原始材料引用

证据访问层不可以：

- 判断最终根因
- 修改配置、重启服务或执行部署
- 把生产主机的完整 Shell 权限交给大模型
- 持续接收、传输、索引或长期保存全部日志
- 创建没有 Case、TTL 和容量预算的常驻日志订阅
- 无限制查询全部日志、全部仓库或全部主机数据
- 因为“可能有用”而永久复制外部系统中的所有数据

**这一层的责任是访问和取得有限证据，不是建设日志平台，也不是解释事实。**

完整方案详见 [L1 Evidence Acquisition 方案](evidence-acquisition.zh-CN.md)。

## L2：案件与证据层（Case & Evidence）

### 定义

把一次事故组织成可复现、可归档、可审计的 `Incident Case`。它是系统的数据主干，也是所有上层能力共同依赖的事实边界。

一个 Case 至少包含：

- 事故身份、状态、影响对象和时间范围
- 原始证据及来源、采集时间、校验值
- 服务、节点、容器、镜像等已知实体
- Analyzer 产生的结构化 Finding
- 调查假设、RCA 版本和人工确认结果
- 使用过的工具、规则、模型和版本
- 产生的修复提案、测试结果和审批记录

### 边界

案件与证据层可以：

- 标准化不同 Evidence Source 的数据
- 保留原始证据与派生结果之间的血缘关系
- 记录证据缺失、冲突和过期状态
- 为同一份证据提供可重复分析的快照

案件与证据层不可以：

- 把推断伪装成原始事实
- 静默覆盖原始证据或历史 RCA
- 自行决定哪个根因正确
- 成为通用日志、指标或 Trace 的长期存储平台

**这一层保存“我们知道什么，以及从哪里知道”，不决定“这意味着什么”。**

## L3：确定性分析层（Analyze）

### 定义

通过可重复运行的 Analyzer，把原始材料转换成结构化事实与局部诊断结果。这里优先使用解析器、算法和明确规则，而不是开放式大模型推理。

第一批 Analyzer 可以包括：

- `hprofx`：HPROF 概览、对象持有关系、retained size、GC root path、线程栈
- Spring AMQP Analyzer：消费者、prefetch、ack mode、客户端队列积压
- JVM Analyzer：GC、线程死锁、类加载、内存区域异常
- Log Analyzer：异常归并、错误频率和关键时间点
- Deployment Analyzer：部署事件、镜像和构建元数据提取
- Git Analyzer：提交差异、blame、文件和符号索引

Analyzer 输出标准化 `Finding`：

```text
finding_id
analyzer
analyzer_version
fact_type
subject
value
evidence_refs[]
severity
rule_id
limitations[]
```

### 边界

分析层可以：

- 对确定的输入产生可重复结果
- 根据明确规则标记已知危险状态
- 展示计算方法、工具版本和限制
- 输出事实或由具体规则支持的局部结论

分析层不可以：

- 在缺少证据时补全数字、配置或调用链
- 把跨系统相关性直接宣布为最终因果关系
- 修改源码或生产环境
- 让特定框架逻辑污染通用 HPROF 解析核心

**这一层回答“材料里能确定地读出什么”。**

## L4：关联层（Correlate）

### 定义

建立 Rootforge 最关键的 Runtime-to-Code 图谱，将事故现场中的实体与实际源代码连接起来。

核心链路是：

```text
Incident
  -> Environment
  -> Node
  -> Service / Container
  -> Image Digest
  -> Build
  -> Repository
  -> Git Commit
  -> File / Symbol / Configuration
```

该层同时建立统一时间线：

```text
部署 -> 指标变化 -> 日志异常 -> 消费积压 -> OOM -> 重启
```

典型输出是 `ContextGraph`，每条边都带来源和可信状态：

- `verified`：由 digest、label、构建证明等直接确认
- `inferred`：通过时间、名称或其他间接信息推断
- `unknown`：尚未建立可靠映射

### 边界

关联层可以：

- 根据不可变标识连接跨系统实体
- 在直接映射缺失时给出明确标注的候选关联
- 生成部署、运行时和代码变更的统一时间线
- 将 Finding 定位到仓库、提交、文件或符号

关联层不可以：

- 仅凭时间接近就宣称某次提交造成事故
- 猜测无法确认的仓库、分支或部署版本
- 读取超出当前 Case 授权范围的代码仓库
- 直接形成最终 RCA 或生成修复

**这一层回答“这些事实分别属于哪个运行实体和哪份代码”。**

## L5：ClayHarness 调查运行时（Investigate）

### 定义

Rootforge 通过 App Server Protocol 接入团队独立开发、独立版本化、独立测试和独立运行的 ClayHarness 服务，通常由生成的 Go SDK 封装协议。ClayHarness 像调查工程师一样执行通用 Agent 循环；Rootforge 将 Case 中的事实映射为通用 Artifact 和工具描述，并通过输出 Schema 赋予运行结果 RCA 或升级请求等事故领域语义。HolmesGPT 可以作为评估基线，但不是运行时依赖。

调查循环：

```text
观察事实
   -> 提出根因候选
   -> 列出候选成立应出现的证据
   -> 调用 Analyzer 或 Connector 补充材料
   -> 支持、降低或排除候选
   -> 记录未知项
   -> 输出 RCA
```

每个关键结论都必须区分：

- `Fact`：证据直接表明的事实
- `Inference`：由事实支持的推断
- `Unknown`：尚无足够材料判断的信息

ClayHarness 的直接输出是符合 Rootforge 所提供 Schema 的通用 `RunResult`。Rootforge 验证并映射后形成版本化 `RCA`：事故摘要、时间线、影响、根因候选、可信度、证据、反证、未知项以及修复建议。

### 边界

调查层可以：

- 选择和调用已授权的只读工具
- 跨运行时、部署、代码和框架 Finding 进行推理
- 要求补充缺失证据
- 对根因候选排序并解释可信度
- 推荐修复方向和验证方法

调查层不可以：

- 绕过 Case 直接获得不受控的生产或仓库权限
- 把模型的常识当成当前事故的事实
- 隐藏互相冲突的证据
- 因为找到一个合理解释就停止寻找关键反证
- 直接执行生产变更

ClayHarness 不直接持有 Docker、GitHub、生产数据库、云平台或其他基础设施凭证，也不能自行执行 Shell。它只能请求当前 Run 声明的工具；Rootforge Tool Gateway 仍必须逐次执行 Case 范围检查和 Policy 检查，在完成受限只读操作并保存 Evidence 与审计记录后，才返回结构化结果。ClayHarness 可以在输出中提出动作，但不能批准或执行动作。

ClayHarness 只拥有可恢复的运行时 checkpoint。Rootforge Case 始终是事故事实、证据、调查历史、RCA、审批和行动记录的唯一权威来源，二者不能各自维护一套调查真相。

**这一层回答“什么最可能导致事故，以及证据是否足够”。**

## L6：修复与行动层（Forge & Act）

### 定义

把已确认或高可信的 RCA 转换为可审查的变更提案，并在隔离环境中验证。随后由 Policy 判断是否允许执行：允许时交给独立 Action Executor；不允许、证据不足或风险过高时，携带完整 Case 通知开发者。

典型流程：

```text
RCA
 -> Change Proposal
 -> Candidate Patch
 -> Static Checks / Tests / Reproduction
 -> Risk and Rollback Plan
 -> Policy Decision
 -> Execute / Request Approval / Notify Developer
 -> Verify
 -> Success / Rollback and Escalate
```

典型输出 `ChangeProposal`：

- 目标仓库和基准提交
- 变更动机及其 RCA 引用
- 补丁和受影响范围
- 测试结果、失败项和未覆盖风险
- 发布、观察与回滚建议
- 审批状态和 Pull Request 引用

### 边界

修复验证层可以：

- 在隔离工作区生成候选补丁
- 运行项目允许的测试、构建和静态检查
- 根据测试失败继续修改候选方案
- 在仓库策略允许或获得审批后创建 Pull Request
- 把白名单内、幂等、可逆的动作交给独立 Action Executor
- 验证动作结果，并在验证失败时请求回滚和升级通知

修复验证层不可以：

- 因为测试通过就自动认定根因正确
- 隐藏失败测试或扩大补丁范围
- 未经 Policy 授权向仓库推送、创建 PR 或修改环境
- 由 ClayHarness 或 Forge 直接使用生产凭证执行动作
- 在 v0.1 中直接部署、重启或修改生产环境
- 将生产密钥和原始敏感证据带入不受控的代码执行环境

**这一层负责“把结论变成安全、可验证、受策略约束的处置”；自动化不等于绕过授权。**

## 横切控制面（Control Plane）

控制面不产生根因结论，而是为每一层施加一致的安全和治理规则：

- 身份与租户隔离
- 最小权限和短期凭证
- Secret 管理与敏感数据脱敏
- 工具、Evidence Source 和 Analyzer 注册
- 允许访问的环境、服务和仓库范围
- 模型调用、数据外发和成本预算策略
- 修复、推送、PR 和生产动作的审批门
- 全链路审计、重放和保留策略

任何层都不能绕过控制面获得额外权限。控制面也不能修改 Analyzer 事实或替 ClayHarness 选择根因。

## 层间契约

为了避免系统最终变成一个拥有全部权限的大 Agent，各层只通过明确产物通信：

| 来源 | 产物 | 消费方 |
| --- | --- | --- |
| Evidence Source | `EvidenceBatch` | Case |
| Case | 证据快照 | Analyzer / Correlator |
| Analyzer | `Finding` | Case / Correlator / ClayHarness Adapter |
| Correlator | `ContextGraph`、Timeline | ClayHarness Adapter |
| ClayHarness | `RunResult`、Event、Checkpoint | Rootforge Adapter |
| Rootforge Adapter | `RCA`、升级请求 | Forge / Notification / Case |
| Forge | `ChangeProposal`、测试结果 | Policy / Case |
| Policy | `PolicyDecision` | Action / Notification / Case |
| Action | `ActionResult`、验证与回滚结果 | Case / Notification |

核心依赖规则：

1. 上层可以读取下层的标准产物，不能绕过下层边界直接扩大权限。
2. 原始 Evidence 不可覆盖；Finding、Graph、RCA 和 Proposal 都需要版本化。
3. 每一个派生产物都必须能引用其输入证据。
4. LLM 输出默认是推断，只有原始证据或确定性 Analyzer 输出才能成为事实。
5. 从只读调查进入写操作，必须经过独立 Policy 决策与 Action Executor。

## 部署边界

逻辑分层不等于所有层都部署在同一个进程中。推荐的物理边界是：

```text
生产环境                  Rootforge 控制平面             隔离执行环境

Node Retriever / Connector -> Case / Analyzer / ClayHarness -> Forge Sandbox -> Action
      只读、低权限               集中推理            候选代码       独立凭证
```

- Node Retriever 在需要时靠近工作负载部署，但不承载开放式 Agent，也不持续向 Rootforge 传输全部日志。
- v0.1 中 Case 与 Correlation 留在 Rootforge；ClayHarness 作为独立服务运行。Rootforge 通过 delegated Execution Host 保留工具策略、凭证、Evidence 持久化和审计权威。
- Forge 使用临时、隔离、无生产凭证的代码工作区。
- Action Executor 使用单独的短期凭证，并且只能执行 Policy 已授权的具体动作。
- 原始大文件可以留在受控存储中，通过引用和按需读取避免重复传输。

## v0.1 实现范围

第一版不需要同时建完六层。应先用最短路径验证产品价值：

| 层 | v0.1 方式 |
| --- | --- |
| L1 Acquire | 接收 Docker OOM 或测试事件；从 Docker、journald 或 Loki 按需取证 |
| L2 Case | 本地目录 + 明确的 Case manifest 与证据索引 |
| L3 Analyze | 集成 `hprofx`，输出 JVM 与 Spring AMQP Finding |
| L4 Correlate | 通过 manifest 映射服务、镜像、仓库和 commit |
| L5 ClayHarness | 无人提问也能基于通用输入和受控工具生成符合 RCA Schema 的结构化结果 |
| L6 Forge & Act | 暂不改生产；输出修复建议并自动通知开发者 |

v0.1 的纵向切片因此是：

```text
收到一次 JVM OOM 事件并自动建案
 -> hprofx 结构化分析
 -> 映射到部署提交和代码
 -> 生成可追溯的 RCA
 -> 自动通知开发者
```

当这个无人值守闭环能够稳定复现今天 RabbitMQ OOM 的关键结论，再扩大数据源、事故类型和受控修复能力。
