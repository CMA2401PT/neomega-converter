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
    ▼  converter.FixChunk（阶段 2，可选）
ConvertedChunk（已修复）
    │
    ▼  消费方自行适配
viewer、Java 存档写出……
```

转换逻辑来自 `neomega-converter-core`，修复规则来自 `neomega-viewer-core/fixer/java-postprocess`，本模块负责组合。

阶段 1 只依赖当前区块，结果可以缓存，供多个邻居修复共享。
阶段 2 需要中心区块和周围 8 个区块的阶段 1 结果。

按坐标浏览或导出局部区域时，可用 `neomega-converter-core/adapt.NewConverted` 缓存阶段 1，再按需要组合 `neomega-converter/adapt.NewFixed`。
两层的组装、缓存与并发约定见 [README 的 adapt 说明](README.md#adapt按坐标读取与缓存)。

**fixer 是可选的**。如果只需要方块内容正确（床颜色、含水、物品），不在意栅栏连接或楼梯拐角，
可以跳过阶段 2，直接消费阶段 1 的结果。

---

## 二、各模块的调用方式和适用范围

### 2.1 方块状态基础转换（不走 ConvertChunk）

以下片段中，`bedrock`、`items` 与 `core` 来自 converter-core，`converter` 来自本模块，`fixer` 来自 viewer-core，`fixpalette` 来自本模块的 `neomega-converter/fixpalette`。

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

包含：方块调色板转换、含水、生物群系（Data3D/2D）、已注册的方块实体规则
（床、花盆、旗帜底色与图案、潜影盒、唱片机、讲台、音符盒、头颅、箱子/木桶/漏斗等容器、
熔炉、酿造台、信标、刷怪笼、营火、命令方块、末地门、饰纹罐、告示牌），
以及 0x32 记录里的普通实体。

不包含：邻接状态（栅栏连接、楼梯拐角等）；digp 实体要用 `ConvertChunkWithActors`
传入，整图导出已经这么做。

---

### 2.3 方块实体规则（单格）

适用：只想对一个格子应用方块实体规则，不走完整区块流程。

`bedrock.ConvertBlockEntity` 根据 ID 选择规则；需要直接调用某条规则时，也可以使用对应子包：

```go
result, err := bed.Convert(blockentities.Input{
    Base:     javaState, // 已转换的 Java 状态
    Source:   nbtCompound,
    LocalPos: [3]uint8{x, y, z},
    WorldPos: celler.BlockPos{X: wx, Y: wy, Z: wz},
})
```

`err!=nil` 表示规则执行失败，调用方保留 Base 状态。
各规则的报错条件和降级见 [转换核心的能力一览](../../neomega-converter-core/README.md#能力一览)。

---

### 2.4 物品转换（单个物品）

适用：把 Bedrock 物品 compound 转成 Java 物品 compound，供箱子等方块实体规则调用，
也可以单独调用。

```go
javaItem := items.ConvertItem(bedrockItemCompound)
// nil 表示无法完整转换，调用方跳过该条目
```

支持：方块物品（Block compound、legacy data value 或名称转出 Java 方块）、
非方块物品（`third_party` 的物品表，来源见 [third_party/README.md](../../neomega-converter-core/third_party/README.md)）、
药水/药水箭、书本、附魔、耐久、自定义名称、lore、皮革颜色、无法破坏、修复成本、嵌套容器。

仍缺：`trim`、属性修饰、`CanPlaceOn`/`CanDestroy`、烟花与地图内容、自定义药水效果；
物品表查不到的名字整条跳过。

---

### 2.5 阶段 2：fixer（单区块，需要邻居）

适用：给定阶段 1 的结果，补全邻接依赖的状态。

**整根区块版**（整图导出、按需模式）：
```go
f, err := fixer.New()
fixed := converter.FixChunk(f, center, [8]*core.ConvertedChunk{
    fixpalette.NW: nwChunk, fixpalette.N: nChunk, /* … */
})
```
邻居为 nil 时，对应方向按空气处理。

**单 section 版**（viewer 的上传预览，没有完整区块上下文）：
```go
fixed := fixpalette.FixSection(f, section, [6]*layer_storage.Palette[ir.IRBlock]{
    // west, east, down, up, north, south
    nil, nil, nil, nil, nil, nil,
})
```

已实现的 pass（按执行顺序；各自的限制见 viewer-core 的 fixer/FIXER.md）：
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
fixed = converter.FixChunk(fixer, converted, 空八邻居数组)
section = fixed.Sections[0]
```

**整图导出**：用 `export` 包，两步互不依赖：
```
report = coreexport.ConvertWorld(bedrockWorld, javaWorld, options)   // 不跑 fixer，写 PostProcessing
fixReport = export.FixWorld(javaWorld, fixer, fixOptions)        // 可选，读回 Java 存档修复后写回
```

---

## 三、实现范围与仍缺的部分

### 3.1 物品

**现状**：已接入物品名表。方块物品仍走方块表，非方块物品查
`neomega-converter-core/third_party` 的表；表由 GeyserMC/mappings 生成，
版本与 sha256 固定在锁文件里。

组件已覆盖：附魔（含附魔书的 `stored_enchantments`）、耐久、自定义名称、lore、
皮革颜色、无法破坏、修复成本、嵌套容器、药水与书本。

**仍缺**：

- `trim`、属性修饰、`CanPlaceOn`/`CanDestroy`、烟花、地图内容；
- 自定义药水效果只保留基础药水种类；
- 物品表查不到的名字整条跳过，不会原样透传到 Java。

### 3.2 方块实体

**现状**：床、花盆、旗帜（底色与图案）、潜影盒、唱片机、讲台、音符盒、头颅、
箱子/木桶/漏斗/发射器/投掷器/末影箱、熔炉三兄弟、酿造台、信标、刷怪笼、营火、
命令方块、末地门、饰纹罐和展示框都已接入。

**仍缺**：蜂巢、拼图方块、结构方块、试炼刷怪笼、宝库；命令方块只复制命令原文，
玩家头颅的皮肤 profile 不保留。

### 3.3 生物群系

**现状**：已实现。优先 Data3D，缺失时用 Data2D；未知编号和缺失 section 回退 plains，
保证写出的 section 都有 `biomes` 字段。映射表同样来自 `third_party`。

### 3.4 展示框

**现状**：已转换。`neomega-converter-core/bedrock/blockentities/item_frame` 把展示框所在格子转成空气，
并在 `ConvertedChunk.Entities` 里产出 Java `minecraft:item_frame` / `minecraft:glow_item_frame` 实体。
还缺的部分见本节末尾。

两版的表示不同，这是这条规则长成这样的原因：
Bedrock 的展示框是方块 `minecraft:frame` / `minecraft:glow_frame`，朝向在方块状态的 `facing_direction` 里，
物品、物品旋转和掉落概率在方块实体 `ItemFrame` / `GlowItemFrame` 里。
Java 的展示框不是方块，是实体，存放在 `entities/` 目录的 region 里，所在格子是空气。

因此：

- neomega-blocks 的转出表把 `frame` / `glow_frame` 映射成空气。Java 没有对应方块，这个映射是对的，不计入 `unknown_block`。
- 规则拿到的 `Base` 已经是空气，朝向要从 `Input.SourceState`（对齐到目标 Bedrock 版本的源状态）读。
- 规则在 `Result.Entity` 里返回实体，区块入口把它追加进 `Entities`。

字段对应（参照 mcconv `internal/convert/entities.go` 的 frames 部分）：

- `facing_direction` 0..5 与 Java `Facing` 编号相同（下、上、北、南、西、东）。
- `Pos` 是方块中心沿朝向反方向偏移 0.46875；方块坐标写 `TileX/TileY/TileZ`（1.21.4 的写法）。
- `Item` 用 `items.ConvertItem` 转换，去掉 `Slot`；`ItemRotation` 是角度，除以 45 取整到 0..7；`ItemDropChance` 缺省为 1。
- `UUID` 由世界坐标和维度确定性生成，重复导出得到相同的实体，不同维度同一坐标也不会重号。

**还缺的部分**：

- 地图物品能保留 id，但地图内容在 `map_*.dat` 里，图案与尺寸不转换。
- `Rotation` 写 `[0, 0]`，没有按朝向写原版的 yaw/pitch。
- `Fixed`、`Invisible` 等字段没有转换，取 Java 默认值。

### 3.5 普通实体

**现状**：已实现。导出时按柱读取 `digp`/`actorprefix`，`ConvertChunk` 也会读 0x32；
转换通用字段（位置、朝向、Motion、UUID、名称、标签、生命、装备、持久化、幼年），
以及掉落物、船/运输船和下落方块的少量特例。

**仍缺**：村民交易、乘骑乘客、宠物主人、盔甲架姿势、生物变种的类型专属字段；
不认识的实体标识记 `unknown_entity` 后跳过。

### 3.6 整图导出

**现状**：已实现，见 converter-core README 的导出说明及本模块 README 的命令行说明。
`coreexport` 是 `neomega-converter-core/export`，`export` 是本模块的 `neomega-converter/export`。
转换和邻接修复是两个独立步骤，修复步骤读写的是已经写好的 Java 存档。

**还缺的部分**：

- 并行。两个步骤都是单线程。
- 未完成柱子默认跳过，`-all-chunks` 才会转换。
- 写出格式没有在游戏里加载验证，待确认的字段见 README。

---

## 四、旗帜图案

**现状**：已实现。底色进方块名，`Patterns` 转成方块实体的 `{pattern, color}`；
未知图案代码跳过，上游表缺的 globe/piglin/flow/guster 由规则补齐。

---

*这份文档随实现推进更新；每完成一项，把对应的「现状」行改为已实现。*
