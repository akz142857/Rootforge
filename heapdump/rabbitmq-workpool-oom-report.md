# RabbitMQ WorkPool 积压导致 OOM — 堆转储分析报告

**分析对象**：`heapdump.eab26338f957.hprof.19`（1,185,965,997 字节 / 1.10 GB）
**格式**：JAVA PROFILE 1.0.2，64 位指针（idSize=8）
**转储时刻**：2026-09-09 23:03:45.648 UTC
**JVM**：Java 17（`Thread.run` @ Thread.java:840）
**应用**：Spring Boot + Spring AMQP + MyBatis + MySQL + Tomcat（`com.wahool.saas`）

---

## 一、结论

`wand-ws-to-hub-queue` 队列的消费者使用了 **`AcknowledgeMode.NONE`（no-ack）**。RabbitMQ broker **不对 no-ack 消费者施加 QoS/prefetch 限流**，而 RabbitMQ Java 客户端的 `ConsumerWorkService.WorkPool` 队列是**无界**的。

两者叠加的结果：配置中的 `prefetch=50` 完全失效，broker 把队列里的消息不受限地推给客户端，**47,584 条消息（749 MB）滞留在客户端内存**，占满整个堆并触发 OOM。

同进程内使用 `AcknowledgeMode.AUTO` 的其他三个队列积压分别为 0、0、103 条 —— 反压机制正常工作，形成了直接对照。

---

## 二、堆概况

| 指标 | 值 |
|---|---|
| 存活对象总量 | **1063.40 MB** |
| 对象总数 | 7,517,744（实例 4,310,818 / 对象数组 346,784 / 基本类型数组 2,838,566）|
| 类数量 | 21,576 |
| GC Roots | 7,463（sticky_class 3,663 / java_frame 3,476 / jni_global 233 / thread_object 91）|
| **不可达垃圾** | **6.85 MB** |
| 线程数 | 91 |

> 不可达对象仅 6.85 MB，意味着堆中几乎全是**存活可达**对象。GC 无论怎么跑都回收不了 —— 这是真实内存占用问题，不是 GC 调优问题。

### 浅堆占用 Top 10

| 类 | 浅堆 (MB) | 实例数 | 占比 |
|---|---:|---:|---:|
| `byte[]` | **832.98** | 2,818,536 | **78.33%** |
| `java.lang.String` | 28.14 | 983,451 | 2.65% |
| `java.math.BigDecimal` | 20.77 | 453,712 | 1.95% |
| `byte[][]` | 14.24 | 116,696 | 1.34% |
| `ScheduleProductDashboardEntity` | 13.94 | 114,212 | 1.31% |
| `java.lang.reflect.Method` | 12.00 | 86,219 | 1.13% |
| `HashMap$Node[]` | 9.73 | 72,518 | 0.92% |
| `ConcurrentHashMap$Node` | 9.68 | 230,648 | 0.91% |
| `HashMap$Node` | 9.51 | 226,700 | 0.89% |
| `Object[]` | 8.37 | 65,610 | 0.79% |

`byte[]` 独占 78%，但**没有任何一个数组超过 1 MB** —— 是 280 万个小数组的堆积，而不是单个大缓冲区。

---

## 三、持有链（支配树）

```
ConsumerWorkService @0xc4677a88                        retained 809.92 MB
 └─ WorkPool @0xc4677a68                               retained 809.92 MB
     └─ HashMap @0xc5e01868  (每个 channel 一条队列)    retained 809.85 MB
         └─ HashMap$Node[1040]
             ├─ VariableLinkedBlockingQueue  (×18)      单条最大 97.66 MB
             │   └─ VariableLinkedBlockingQueue$Node    (共 47,167 个)
             │       └─ ConsumerDispatcher$5            (共 47,584 个)
             │           ├─ val$body      → byte[]      ← 精确持有 748.68 MB
             │           ├─ val$envelope  → Envelope
             │           └─ val$properties → AMQP$BasicProperties
```

**`byte[]` 直接引用者归类**（精确统计，非估算）：

| 直接引用者 | MB | 数组个数 |
|---|---:|---:|
| **`ConsumerDispatcher$5`** | **748.68** | 47,480 |
| `java.lang.String` | 37.73 | 983,062 |
| `byte[][]`（MySQL 结果集行） | 34.92 | 1,625,402 |
| `LongStringHelper$ByteArrayLongString`（AMQP header） | 6.19 | 143,075 |
| `NativePacketPayload`（MySQL 协议缓冲） | 1.43 | 28 |

---

## 四、积压详情

### 4.1 总量

| 项 | 值 |
|---|---|
| 积压消息数 | **47,584 条** |
| 消息体总大小 | **749.47 MB** |
| 平均消息体 | 16.1 KB |
| 大小分布 | min 414 B / p50 2,416 B / p90 54,571 B / p99 121,013 B / **max 163,265 B** |

### 4.2 按 exchange / routingKey

| exchange / routingKey | 消息数 | MB |
|---|---:|---:|
| `wahool.saas.topic` / `wand.ws.to.hub` | **47,429** | **749.32** |
| `wahool.saas.topic` / `wand.ws.to.hub.ordered` | 145 | 0.14 |
| `wahool.saas.topic` / `wahool.saas.live.chat.receive.message` | 10 | 0.01 |

### 4.3 按消费者（consumerTag）

18 个 consumerTag 分摊积压，最大单个 5,760 条 / 89.34 MB，最小 10 条。分布均匀，说明不是单个消费者卡死，而是**整个队列的消费能力集体崩溃**。

### 4.4 积压时长分布

以消息体内 `traceId` 前缀的 epoch 毫秒计算，相对转储时刻：

| 时间区间 | 消息数 |
|---|---:|
| 0 – 1 分钟 | 0 |
| 1 – 5 分钟 | 0 |
| 5 – 15 分钟 | **0** |
| 15 – 30 分钟 | 78 |
| 30 – 60 分钟 | 6,842 |
| 60 – 120 分钟 | 24,678 |
| 120 – 180 分钟 | 15,976 |

分位数：p1=34.3 min / p25=73.1 / **p50=101.5** / p75=130.1 / p99=156.4 / **max=157.7 min**

> **关键信号：最近 15 分钟内没有任何新消息进入积压。**
> 积压从转储前约 158 分钟开始堆积，在转储前 15 分钟**完全停止增长**。这不是消费变慢，而是 JVM 在那一刻已进入 GC 死亡螺旋 —— 堆耗尽、Full GC 反复、应用线程近乎全停，连 socket 都读不动了。

### 4.5 消息内容

积压的是 TikTok 直播看板推送：

```json
{"handler":"LIVE_TIK_BOX","fromClient":"d63162e2f03c40c0…","type":"application",
 "scope":"live_center_control","tenantId":"1599","traceId":"1788992734455-btyoly",
 "command":{"namespace":"live_watch","action":"app:dashboard:products"},
 "payload":"{\"segments\":[{\"stats\":[{\"id\":\"1732588596221678178\",\"exposure_cnt\":858,…"}
```

体积最大的一类是 `store_flash_sale` / `app:flash_sale:sync_all_goods` 的**全量商品列表推送**，单条 54–159 KB。

---

## 五、消费者配置

从堆中直接读取的 20 个 `BlockingQueueConsumer` 实例：

| 队列 | 消费者数 | prefetch | **ackMode** | 积压 |
|---|---:|---:|---|---:|
| **`wand-ws-to-hub-queue`** | **16** | 50 | **`NONE`** | **47,429 条 / 749 MB** |
| `wand-ws-to-hub-queue-ordered` | 1 | 250 | `AUTO` | 103 条 |
| `postman.message.event.queue.saas` | 1 | 250 | `AUTO` | 0 |
| `wahool.saas.live.chat.receive.message…` | 1 | 250 | `AUTO` | 0 |
| `wahool.saas.schedule.push.restart.con…` | 1 | 250 | `AUTO` | 0 |

对应容器配置（`SimpleMessageListenerContainer`）：`concurrentConsumers=8`、`maxConcurrentConsumers=16`、`prefetchCount=50`、`batchSize=1`。

其中 2 个消费者的 `abortStarted` 已置位（转储前 164 秒和 102 秒），说明容器已在尝试中止消费者。

---

## 六、线程栈证据

### 6.1 锁竞争链

```
#4-8   [WAITING_TIMED]  持有 consumersMonitor
       BlockingQueueConsumer.basicCancel:458
        → RabbitUtils.cancel:187 → $Proxy422.basicCancel
        → PublisherCallbackChannelImpl.basicCancel:613
        → ChannelN.basicCancel:1498
        → AMQChannel$BlockingRpcContinuation.getReply:505
        → BlockingCell.get  ← 等 broker 的 Cancel-Ok 回复

#4-1,2,3,4,9,10 +4 more  [PARKED]  ×10
       SimpleMessageListenerContainer.isActive:691   ← 阻塞在 consumersMonitor

#4-16  [PARKED]
       SimpleMessageListenerContainer.considerStoppingAConsumer:803  ← 同一把锁
```

**11 个消费者线程被同一把 `consumersMonitor` 锁堵死**，持锁者卡在等 broker 的 RPC 回复。

### 6.2 投递侧背压

```
pool-4-thread-3, pool-4-thread-4  [WAITING_INDEFINITELY]  ×2
       ConsumerWorkService$WorkPoolRunnable.run:111
        → ConsumerDispatcher$5.run:149
        → BlockingQueueConsumer$InternalConsumer.handleDelivery:1025
        → LinkedBlockingQueue.put:343   ← Spring 内部缓冲区已满
```

RabbitMQ 客户端的 work-pool 线程阻塞在 Spring 的有界内部队列上，无法继续消费 WorkPool 队列 —— WorkPool 于是无限膨胀。

### 6.3 业务线程本身也慢

```
#4-7   [RUNNABLE]  MyBatis update → DefaultParameterHandler.setParameters
#3-1   [RUNNABLE]  MySQL ServerPreparedStatement → BinaryResultsetReader 读结果集
i-scheduling-6  [RUNNABLE]  ResultSetImpl.getString → InternalTime.toString → String.format
```

---

## 七、次要问题

### 7.1 单次查询加载 114,212 行

单个 `org.apache.ibatis.executor.result.DefaultResultHandler` retained **65.34 MB**，持有 114,212 个 `ScheduleProductDashboardEntity`（连带 453,712 个 `BigDecimal`、116,505 个 `ByteArrayRow` + 116,696 个 `byte[][]`）。

对应的正是 `app:dashboard:products` 这个 action —— 既是内存放大源，也是消费变慢的直接原因。

### 7.2 日志序列化在热路径上重建 ObjectMapper

```
AsyncAppender-Worker-ASYNC_JSONFILE  [RUNNABLE]
       JsonFileAppender.append:148 → JsonUti.toJsonString:28
        → JavaTimeModule.<init>:140  ← 每次序列化都 new 一个 JavaTimeModule
```

`JsonUti.toJsonString` 每次调用都在构造 `JavaTimeModule`（进而 `SimpleModule.addSerializer` 填 HashMap）。这在日志热路径上是显著的 CPU 和垃圾开销。

### 7.3 AspectJ 切点匹配缓存

107,819 个 `ShadowMatchImpl` + 107,819 个 `ExposedState`（合计 14 MB）。数量与消息量同阶，说明 Spring AOP 切点匹配结果在按调用累积而非按方法缓存。量级不致命，但值得关注。

---

## 八、修复建议

按优先级排列：

### P0 — 把 `wand-ws-to-hub-queue` 的 ackMode 从 `NONE` 改为 `AUTO`

这是唯一能真正建立反压的开关。改完之后 `prefetch=50 × 16 consumers` 最多让客户端持有 800 条消息（约 12 MB），而不是 47,584 条（749 MB）。

```java
@RabbitListener(queues = "wand-ws-to-hub-queue",
                ackMode = "AUTO")          // 或容器工厂里 setAcknowledgeMode(AcknowledgeMode.AUTO)
```

> 注意：`AcknowledgeMode.NONE` 语义上等价于「投递即丢弃、不保证送达」。如果当初选它是为了避免 ack 开销，代价就是失去全部流控 —— 这笔交易在本次事故中已经证明是亏的。

### P1 — 如果业务确实必须 no-ack

- 改用 `DirectMessageListenerContainer`：消息直接在客户端 I/O 线程分发，不经过 `ConsumerWorkService.WorkPool` 二次缓冲，消除无界队列。
- 或给 `CachingConnectionFactory` 配置自定义的有界 executor，并设置 `ConnectionFactory.setWorkPoolTimeout()`，让 WorkPool 满时快速失败而不是无限堆积。

### P2 — 限制单条消息体积

159 KB 的 `sync_all_goods` 全量商品推送应改为增量推送，或把 payload 放对象存储、消息里只带引用。当前 p90 已达 53 KB，对一个高频推送队列而言过重。

### P3 — 给 `ScheduleProductDashboardEntity` 查询加分页或流式处理

114,212 行一次性加载进 `DefaultResultHandler`。改为分页，或用 MyBatis 的 `ResultHandler` 流式回调 + `fetchSize`，避免全量物化。

### P4 — 收敛 `basicCancel` 的等待

给 `SimpleMessageListenerContainer` 设置 `setShutdownTimeout()`，避免持有 `consumersMonitor` 长时间等待 broker RPC 回复而拖垮整个容器的 11 个线程。

### P5 — 缓存 `ObjectMapper`

`JsonUti.toJsonString` 里把 `ObjectMapper`（含 `JavaTimeModule`）改为静态单例，不要每次调用重建。

---

## 九、分析方法与工具

本机未安装 Eclipse MAT / VisualVM，且 JDK 25 已移除 `jhat`，因此直接编写了一个零依赖的 HPROF 解析器 **`hprofx/`** 完成全部分析。它直接读二进制格式，单次全量分析 1.1 GB 转储约 4 秒。

复现本报告各节的命令：

| 报告章节 | 命令 |
|---|---|
| 二、堆概况 | `hprofx <dump> summary` / `histo` |
| 三、`byte[]` 归属 | `hprofx <dump> holders -class 'byte\[\]'` |
| 三、支配链与 retained | `hprofx <dump> retained -top 20` |
| 五、消费者配置 | `hprofx <dump> inspect -class 'BlockingQueueConsumer$' -limit 20 -field 'queues|prefetchCount|acknowledgeMode'` |
| 四、积压详情（全节） | `hprofx <dump> amqp` |
| 六、线程栈 | `hprofx <dump> threads` |
| 存活原因追溯 | `hprofx <dump> paths -class '^com\.rabbitmq\.client\.impl\.ConsumerWorkService$'` |
| 全量报告 | `hprofx <dump> report -o analysis.md` |

`amqp` 子命令按结构识别 delivery 对象（任何持有 `com.rabbitmq.client.Envelope` 的对象），因此同时覆盖客户端 WorkPool 里的 `ConsumerDispatcher$5` 和 Spring 侧的 `Delivery`。用它复跑本次转储，比第四节多找到 **205 条**缓冲在 Spring 内部队列的消息（总计 47,789 条 / 751.05 MB）。

积压年龄现在直接取标准的 `BasicProperties.timestamp`（47,789 条中有 47,779 条带了时间戳），不再需要从消息体 `traceId` 里抠。两种算法结果一致：15–30 分钟桶同为 78 条，最新一条消息为转储前 26 分钟。

### 准确性说明

- **精确值**：类直方图、`byte[]` 引用者归类（748.68 MB）、消息条数与总字节（47,584 条 / 749.47 MB）、消息体大小分位、积压时长分布、线程栈、消费者配置 —— 均为逐对象精确统计。
- **retained size 亦为精确值**：使用 Lengauer-Tarjan 支配树算法（带路径压缩）对完整引用图求解，根节点为连接所有 GC Root 的虚拟节点。`ConsumerWorkService` 的 809.92 MB 与独立统计出的 748.68 MB 消息体 + envelope / properties / 队列节点开销相互印证。
- **报告中「按类汇总 retained」一栏不可相加** —— 链表结构下每个节点都支配其后继，`VariableLinkedBlockingQueue$Node` 汇总出的 1.7 TB 是嵌套重复计数的产物，不代表真实占用。
- **仅记录强引用**：HPROF 不区分软/弱/虚引用的可达性，因此仅靠 SoftReference 存活的对象在本分析中同样显示为可达。
