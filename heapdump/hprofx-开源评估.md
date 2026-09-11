# hprofx 开源评估

**评估对象**：`hprofx/` —— 2,563 行 Go，零依赖 HPROF 堆转储分析器
**评估日期**：2026-09-10
**结论**：**不建议作为通用堆分析器开源。** 已有工具覆盖了它绝大部分能力；但它有一处真实且关键的空白，值得换个定位。

---

## 一、结论摘要

| 问题 | 结论 |
|---|---|
| 有没有更好的工具？ | **有。** `heaptrail`（Rust，2026-05）覆盖 hprofx 全部核心能力，另有 5 项 hprofx 没有的功能 |
| 那还有空间吗？ | **有一条窄缝。** heaptrail 明确不支持任意对象字段读取，而这正是本次排查中定位根因的关键手段 |
| 现在能发布吗？ | **不能。** 零测试、仅在单个转储上验证过、无 LICENSE、模块名不可安装、仓库内含生产数据 |
| 推荐怎么做？ | 自用为主；若要开源，改做**框架感知的诊断器**，而不是第三个通用分析器 |

> 说明：本文修正了此前一次基于印象的判断。当时认为「带支配树的轻量 CLI 是市场空位」，实际调研后发现该位置已被 heaptrail 占据。

---

## 二、hprofx 现有能力

| 命令 | 回答的问题 |
|---|---|
| `summary` | 转储规模、对象数量、GC Root 构成 |
| `histo` | 什么类型占内存最多 |
| `holders` | 这些对象挂在谁身上（按引用者的类聚合） |
| `retained` | 释放哪个对象能回收最多内存（Lengauer-Tarjan 支配树） |
| `paths` | 这个对象为什么还活着（GC Root 最短路径 + 字段名边标签） |
| `inspect` | 它的字段值到底是什么 |
| `threads` | 当时线程在干什么（完整栈重建 + 按相同栈归并） |
| `amqp` | RabbitMQ 积压全貌（路由键、消费者、年龄、ackMode） |
| `report` | 一次跑完，输出 markdown |

**实测性能**（1.1 GB 转储 / 751 万对象 / 1,118 万条引用）：`retained` 3.1 秒，`report` 全流程 5.0 秒。二进制 2.2 MB，静态链接。

---

## 三、现有工具调研

### heaptrail —— 直接竞品

Rust CLI，从 `hprof-slurp` fork 而来并加入投研功能。

| 项 | 值 |
|---|---|
| 最新版本 | v1.4.0 |
| 首次发布 | 2026-05-09 |
| 最后更新 | 2026-05-25（至评估日已停滞 3.5 个月）|
| crates.io 总下载 | **258 次** |
| GitHub | **3 star** / 0 fork / 1 open issue |
| 许可证 | Apache-2.0 |

宣称解析速度约 2 GB/s（多核，建议 ≥4 核）；`--retained-size` 模式在 200 MiB Android 转储上约需 250 MiB 内存、1–3 秒。

### hprof-slurp —— heaptrail 的上游

| 项 | 值 |
|---|---|
| GitHub | **165 star** / 16 fork |
| 创建 | 2021-01-02 |
| 最后提交 | 2026-09-09（活跃）|

单遍流式解析，只做浅堆直方图，**不构建引用图**，因此没有 retained size、支配树和引用链。成熟且活跃，但能力层次低于 hprofx。

### Eclipse MAT —— 事实标准

功能最全，headless CLI（`ParseHeapDump.sh` + `org.eclipse.mat.api:suspects`）可生成泄漏嫌疑报告并支持自定义查询。代价是 GUI 定位、部署笨重，解析大转储本身需要可观的堆。

---

## 四、能力对比

| 能力 | hprofx | heaptrail | hprof-slurp |
|---|---|---|---|
| 类直方图 | ✓ | ✓ | ✓ |
| 支配树 / retained size | ✓ | ✓ `--retained-size` | ✗ |
| path to GC roots | ✓ | ✓ `--paths-from-id` | ✗ |
| 引用者聚合 | ✓ `holders` | ✓ `--group-holders` | ✗ |
| 基本类型数组内容预览 | ✓ | ✓ `--preview-bytes` | ✗ |
| **任意对象字段读取** | **✓ `inspect`** | **✗** | ✗ |
| 线程栈 | ✓ 完整栈 + 归并 | 仅线程名 + 顶层帧 | ✗ |
| 泄漏嫌疑自动排名 | ✗ | ✓ `--leak-suspects` | ✗ |
| 快照 diff / 时间序列 | ✗ | ✓ `--diff-from` / `--diff-series` | ✗ |
| 软弱引用过滤 | ✗ | ✓ `--exclude-soft-weak` | ✗ |
| 分配点栈轨迹 | ✗ | ✓ `--allocation-sites` | ✗ |
| Android（R8 反混淆 / Bitmap）| ✗ | ✓ | ✗ |
| JSON 输出 | ✗ | ✓ | ✓ |
| 并行解析 | ✗ 单线程 | ✓ 多核 | ✓ |
| 框架感知分析（AMQP 等）| ✓ | ✗ | ✗ |

hprofx 此前自列的三项局限 —— 快照 diff、软弱引用区分、泄漏嫌疑排名 —— heaptrail 全部具备。

---

## 五、唯一的真实空白：对象字段读取

heaptrail 的能力表中「任意对象字段读取」明确标注为不支持。这不是边角功能。

回顾本次 OOM 排查的实际路径：

1. `histo` → `byte[]` 占 78% —— **只是现象**
2. `holders` → 748 MB 挂在 `ConsumerDispatcher$5` 上 —— **定位到了组件**
3. `retained` → `ConsumerWorkService` 独占 810 MB —— **定位到了持有者**
4. `inspect` → `acknowledgeMode=NONE` / `prefetchCount=50` —— **解释了原因**

前三步 heaptrail 都能做到，第四步做不到。而**配置错误藏在对象字段里**：栈里没有，直方图里没有，支配树里也没有。没有第四步，结论只能停在「RabbitMQ 积压了」，给不出「把 ackMode 改成 AUTO」这个可执行的修复。

同理，本次的锁竞争链（11 个线程阻塞在同一把 `consumersMonitor` 上，持锁者卡在 `basicCancel` 等 broker 回复）需要**完整栈**才能看出来；heaptrail 的线程支持只到线程名加顶层帧。

---

## 六、发布前的工程缺口

即使定位调整后决定开源，以下四项是硬性前置条件。

### 6.1 仓库内含生产数据（最高优先级）

上级目录的 `rabbitmq-workpool-oom-report.md` 包含：内部队列名与 exchange 名、consumerTag、tenantId、直播间 ID、商品标题、内部包名 `com.wahool.saas.*`、容器主机名。`hprofx/README.md` 第 117 行直接引用了该文件。

**Go 源码本身已确认干净**（无任何公司相关字符串）。

处理方式：新建独立仓库，**不要**对当前目录执行 `git init` —— 否则生产数据会进入 git 历史。

### 6.2 仅在单个转储上验证过

这是最大的技术风险。以下代码路径**从未被执行**：

- JDK 8 的 `char[]` 字符串解码分支（已实现，未运行）
- 32 位 id 的转储（当前直接拒绝）
- Android 的 `HEAP_DUMP_INFO` 记录
- 无栈轨迹的转储（`jmap -F` 生成）
- `amqp -age` 正则回退路径（本次 `BasicProperties.timestamp` 恰好存在，未触发）
- `float[]` / `double[]` / `short` 等类型的值渲染

一个只在单个文件上验证过的二进制格式解析器，他人首次运行即崩溃的概率很高。

### 6.3 零测试与错误处理

2,563 行代码，**没有任何测试**。解析器全部使用 `panic`（`must` / `skip` / `take`），对畸形输入会直接崩溃而非返回错误。

最低要求：手工构造的小 hprof 二进制作为 golden fixture、支配树算法对照已知答案的单元测试、`go test -fuzz` 覆盖畸形输入、解析层改 panic 为 error。

### 6.4 工程基建

- 无 LICENSE（建议 Apache-2.0 或 MIT）
- 无 CI
- 模块名为 `hprofx` 而非 `github.com/<user>/hprofx`，他人无法 `go install`
- README 仅有中文（此类工具受众为全球，英文为刚需）

---

## 七、架构建议

`amqp` 子命令内嵌于核心工具是一处设计味道：把特定中间件的知识焊死在通用分析器里，后续 Kafka、Netty、Hikari 都会想加，很快臃肿。

建议将解析与图分析抽为 `pkg/hprof` 库，`amqp` 改为基于该库的 analyzer。他人可引库编写自己的场景分析器，而非向你提 PR 加中间件。此重构在开源前做成本最低，之后即为破坏性变更。

**若采纳第八节的方向 3，则此项性质改变** —— `amqp` 不再是设计味道，而正是产品定位本身。

---

## 八、建议

### 方向 1：保持自用（最省事，推荐默认）

工具可用、作者熟悉、5 秒出报告。维护公开项目的成本（issue、跨 JVM 版本兼容、多平台支持）换不来对应收益。

### 方向 2：把独特能力贡献给上游

向 `hprof-slurp` 或 `heaptrail` 提交「对象字段读取」与「完整线程栈重建」。实现路径已经验证过：类布局按父类链展平、String 双编码解码（JDK 9+ `byte[]`+`coder` 与旧版 `char[]`）、枚举取 `name` 字段。比重造一个通用工具影响面更大。

### 方向 3：改做框架感知的诊断器

不与通用工具竞争，转做无人覆盖的方向 —— `inspect` + `amqp` 这条线：

- 读出 Spring AMQP 的 `ackMode`，解释 prefetch 为何失效
- 读出 Hikari 连接池配置与泄漏连接
- 读出 Netty `ByteBuf` 泄漏与引用计数

**通用工具告诉你哪块内存大；这类工具告诉你哪个配置错了。** 该方向目前未见同类产品。

---

## 九、法务

工具本身不含公司代码，但产生于排查公司生产问题的过程中，使用了公司时间与设备，**属于职务作品的灰区**。开源前须走公司开源审批流程，不应自行判断。

---

## 十、调研边界

本次仅调研了 heaptrail / hprof-slurp 这一条线，以及 Eclipse MAT 的 CLI 背景。**未系统调研**：JXRay、SJK（jvm-tools）、NetBeans Profiler 库、VisualVM、各类商业 profiler。若最终决定发布，这几项各值得再花十分钟核实。

### 数据来源

- [johnneerdael/heaptrail](https://github.com/johnneerdael/heaptrail) —— GitHub API 直接查询
- [heaptrail on crates.io](https://crates.io/crates/heaptrail) —— crates.io API 直接查询
- [agourlay/hprof-slurp](https://github.com/agourlay/hprof-slurp) —— GitHub API 直接查询
- [Creating and Analyzing Java Heap Dumps](https://reflectoring.io/create-analyze-heapdump/) —— MAT CLI 背景
