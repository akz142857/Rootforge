# hprofx

一个零依赖的 Java 堆转储（HPROF）分析器。直接读二进制格式，不需要 Eclipse MAT、VisualVM 或 `jhat`（JDK 9 起已移除）。

1.1 GB 的转储，完整分析约 **4 秒**。

## 构建

```sh
make            # 当前平台的二进制 → ./hprofx
make test       # go vet + gofmt 检查
make install    # 安装到 ~/.local/bin/hprofx
make dist       # 交叉编译全平台 → dist/
make clean
```

只依赖 Go 标准库，`CGO_ENABLED=0` 静态链接，产物约 2.2 MB。

`make dist` 产出四个平台的二进制：

```
dist/hprofx-darwin-arm64
dist/hprofx-darwin-amd64
dist/hprofx-linux-amd64     ← 静态链接，可直接丢进任何容器（含 Alpine / scratch）
dist/hprofx-linux-arm64
```

堆转储通常产生在 Linux 服务器上而不是本地，把对应的 Linux 二进制 scp 过去直接跑，比把 1 GB 的 dump 拉回本地快得多：

```sh
scp dist/hprofx-linux-amd64 server:/tmp/hprofx
ssh server '/tmp/hprofx /var/dumps/heap.hprof report -o /tmp/analysis.md'
scp server:/tmp/analysis.md .
```

`hprofx version` 查看版本。

## 用法

```
hprofx <dump.hprof> <command> [flags]
```

| 命令 | 作用 |
|---|---|
| `summary` | 文件头、对象数量、GC Root 统计 |
| `histo` | 类直方图 `[-top N] [-by count\|shallow]` |
| `holders` | **谁在持有某个类型** `[-class REGEX] [-top N]` |
| `retained` | 支配树 + retained size + 最大占用对象 `[-top N] [-min MB]` |
| `threads` | 重建线程栈，按相同栈归并 `[-frames N]` |
| `amqp` | RabbitMQ 积压：路由键、消费者、消息年龄、ack 模式 `[-top N] [-samples N] [-age REGEX]` |
| `inspect` | 打印实例字段 `-class REGEX [-limit N] [-field REGEX]` |
| `paths` | 从 GC Root 到对象的最短引用路径 `-class REGEX [-limit N]` |
| `report` | 跑完整流程并输出 markdown `[-o FILE]` |

全局：`-q` 静默进度输出。

## 排查一次内存问题的典型顺序

```sh
# 1. 什么东西大？
hprofx heap.hprof histo -top 20

# 2. 谁在持有它？（histo 里排第一的类型）
hprofx heap.hprof holders -class 'byte\[\]'

# 3. 从对象图看，哪个对象真正吃掉了堆？
hprofx heap.hprof retained -top 20

# 4. 它为什么还活着？
hprofx heap.hprof paths -class '^com\.example\.SessionCache$' -limit 3

# 5. 看具体字段，确认配置或状态
hprofx heap.hprof inspect -class 'SessionCache$' -field 'maxSize|ttl|entries'

# 6. 当时线程在干什么？
hprofx heap.hprof threads

# 7. 如果是 RabbitMQ 场景
hprofx heap.hprof amqp

# 或者一把梭，生成完整报告
hprofx heap.hprof report -o analysis.md
```

## `amqp` 子命令

客户端侧的消息积压是一种很容易被误读的 OOM 形态：直方图只会说 `byte[]`，而放任积压发生的那几个配置项藏在对象字段里，栈里一点痕迹都没有。`amqp` 把整张图一次性拉出来：

- 缓冲在哪里（客户端 `ConsumerWorkService` WorkPool / Spring 侧 `Delivery`）
- 按 exchange + routingKey、按 consumerTag 的分布
- 消息体大小分位
- **积压年龄**：优先用标准的 `BasicProperties.timestamp`；producer 没设的话，用 `-age` 传一个正则，第一个捕获组是毫秒时间戳，例如 `-age '"ts":(\d{13})'`
- Spring AMQP 消费者配置表（队列、并发数、prefetch、**ackMode**、Spring 内部缓冲深度），`ackMode=NONE` 会被标红警告

delivery 对象是**按结构识别**的（任何持有 `com.rabbitmq.client.Envelope` 的对象），不依赖硬编码类名 —— 所以客户端的 `ConsumerDispatcher$N` 匿名类、Spring 的 `Delivery`、以及以后改名的版本都能认出来。

`holders` 通常是最关键的一步。直方图只告诉你 `byte[]` 占了 78%，`holders` 才告诉你这些 `byte[]` 挂在谁身上 —— 这才是能直接对应到代码的信息。

## 设计要点

- **流式解析，三遍扫描**：第一遍读元数据（字符串表、类定义、字段布局、线程栈），第二遍建对象索引，第三遍读引用边。不需要边的命令（`histo`、`threads`、`inspect`）跳过第三遍。
- **内存占用与对象数成正比，而非与堆大小成正比**：每个对象只存 id / 类 / 大小 / 文件偏移（约 28 字节）。需要字段值时按偏移随机读回原文件，所以 `inspect` 和 `paths` 很便宜。
- **id 索引用排序数组 + 二分**，不用 map。750 万对象省下约 350 MB。
- **retained size 用 Lengauer-Tarjan**（带路径压缩，COMPRESS 改写成迭代形式避免爆栈）。堆图环多且不可约，简单的迭代式支配点算法（Cooper-Harvey-Kennedy）在这种图上收敛极慢 —— 实测 20 轮仍未收敛，只能截断出近似值；LT 一次算完，3 秒。
- **字段值渲染解析一层**：String 解码（兼容 JDK 9+ 的 `byte[]`+`coder` 与旧版 `char[]`）、枚举取 `name`、装箱类型取 `value`、常见集合显示 size、短数组直接展开元素。所以 `inspect` 一行就能看清 `acknowledgeMode NONE` 和 `queues String[1]{"some-queue"}`。

## 限制

- 只支持 64 位 id 的转储（`idSize=8`）。绝大多数现代 JVM 都是。
- HPROF 不记录引用强度，因此仅靠软/弱引用存活的对象在这里同样显示为可达。
- 「按类汇总 retained」一栏是嵌套计数，用于排序，不能相加。
- 未实现的 MAT 功能：OQL、直方图对比（diff 两个转储）、未使用内存估算。

## 来源

从一次线上 OOM 排查中提炼出来 —— RabbitMQ 客户端 WorkPool 积压 47,584 条消息占掉 810 MB。完整分析见上级目录 `rabbitmq-workpool-oom-report.md`。
