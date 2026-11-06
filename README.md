# neomega-converter

`neomega-converter` 在 Bedrock 与 Java 存档之间转换；Bedrock → Java 时还可按邻居补全栅栏连接、楼梯拐角等方块属性。它既提供命令行导出，也提供按区块坐标读取的缓存来源，供浏览器或区域导出使用。

转换来自 [neomega-converter-core](../../neomega-converter-core/README.md)，邻接修复来自 [viewer-core 的 fixer](../../neomega-viewer-core/fixer/java-postprocess/README.md)。两者由本模块组合，调用方可以只转换，也可以转换后修复，或单独修复已有 Java 世界。

## 先导出一份存档

需要 Go 1.25.5 或以上，并保留工作区中的兄弟模块目录，具体依赖由 `go.mod` 指定。下面的命令从 `apps/neomega-converter` 目录执行，转换目标为 Java 1.21.4。


`cmd/bedrock-to-java` 组合存档转换与可选的邻接修复。
LevelDB 打开时可能写文件，所以要对 Bedrock 存档的副本运行。

```sh
go build -o bedrock-to-java ./cmd/bedrock-to-java

# 转换并可选修复邻接属性
./bedrock-to-java -input <Bedrock 世界副本> -output <新建或空的目录> [-dims 0,1,2] [-all-chunks] [-fix] [-strict]

# 对已有 Java 世界只修复邻接属性
./bedrock-to-java -fix-only -output <Java 世界> [-dims 0,1,2]
```

`-dims` 是 Bedrock 维度编号，默认只导出主世界。
`-all-chunks` 也转换未生成完成的柱子；默认按 0x36 记录的 FinalizedState 跳过。
命令结束时打印两步的报告，一行一个数字。

`-strict` 在第一根转换失败的区块柱子处停止；默认继续处理其他柱子并在报告中列出失败项。转换模式的输出目录必须不存在或为空；`-fix-only` 则要求目录已经存在。转换并不保留全部游戏数据，物品、实体及世界元数据的支持范围见 [转换核心的能力一览](../../neomega-converter-core/README.md#能力一览)。写出的世界尚未经过完整的游戏内兼容性验证。

### 反方向：Java → Bedrock

`cmd/java-to-bedrock` 把 Java 1.21.4 存档转成 Bedrock 1.21.50。
目标目录必须不存在或为空。

```sh
go build -o java-to-bedrock ./cmd/java-to-bedrock
./java-to-bedrock -input <Java 世界> -output <新建或空的目录> [-dims 0,1,2] [-strict] [-entity-defs <bedrock_entities.json>]
```

目前写出方块、群系、level.dat 和箱子/熔炉/旗帜/告示牌方块实体；普通实体和其他方块实体还在补。
`-strict` 在第一根失败的柱子处停止，默认继续并列出失败项。
`-entity-defs` 可加载 mcconv 格式的 `bedrock_entities.json`，给实体补 spawn/born 组和
health 的 Base/Max；没有这张表时实体使用 Bedrock 默认行为。

## 结构文件转存档

`cmd/structure-to-world` 复用 converter-core 的流式转换。schem、schematic、litematic 和原版 nbt 写成 Java；mcstructure 和 mcworld 写成 Bedrock。从本模块目录执行：

```sh
go run ./cmd/structure-to-world -input /path/to/城堡.schem
# 指定其他输出位置时，目标必须不存在或为空。
go run ./cmd/structure-to-world -input /path/to/城堡.mcstructure -output /path/to/基岩城堡
```

`-memory-mib` 设置工作预算，默认 512，至少 64；例如 `-memory-mib 1024` 允许约 1 GiB 的映射与并行工作预算。它不是进程内存硬上限。`-workers` 设置 Java 写出任务数上限，默认 0 按 CPU 自动选择（最多 8 个），实际并发仍受预算约束。

默认输出与输入同路径、同名的文件夹，去掉扩展名。取消或失败不留下半成品世界，也不覆盖已有存档。格式、内存策略和兼容性范围见 [核心说明](../../neomega-converter-core/README.md#结构文件写成原生存档)。

## 在 Go 程序中组合

模块导入路径是 `neomega-converter`，不包含目录前缀 `apps/`。一根柱子覆盖水平 16×16 的区域，内部按高度分成 16×16×16 的 section。


整图导出先调用 `neomega-converter-core/export.ConvertWorld` 写出 Java 世界，再按需调用本模块的 `export.FixWorld`。
转换返回时已刷新写入，修复读取的是 Java 数据，因此也能独立处理其他来源的 Java 世界。
命令行负责最后提交或关闭目标世界。

写回仅替换方块状态，保留区块及 section 的其他 NBT。
发生错误时，之前的写入仍然保留；`export.FixWorld` 不提供整图回滚。

已持有内存转换结果时，使用 `FixChunk`。
它返回修复后的方块调色板。
实体、诊断、生物群系和未修改的 section 继续共享输入。
八邻居必须是修复前的结果，顺序为西北、北、东北、西、东、西南、南、东南。

按需浏览使用 `adapt.NewFixed`，其读取与缓存约定如下。

## adapt：按坐标读取与缓存

浏览器和区域导出通常只需要世界中的一部分柱子。
转换 core 的 `adapt` 提供 `Converted` 与 `ChunkSource`，本模块的 `adapt` 提供 `Fixed`。
消费方调用 `Chunk(ctx, pos)`，取得共享只读的 `ConvertedChunk`。
不存在的柱子返回 `nil, nil`。

只需要转换结果时，使用 `Converted`。
它调用读取回调取得 Bedrock 柱子，再执行 `be2je.ConvertChunk`。
需要邻接状态时，在它上面接一个 `Fixed`。
`Fixed` 从同一个 `Converted` 取得中心及八个邻居，再调用 `FixChunk`。
两种来源都实现 `ChunkSource`，消费方可以使用同一套读取代码。

修复可能改变邻居的形状，例如直楼梯变成内拐角后会多出一个完整侧面。
如果再拿这个形状去修复旁边的栅栏，结果就会依赖柱子的访问顺序。
因此 `NewFixed` 的输入类型是 `*Converted`，而不是任意 `ChunkSource`。
这使 `Fixed` 无法被误接到另一个 `Fixed` 上。
调用方仍可同时读取同一个 `Converted` 和它上面的 `Fixed`。
已有其他形式的阶段 1 数据时，可以直接调用 `FixChunk`。

### 冷请求与复用

冷请求指依赖的数据尚未进入缓存。
单独请求一根修复柱子，最多需要读取并转换 3×3 范围的九根柱子，然后修复中心。
渲染器为了剔除被邻居遮住的面，还会请求中心的东、西、南、北四根修复柱子。
这五个位置的 3×3 范围合并后，是 5×5 去掉四个角。
在柱子都存在且转换缓存足够容纳这些输入时，一次这样的渲染需要转换 21 根、修复 5 根。
上下相邻的 section 属于同一根柱子，不再扩大这个水平范围。

`Converted` 缓存完整的阶段 1 结果，包括诊断。
`Fixed` 另存完整的修复结果。
相邻视点和不同高度窗口可以复用这些柱子。
缺失结果也进入缓存，避免在空区域重复查找。
中心不存在时，修复层直接返回缺失结果。
读取或转换失败通过 `error` 返回，下次请求会重试。

两层使用同一套 LRU 实现。
LRU 会在预算不足时淘汰最久没有被访问的条目。
每层通过 `Options.CacheBytes` 单独设置预算，零值为 64 MiB。
负值只合并正在进行的任务，完成后不保留结果。
超过整层预算的单根柱子也只交给当前等待者。

预算按调色板、状态文本、实体 NBT 和诊断的估算字节计费。
条目还计入 map 和链表的估算开销，因此缺失结果也占预算。
两个阶段共享的对象在各自缓存中分别计费。
预算用于限制缓存留存，不等于进程内存上限。
正在计算的柱子、底层读取缓冲以及调用方仍持有的结果都可能占用额外内存。
缓存淘汰只释放缓存的引用，调用方拿到的柱子仍然有效。

### 并发、取消与来源生命周期

同一实例的 `Chunk` 可以并发调用。
两层各自按坐标合并正在进行的任务，这称为单飞。
同时请求同一个坐标的调用者等待同一份结果。
不同坐标可以并行计算，因此读取回调也必须允许并发调用。
同一来源上的多个 `Fixed` 可以共享一个 `Converted` 的转换缓存。

每个调用者的 context 控制自己的等待。
首个调用者超时或取消，也不会打断其他等待者。
最后一个等待者离开时，共享任务的 context 被取消，坐标立即允许重新请求。
读取回调会收到这个共享 context，它保留首个调用者的 context 值。
回调应以共享 context 的取消信号结束可中断的工作。

`ConvertChunk` 和 `Fix` 本身没有中途取消入口。
取消通过阶段边界生效，已经开始的计算可能继续到返回。
不支持中途取消的读取也可能与同坐标的重试短暂重叠。
废弃任务的结果不会进入缓存，也不会替换重试任务。
因此 `Chunk` 返回取消错误，不代表底层读取已经退出。
读取回调的拥有者负责协调正在使用的句柄与关闭操作。

一个 `Converted` 实例绑定一个稳定的数据来源和维度。
切换来源、维度或修改底层数据时，应创建新的来源实例和对应的修复层。
返回值中的 map、调色板、NBT 和诊断都按只读共享。
调用方要修改其中的数据时，先复制需要修改的部分。

### 组装与使用

以下代码中 `adapt` 是 `neomega-converter-core/adapt`，`fixedadapt` 是 `neomega-converter/adapt`，`fixer` 是 `neomega-viewer-core/fixer/java-postprocess`。

先提供一个已打开的 `*cellerbedrock.World`，并选定 Bedrock 维度编号 `dimension`。
下面是省略应用层错误处理的组装伪代码。
回调借用 world；world 的关闭由应用层与读取操作协调。
`cellerbedrock.World.ReadChunk` 在找不到任何区块记录时返回 `fs.ErrNotExist`。
`NewConverted` 会识别这个错误，包括包装后的错误，并将其转换为缺失结果。

```go
read := func(
    ctx context.Context,
    pos celler.ChunkPos,
) (*cellerbedrock.Chunk, error) {
    return world.ReadChunk(dimension, pos)
}

converted := adapt.NewConverted(read, adapt.Options{})
```

只转换时，把 `converted` 交给消费方：

```go
var source adapt.ChunkSource = converted
chunk, err := source.Chunk(ctx, celler.ChunkPos{X: 10, Z: -3})
```

需要修复时，加载 fixer 并组合第二层：

```go
f, err := fixer.New()
// 处理 err 后再组装。
var source adapt.ChunkSource = fixedadapt.NewFixed(
    converted,
    f,
    adapt.Options{},
)
chunk, err := source.Chunk(ctx, celler.ChunkPos{X: 10, Z: -3})
```

viewer 可以把取得的柱子适配成渲染 section。
区域导出可以遍历所需坐标，把柱子传给 `export.ToJavaChunk` 和 `export.ToJavaEntities`。
两种用途共用上述来源，邻居仍从原始 Bedrock 来源读取。
转换和修复的语义及诊断含义见[转换核心说明](../../neomega-converter-core/README.md)与[修复规则](../../neomega-viewer-core/fixer/java-postprocess/FIXER.md)。

开发结构、依赖边界与验收命令见 [AGENTS.md](AGENTS.md)。更详细的调用场景和能力说明见 [转换指南](CONVERSION-GUIDE.md)。
