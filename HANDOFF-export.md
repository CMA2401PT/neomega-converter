# neomega-converter 下一步：导出能力

这份文档给下一个接手「导出到 Java 存档」的会话用。
工作方式沿用 codex-collab skill：出简报给 Sol，验收自己跑。

先读 [README 的「能力一览」](README.md#能力一览)。
那一节列出了现有转换能力的签名、支持范围、报错条件和降级情况，这里不再重复。

## 目标

导出这一侧也要提供多种粒度、可以单独使用的能力，命令行只是把它们串起来的一个调用方。
不要把读世界、转换、修复、写出合成一个大入口，让调用方只能整图跑。
每一层都应能单独调用和测试，例如只把一根 `ConvertedChunk` 变成 `java.Chunk`，或只转换 `level.dat`。

这一轮先做不追求速度和并发的基础实现，以验证转换正确性为主。
fixer 可能很慢（每根柱子要读 8 个邻居），因此修复是调用方可选的一步：
可以只转换（快，邻接状态近似），也可以转换后修复（慢，邻接状态精确）。

## 已完成的状态（2026-11-01）

- `bedrock.ConvertChunk`：`*cellerbedrock.Chunk` → `*converter.ConvertedChunk`，含方块实体。
- `fixer.Fix`：区块级邻接修复，`ChunkNeighbours [8]*ConvertedChunk`。
- celler/bedrock：`World.ScanChunks` 顺序扫描快照，`ReadChunk` 随机读取，`ReadLevel` 读 `level.dat`。
- celler/java：`NewChunk`、`SetSection`、`WriteChunk`、`WriteLevel`，region 格式已就绪。
- 留位：`cmd/bedrock-to-java/main.go`（只有 package main），`adapt.OnDemandConverter`（空类型）。

还没实现：生物群系、普通实体、旗帜图案、展示框里的地图。
展示框已转成 `ConvertedChunk.Entities` 里的 Java 实体，导出时要写进 `entities/` region，见 CONVERSION-GUIDE 3.4。

## 边界

核心能力（`converter.go`、`bedrock/`、`fixer/`）不依赖「是否导出整图」的决策。
导出能力放在 `adapt/`，命令行放在 `cmd/bedrock-to-java/`。
不改动 `bedrock/`、`fixer/`、`converter.go` 的公开接口；需要新增诊断种类时只做追加。

## 建议的导出能力

下面是建议的形状，不是定稿，实现时可以调整命名。
要保持的是：每一项输入输出明确，能单独调用，报错和降级写进 README 的能力一览。

```text
adapt.ToJavaSection(y, *ConvertedSection, opts) → (*java.Section, err)
  Blocks 直接用；Biomes 为 nil 时不写
  需要决定 waterlogged 的处理（见下文「写出时要处理的事」）

adapt.ToJavaChunk(*ConvertedChunk, dataVersion, opts) → (*java.Chunk, err)
  对每个 section 调 ToJavaSection，再 SetSection
  BlockEntities 写成 NBT 的 block_entities 列表（java.Chunk 没有写方块实体的方法）
  写 Status 等让游戏把它当作已完成区块的字段

adapt.JavaDimension(bedrockDim int32) → (java.Dimension, ok)
  0 → Overworld，1 → Nether，2 → End，其它 ok=false

adapt.ConvertLevel(cellerbedrock.Level, dataVersion) → (nbt.Document, err)
  Level 只有 Version 和 NBT 两个字段，名称和出生点要从 NBT 里取
  LevelName → Data.LevelName；SpawnX/Y/Z → Data.SpawnX/Y/Z
  其余字段写 Java 默认值；字段缺失或类型不对时的行为要写进能力一览

邻居查找（可选的小工具，不是必需）：
  adapt.Neighbours(lookup func(dim, pos) *ConvertedChunk, dim, pos) → fixer.ChunkNeighbours
```

命令行把这些串起来：

```text
cmd/bedrock-to-java -input <bedrock 世界> -output <java 世界> [-dim 0] [-fix]
  1. cellerbedrock.Open(input, ReadOnly)；java.OpenDirectory(output)
  2. ReadLevel → ConvertLevel → WriteLevel
  3. ScanChunks，只保留目标维度，ConvertChunk 存入 stage1
  4. 每个 pos：out = fix ? Fix(stage1[pos], Neighbours(stage1)) : stage1[pos]
  5. ToJavaChunk(out) → WriteChunk；最后 Commit / Close
  6. 打印统计：区块数、ConvertChunk 失败数、各诊断种类的合计、未知状态前 N 个
```

基础实现不做流式、并行、进度条和增量导出。

## 写出时要处理的事

这些是整理能力一览时从代码里确认的，写出层必须做决定。

- **waterlogged**：阶段 1 会把它附加到任何第二层是水的格子上，包括石头和空气。
  它是本仓库 IR 的扩展属性，`java.BlockProperties` 的属性表里没有它，连 `oak_slab` 也没有。
  Java 原版只有部分方块能含水，写出时需要一份「哪些方块可含水」的数据，目前仓库里没有。
  可选做法：找到或生成这份数据后，可含水的保留、其余去掉（空气含水可写成 `water`）；
  或者先全部去掉，把丢失含水记成降级。选哪种要写进能力一览。
- **Status 和其它区块字段**：`java.NewChunk` 只写 `xPos`、`zPos`、`DataVersion`、`sections`。
  缺少 `Status`（应为 `minecraft:full`）的区块可能被游戏当作未完成区块重新生成。
  高度图、光照是否需要写，要用实际游戏加载验证。
- **方块实体**：`ConvertedChunk.BlockEntities` 是 `[]java.BlockEntity`，NBT 里已带 `id` 和世界坐标 `x/y/z`。
  `java.Chunk` 只有读方块实体的方法，写出层需要自己写 `block_entities` 列表。
- **ConvertChunk 返回 err**：整根柱子没有输出，例如展平前的旧调色板。
  整图时建议跳过并计数，另给一个严格模式在第一个错误处停下。
- **没有诊断的丢失**：没有注册规则的方块实体（熔炉、木桶、漏斗等）整条被忽略，`Diagnostics` 里看不到。
  如果希望导出统计里能看到，可在包装层自己统计源实体 id，或在 `converter.go` 追加一个诊断种类。
  后者改动核心包，先和用户确认。
- **维度**：`ScanChunks` 给出所有维度。只导出 `-dim` 指定的维度，用 `JavaDimension` 选目录。
- **DataVersion**：目标 Java 1.21.4，DataVersion 4189，与 block-converts 的 `JavaTargetVersion` 一致。

## 验收标准

1. `./check.sh neomega-converter` 全通过；新增的每个导出能力有单独的测试。
2. 对 `resource/基岩版` 的副本跑命令，输出目录能被 `java.World.ReadChunk` 读回。
3. 读回几个区块，section 里的方块状态能用 `java.HashToBlock` 查到；方块实体能用 `Chunk.BlockEntities` 读回。
4. 开 `-fix` 和不开都能跑完，不 panic。
5. 新增能力的报错和降级补进 README 的能力一览。

## 参考

- `分析mcconv/06-区块与存档存储/README.md`：celler 的 region 格式细节。
- `分析mcconv/08-实现路线/README.md`：mcconv 的整体流程。
- `celler/bedrock/world.go`：`ScanChunks`、`ReadLevel`、`Level`。
- `celler/java/world.go`：`WriteChunk`、`WriteLevel`、`Commit`、`Close`。
- `celler/java/chunk.go`：`NewChunk`、`SetSection`、`Section`、`BlockEntities`。

## 给新会话的提醒

- Sol 简报要转述 codex-collab 的代码和文档规范。
- celler 已在 go.mod 里有 require/replace，不需要再加。
- LevelDB 打开时可能写文件，跑真实存档前先复制 `resource/基岩版`。
- 这份文件是交接用的临时文件，完成验收后删除它。
