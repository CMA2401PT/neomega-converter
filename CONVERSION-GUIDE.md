# neomega-converter 转换能力：现状、用法与组装指南

这份文档给需要调用转换器的人看，也给下一次接手开发的人看。
内容分三部分：已实现什么、每个模块怎么调用、还没做什么以及为什么。

---

## 一、整体结构

转换分两个独立阶段，可以单独调用，也可以组合：

```
Bedrock 区块
    │
    ▼  bedrock.ConvertChunk（阶段 1）
ConvertedChunk（已转换，可缓存）
    │
    ▼  fixer.Fix（阶段 2，可选）
ConvertedChunk（已修复）
    │
    ▼  消费方自行适配
viewer、Java 存档写出……
```

阶段 1 只依赖当前区块，结果可以缓存，供多个邻居修复共享。
阶段 2 需要中心区块和周围 8 个区块的阶段 1 结果。

**fixer 是可选的**。如果只需要方块内容正确（床颜色、含水、物品），不在意栅栏连接或楼梯拐角，
可以跳过阶段 2，直接消费阶段 1 的结果。

---

## 二、各模块的调用方式和适用范围

### 2.1 方块状态基础转换（不走 ConvertChunk）

适用：只想转换单个 Bedrock 方块状态，不需要 NBT 或邻居。

```go
// 仅转换方块状态：先对齐到目标 Bedrock 版本，再转 Java
nemc, ok := converts.ConvertFromBedrock(bedrockBlock.State.Tidy())
state := ir.IRBlock("minecraft:air []")
if ok {
    state, ok = converts.ConvertToJava(nemc)
}
if !ok {
    state = ir.IRBlock("minecraft:air []")
}
```

局限：不处理含水、不读实体 NBT、不修复邻接状态。

---

### 2.2 阶段 1：整根区块转换

适用：给定一个 `*cellerbedrock.Chunk`，得到完整的 Java 方块调色板 + 方块实体。

```go
converted, err := bedrock.ConvertChunk(src)
// converted.Sections[y].Blocks  → Palette[ir.IRBlock]
// converted.BlockEntities        → []java.BlockEntity（绝对坐标 + NBT）
// converted.Diagnostics          → 未知方块、NBT 错误等
```

包含：方块调色板转换、含水、所有已注册的方块实体规则（床、花盆、旗帜、潜影盒、
唱片机、讲台、音符盒 note、头颅、箱子双箱配对 + 物品、告示牌）。

不包含：邻接状态（栅栏连接、楼梯拐角等）、生物群系、普通实体。

---

### 2.3 方块实体规则（单格）

适用：只想对一个格子应用方块实体规则，不走完整区块流程。

按 id 选规则的注册表在 `bedrock` 包内部，没有导出。调用方自己按 ID 选子包：

```go
result, err := bed.Convert(blockentities.Input{
    Base:     javaState, // 已转换的 Java 状态
    Source:   nbtCompound,
    LocalPos: [3]uint8{x, y, z},
    WorldPos: celler.BlockPos{X: wx, Y: wy, Z: wz},
})
```

`err!=nil` 表示规则执行失败，调用方保留 Base 状态。
各规则的报错条件和降级见 [README 的能力一览](README.md#能力一览)。

---

### 2.4 物品转换（单个物品）

适用：把 Bedrock 物品 compound 转成 Java 物品 compound，供箱子等方块实体规则调用，
也可以单独调用。

```go
javaItem := items.ConvertItem(bedrockItemCompound)
// nil 表示无法完整转换，调用方跳过该条目
```

支持：方块物品（Block compound、legacy data value 或名称转出 Java 方块）、药水/药水箭（potionIDs 编号表）、
书本（written_book/writable_book）、附魔。

不支持：工具/武器/盔甲/食物（需要 Bedrock→Java 物品名映射表，见 `bedrock/items/doc.go`）、嵌套容器。

---

### 2.5 阶段 2：fixer（单区块，需要邻居）

适用：给定阶段 1 的结果，补全邻接依赖的状态。

**整根区块版**（整图导出、按需模式）：
```go
f, err := fixer.New()
fixed := f.Fix(center, fixer.ChunkNeighbours{
    fixer.NW: nwChunk, fixer.N: nChunk, /* … */
})
```
邻居为 nil 时，对应方向按空气处理。

**单 section 版**（viewer 的上传预览，没有完整区块上下文）：
```go
fixed := f.FixSection(section, [6]*layer_storage.Palette[ir.IRBlock]{
    // west, east, down, up, north, south
    nil, nil, nil, nil, nil, nil,
})
```

已实现的 pass（按执行顺序；各自的降级见 README 的能力一览）：
1. connections：栅栏、玻璃板、铁栏杆连接
2. fireConnections：火的附着方向
3. stairShapes：楼梯拐角
4. doorHalves：门上下两半互补
5. snowyGround：含雪草方块
6. redstoneConnections：红石线（保守版）
7. leafDistances：树叶距离 BFS
8. plantHeads：kelp/缠怨藤/垂泪藤/洞穴藤 头茎区分
9. campfireSignal：营火 signal_fire
10. noteBlockInstrument：音符盒乐器

---

### 2.6 伪代码：几种典型用途

**只转单个方块（不需要 NBT 和邻居）**：
```
state = ConvertToJava(ConvertFromBedrock(bedrockState))
```

**转单个 section，含 NBT，不修复邻接**：
```
chunk = 临时构造只含这一个 section 的 Chunk
converted = ConvertChunk(chunk)
section = converted.Sections[0]
```

**转单个 section，含 NBT，修复邻接（无邻居时按空气）**：
```
converted = ConvertChunk(chunk)
fixed = fixer.Fix(converted, 空 ChunkNeighbours)
section = fixed.Sections[0]
```

**整图导出（不并发版）**：
```
for each chunk in world:
    stage1[chunk.Pos] = ConvertChunk(chunk)

for each chunk in world:
    neighbours = 从 stage1 取 8 个邻居
    fixed = fixer.Fix(stage1[chunk.Pos], neighbours)
    writeJavaChunk(fixed)
```

**整图导出（fixer 可选，不想等邻居时）**：
```
for each chunk in world:
    converted = ConvertChunk(chunk)
    writeJavaChunk(converted)   // 栅栏等邻接状态不精确，但物品/颜色正确
```

---

## 三、还没实现的，以及为什么

### 3.1 物品：非方块普通物品

**现状**：工具、武器、盔甲、食物等在箱子中整条跳过。

**卡在哪**：需要 Bedrock→Java 物品名映射表。这张表的来源是 GeyserMC/mappings，
mcconv 在 `tools/gendata/main.go` 从那里生成 `Tables.ItemBedrockToJava`。
我们没有自己复现这个生成流程，因此不直接使用该数据。

**mcconv 的做法**：`bedrockToJava` 先查 `ItemBedrockToJava(name, aux)` 得到 Java id，
再处理附魔、耐久等字段。覆盖范围见 `convert/items.go`。

**接入路径**：在 neomega-blocks 或 neomega-converter 里生成一张物品名映射表，
数据来源使用 GeyserMC/mappings，自行复现生成流程（不复制 mcconv 的输出文件）。
生成完成后扩展 `items.ConvertItem` 的第三分支即可。

---

### 3.2 物品：嵌套容器（背包里的潜影盒）

**现状**：带 `tag.Items` 的物品（潜影盒物品形态）整条跳过。

**卡在哪**：递归依赖 `ConvertItem` 自身，需要等 3.1 完成后一起接入。

---

### 3.3 生物群系

**现状**：`ConvertedSection.Biomes` 字段存在但始终为 nil。

**卡在哪**：需要 Bedrock→Java 生物群系 ID 映射。celler/bedrock 可以读 Biomes3D，
但 Java 用字符串 ID，Bedrock 用整数，需要一张对照表。
mcconv 在 `translate/tables.go` 用 GeyserMC/mappings 和 pmmp/BedrockData 生成这张表。

**留位**：`bedrock/biomes/`，等数据来源确定后接入。

---

### 3.4 展示框：方块和里面的物品一起消失，且没有诊断

**现状**：存档里的展示框转换后什么都不剩，`Diagnostics` 里也看不到。

两版的表示不同。
Bedrock 的展示框是方块 `minecraft:frame` / `minecraft:glow_frame`，朝向在方块状态的 `facing_direction` 里，
物品、物品旋转和掉落概率在方块实体 `ItemFrame` / `GlowItemFrame` 里。
Java 的展示框不是方块，是实体 `minecraft:item_frame` / `minecraft:glow_item_frame`，
存放在 `entities/` 目录的 region 里，所在格子是空气。

当前的转换在两层都丢掉了它：

- 方块层：neomega-blocks 的转出表把 `frame` / `glow_frame` 映射成空气。Java 没有对应方块，这个映射本身是对的，所以不计入 `unknown_block`。
- 方块实体层：`ItemFrame` / `GlowItemFrame` 没有注册规则，整条忽略。没有规则的方块实体不产生诊断。

所以展示框本身、框里的物品（包括地图）都会消失。

**为什么不能在方块层修**：Java 没有展示框方块，必须在阶段 1 产出 Java 实体。
现有的方块实体规则只能返回方块状态和方块实体，`ConvertedChunk` 也没有实体字段，需要先追加实体输出。

**接入时要注意**：

- 朝向要从 Bedrock 源状态读。规则拿到的 `Base` 是转换后的 Java 状态，已经是空气，`facing_direction` 不在里面。
- 字段对应（参照 mcconv `internal/convert/entities.go` 的 frames 部分）：
  - `facing_direction` 0..5 与 Java `Facing` 编号相同（下、上、北、南、西、东）。
  - `Pos` 是方块中心沿朝向反方向偏移 0.46875，另写 `block_pos` 为方块坐标。
  - `Item` 用 `items.ConvertItem` 转换，去掉 `Slot`。物品转不出时保留空展示框。
  - `ItemRotation` 是角度（Float），除以 45 取整到 0..7 写成 Byte；`ItemDropChance` 原样保留，缺省为 1。
  - `UUID` 由方块坐标确定性生成，重复导出得到相同的实体。
- Java 实体写进 `entities/` region，块的外层是 `{DataVersion, Position: [x, z], Entities: [...]}`，与方块所在的 `region/` 分开。

**留位**：实体输出与普通实体（3.5）共用；展示框不依赖 celler 读取 actor 记录，数据全在方块和方块实体里，可以先于普通实体接入。

---

### 3.5 普通实体（生物、盔甲架等）

**现状**：celler/bedrock 只有 `TagEntities` 常量，没有读取接口。

**卡在哪**：两个层面都缺失。一是 celler 没有暴露读 actor/digp 记录的 API；
二是实体字段的语义转换本身复杂（生物、盔甲架的格式差异很大）。

**mcconv 的做法**：`convert/entities.go` 单独处理普通实体，
从 bedrock/world.go 的 actor/digp 记录读取，写入独立的 Java entities region。

**留位**：`bedrock/actors/`，等 celler 暴露读取接口后接入。

---

### 3.6 整图导出命令

**现状**：`cmd/bedrock-to-java/main.go` 只有 package 声明，是留位。

**卡在哪**：不是卡住，只是还没做。需要把上面的转换流程串起来，
加上并行调度、celler 的区块扫描、Java region 写出、level.dat 转换。

---

## 四、旗帜图案

**现状**：旗帜底色已转换（颜色在方块名里），图案 NBT（`patterns` 列表）整条丢弃。

**卡在哪**：需要 Bedrock 图案代码 → Java 图案名的映射表，
来源是 pmmp/BedrockData，mcconv 内嵌在 `translate/tables.go`。
viewer 目前只能画纯色旗帜（TBD.md 有记录）。

---

*这份文档随实现推进更新；每完成一项，把对应的「现状」行改为已实现。*
