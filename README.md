# neomega-converter

neomega-converter 把 Bedrock 区块转换为 Java 方块状态，供渲染或存档写出使用。
这里的区块是一根横向 16×16 的柱子，由多个 section 组成。
一个 section 是 16×16×16 的方块区域。

转换分两步。
第一步只读一个源区块，得到可缓存的 Java 状态。
第二步读取中心和周围八个区块，补全依赖邻居的状态。
邻居必须保持第一步的结果，这样修复中心时不会受到其它区块修复顺序的影响。

## 能力一览

本模块对外提供几层相互独立的能力，调用方按需要组合。
下面用伪代码列出每层的签名、支持到哪一步、什么时候报错，以及不报错但结果降级的情况。
「降级」指调用成功，但输出比源数据少了信息或只是近似。
所列行为以当前代码为准，修改实现时请同步这一节。

```text
能力                                   输入 → 输出
bedrock.ConvertState(state)            单个 Bedrock 状态 → Java 状态, ok
items.ConvertItem(item)                Bedrock 物品 compound → Java 物品 compound 或 nil
bedrock.ConvertBlockEntity(id, Input)  单格方块实体 → Java 状态 + 可选方块实体 / 实体, ok, err
bedrock.ConvertSection(pos, dim, ...)  单个 Bedrock section + 方块实体 → ConvertedChunk （阶段 1）
bedrock.ConvertChunk(chunk)            Bedrock 整根柱子 → ConvertedChunk, err      （阶段 1）
fixer.New()                            → *Fixer, err
fixer.Fix(center, 8 个邻居)            ConvertedChunk → 修复后的 ConvertedChunk    （阶段 2）
fixer.FixSection(palette, 6 个邻居)    单 section 的 Java 调色板 → 修复后的调色板
adapt.NewConverted(read, opts)        按坐标读取 Bedrock 柱子 → 缓存阶段 1 的来源
adapt.NewFixed(converted, fixer, opts) 阶段 1 来源 → 缓存修复结果的来源
export.ToJavaChunk / ToJavaEntities    ConvertedChunk → Java 区块 / Java 实体记录
export.ConvertLevel(level)             Bedrock level.dat → Java level.dat
export.ConvertWorld(src, dst, opts)    Bedrock 存档 → Java 存档，不跑 fixer          （导出第 1 步）
export.FixWorld(world, fixer, opts)    已写好的 Java 存档 → 修复邻接状态后写回       （导出第 2 步）
```

这些入口都在本模块；`ConvertState` 内部使用 `neomega-blocks/presets/block-converts/v1` 的两个函数。
阶段 1 的三个粒度共用同一套实现：`ConvertSection` 和 `ConvertChunk` 只差在输入是已解码的单个 section 还是整根柱子的原始记录。

### 单个方块状态

存档里的状态可能来自任意 Bedrock 版本，而 Java 转出表只认目标版本的状态。
所以转换分两步：`ConvertFromBedrock` 先把旧方块名、旧属性和缺失的属性对齐到目标版本，`ConvertToJava` 再转成 Java。
`bedrock.ConvertState` 就是这两步，本模块的区块转换、花盆规则和物品转换都经过它。

```text
ConvertState(state.Tidy()) → (java, ok)
  内部：nemc = ConvertFromBedrock(state)；java = ConvertToJava(nemc)
  目标集合：NEMC 调色板（参考 Bedrock 1.21.50，共 15156 个状态），全部能转出
  输出：Java 1.21.4 的完整状态，已经 tidy
  ok=false：方块名不认识；或属性值不在取值域内，例如 weirdo_direction=99
  降级（ok=true）：
    缺少的属性补默认值        oak_stairs []          → 朝东的直楼梯
    179 个方块名映射成空气    教育版元素与化合物、hard_*glass、colored_torch_*、
                              underwater_torch、border_block、invisible_bedrock、micro_block、
                              旧版 PE 方块（glowingobsidian、netherreactor、info_update 等）
    frame / glow_frame 映射成空气，因为 Java 的展示框是实体，不是方块；实体由下文的 item_frame 规则产出
    颜色或内容在 NBT 里的方块只得到默认外观
                              bed → red_bed，standing_banner → white_banner，flower_pot → 空花盆
    输出不带 waterlogged
```

两个函数都不看调色板条目的版本号。
同一段状态文本在不同版本含义不同时，按生成转换表时的优先级解释，详见 neomega-blocks 的 README。

### items.ConvertItem：单个物品

```text
items.ConvertItem(bedrockItem) → javaItem 或 nil
  支持：方块物品，Java 物品 ID 取转出的 Java 方块名；变体按以下顺序确定
          1. 物品带 Block compound（name + states）时用它
          2. 否则按 Name + 顶层 Damage 查 legacy data value
          3. 都失败时只用 Name，取默认状态
        potion / splash_potion / lingering_potion、药水箭（arrow 且 Damage>0）
        written_book（标题、作者、代数、书页）、writable_book（书页原文）
        附魔 tag.ench → minecraft:enchantments；保留 Slot
  返回 nil，调用方跳过整个物品：
        名称为空或 air、Count<=0
        工具、武器、盔甲、食物、普通箭等非方块物品（缺物品名映射表）
        带 tag.Items 的嵌套容器
        药水编号越界或为 decay、附魔编号越界或字段类型错误、书页不是字符串
  降级：
        没有 Block compound 的旧物品，名称已不在目标版本时（如 wool + Damage 14）
          legacy data value 查不到，退回默认状态（white_wool）
        床、旗帜、头颅的 Damage 是颜色或种类，不是方块数据，退回默认外观
        不写耐久：目前支持的物品都没有耐久
        tag 中其它字段（自定义名称、lore、CanPlaceOn 等）丢弃
        § 格式码原样放进文本组件，不解析成样式
```

### 方块实体规则：单格

每个规则子包在 `bedrock/blockentities/` 下，导出 `ID`（Bedrock 实体 id）和 `Convert`。
`bedrock.ConvertBlockEntity` 按 id 从注册表选规则。

```text
ConvertBlockEntity(id, Input{Base, SourceState, Source, LocalPos, WorldPos, Dimension})
    → (Result{State, BlockEntity, Entity}, ok, err)
  Base 是已转换的 Java 状态，规则在它的基础上改名或改属性
  SourceState 是对齐到目标 Bedrock 版本的源状态，供 Java 状态里已经没有的属性使用（展示框朝向）
  ok=false：这个 id 没有规则
  err != nil：调用方应保留 Base，丢弃本条输出
```

| 子包 / ID | 改什么 | 报错 | 降级 |
|---|---|---|---|
| bed / `Bed` | 颜色进方块名 | color 不是 0..15 的 Byte | Java 不需要床的方块实体，不产出 |
| flower_pot / `FlowerPot` | 植物进盆栽名 | PlantBlock 不是 compound，或缺 name/states | 植物转不出或没有对应盆栽时保留空花盆 |
| banner / `Banner` | 底色进方块名 | Base 不是 0..15 的 Int | 图案 Patterns 丢弃，不产出方块实体 |
| shulker_box / `ShulkerBox` | facing | facing 不是 0..5 的 Byte | 箱内物品丢弃，不产出方块实体 |
| jukebox / `Jukebox` | has_record | 不报错 | 唱片物品丢弃，has_record 可能为 true 而没有唱片 |
| lectern / `Lectern` | has_book | hasBook 不是 Byte | 书丢弃，has_book 可能为 true 而没有书 |
| note_block / `Music` | note | note 不是 0..24 的 Byte | instrument 留给 fixer |
| skull / `Skull` | 落地头颅的 rotation | 落地头颅缺 Float Rotation | 玩家头颅的皮肤丢弃，不产出方块实体 |
| chest / `Chest` | 双箱 type=left/right；产出方块实体 | Items 不是 compound 列表；pairx/pairz 不是 Int；配对箱没有水平 facing | 转不出的物品逐条跳过；配对坐标不相邻时按单箱；LootTable 原样复制，不改路径格式 |
| sign / `Sign`、`HangingSign` | 产出方块实体 | FrontText 不是 compound | 颜色取最近的 16 种染料；每面只取前 4 行；格式码不解析 |
| item_frame / `ItemFrame`、`GlowItemFrame` | 方块变成空气，产出 Java 展示框实体 | SourceState 不是 frame / glow_frame，或 facing_direction 不是 0..5 | 物品转不出时保留空框（地图不是方块物品，目前都转不出）；Rotation 写 [0,0]，未在游戏里验证 |

只有 chest 和 sign 产出 Java 方块实体，只有 item_frame 产出 Java 实体，其余规则只改方块状态。

### bedrock.ConvertChunk：整根柱子（阶段 1）

```text
ConvertChunk(*cellerbedrock.Chunk) → (*ConvertedChunk, err)
ConvertSection(pos, dim, *cellerbedrock.Section, []blockEntity) → *ConvertedChunk
  第 0 层：按调色板条目做 ConvertState，失败的条目输出空气
  方块实体：对上表注册的 id 调用规则，产出的方块实体坐标改写为世界坐标，实体进 Entities
  第 1 层：water / flowing_water 让最终状态附带 waterlogged="true"
  err（仅 ConvertChunk，整根柱子失败）：任一 section 解码失败
        不支持的 subchunk 版本（只支持 1、8、9）、运行时调色板、
        调色板条目缺 states 或 Int version（展平前的 name+val 条目属于此类）、
        索引越界、尾部多余字节
  Diagnostics（柱子照常输出）：
    unknown_block  两步转换失败的源状态，输出空气；同一状态合并，Count 为格数，没有坐标
    nbt_error      方块实体记录整体解码失败；整根柱子跳过实体规则，方块照常
    rule_error     实体缺 Int x/y/z、不在本柱子或所在 section 不存在、缺字符串 id（这几种没有坐标）
                   或规则返回 err（有坐标和基础状态）；该实体的效果丢弃，保留基础状态
    unsupported_block_entity  没有规则的方块实体，按 BlockEntityID 合并，Count 为条数；只保留基础状态
      例如熔炉、木桶、漏斗、发射器、投掷器、酿造台的物品，刷怪笼、命令方块、信标、蜂巢的数据
  降级（没有诊断）：
    单个方块状态的所有降级，包括 179 个映射成空气的方块名
    基础状态转不出的格子不执行实体规则（已计入 unknown_block）
    第 1 层只识别水；waterlogged 会附加到任意基础状态上，包括 Java 不能含水的方块和空气
      waterlogged 是本仓库 IR 的扩展属性，不在 Java 属性表里，写出存档时由写出方决定怎么处理
    Biomes 恒为 nil；普通实体（生物、盔甲架等）不读取，Entities 里只有展示框
```

### fixer：邻接修复（阶段 2）

```text
fixer.New() → (*Fixer, err)          err 只来自内嵌实心表加载失败
f.Fix(center, ChunkNeighbours) → *ConvertedChunk
f.FixSection(palette, [6]neighbour) → palette        邻居顺序 west/east/down/up/north/south
  两个入口都不返回 err
  入口先补全状态：查不到 hash 的状态去掉 waterlogged 后经 JavaCanon 补齐默认属性，再加回 waterlogged
        oak_stairs [] 补成朝北的直楼梯；JavaCanon 也不认识的状态原样保留，各 pass 跳过它
  Fix 的中心 section 不能为 nil
  邻居或 section 缺失：取格得到空气并标记没有数据，红石、植物、营火、音符盒据此跳过或取默认
  输入不被修改；返回值与输入共享未改动的 section、BlockEntities 和 Diagnostics
```

| pass | 修什么 | 降级 |
|---|---|---|
| connections | 栅栏、玻璃板、铁栏杆的四向连接 | 实心表缺记录时按不实心 |
| fireConnections | 火的四向与向上附着 | 用实心面近似可燃面 |
| stairShapes | 楼梯拐角 | 没有原版的抵消检查，少数位置拐角与原版不同 |
| doorHalves | 门两半互补 facing/open/hinge | powered 无法恢复 |
| snowyGround | 带 snowy 属性的方块 | 无 |
| redstoneConnections | 平铺红石线的 side/none | 遇到元件、白名单外的邻居、可能的坡道或缺数据时整格不改，保持输入状态（转换给出的是四向 side） |
| leafDistances | 非 persistent 树叶的 distance | 只从中心柱子内的原木出发；够不到的树叶在柱子边缘 6 格内取 6，其余取 7（游戏里会凋落） |
| plantHeads | 海带、缠怨藤、垂泪藤、洞穴藤的头部与茎 | 新头部 age 取 0 |
| campfireSignal | 营火 signal_fire | 无 |
| noteBlockInstrument | 音符盒 instrument | 下方方块按名称归类，未覆盖的材质取 harp |

没有修复的属性（通电状态、红石线的点与十字之分等）见 [fixer/FIXER.md](fixer/FIXER.md)。

### 组合方式

```text
单格，不需要 NBT：
  java, ok = bedrock.ConvertState(state.Tidy())

单格或单 section，需要 NBT 和含水（阶段 1）：
  converted = bedrock.ConvertSection(pos, dim, section, 落在这个 section 里的方块实体)
  palette = converted.Sections[section.Y].Blocks

单 section，没有区块上下文（viewer 上传预览）：
  fixed = f.FixSection(javaPalette, [6]{west, east, down, up, north, south 或 nil})

单根柱子，带邻居：
  stage1[p] = ConvertChunk(read(p))，p 取中心和周围 8 个
  fixed = f.Fix(stage1[中心], 从 stage1 取 8 个邻居)

整图，顺序执行：
  world.ScanChunks(c → stage1[c.Dimension, c.Pos] = ConvertChunk(c))
  for each pos in stage1:
      out = fix ? f.Fix(stage1[pos], 从 stage1 取同维度的 8 个邻居) : stage1[pos]
      写出 out
```

邻居必须取阶段 1 结果，不能取别的柱子修复后的结果。
`ScanChunks` 会给出所有维度的柱子，调用方按 `Dimension` 区分。

整图导出不用自己写这个循环，见下一节的 `export` 包。

### export：导出 Java 存档

存档到存档分成两个互不依赖的步骤。
fixer 每根柱子要读 8 个邻居，而且邻居必须是修复前的原值，和逐根柱子独立进行的转换差别很大，所以单独成一步。
第 2 步只读磁盘上的 Java 存档，也可以对别的来源的 Java 存档运行。

```text
第 1 步  ConvertWorld(bedrockWorld, javaWorld, {Dimensions, Strict}) → (Report, err)
  逐根柱子：ScanChunks → ConvertChunk → ToJavaChunk / ToJavaEntities → 写 region / entities
  不跑 fixer；需要按邻居重算形状的格子登记进 PostProcessing，游戏加载区块时重算
    登记：栅栏、栅栏门、玻璃板、铁栏杆、楼梯、墙、红石线、绊线、紫颂植物
    其余方块不登记：重算会让缺支撑的方块掉落
  err：Strict 时第一根 ConvertChunk 失败的柱子；写盘失败
  Report：每个维度的柱子数、失败的柱子、各诊断种类合计、未知状态前 10、
          不支持的方块实体按 id 合计、超出 Java 高度被丢弃的 section 数、方块实体数、实体数

第 2 步  FixWorld(javaWorld, fixer, {Dimensions}) → (FixReport, err)
  柱子按 (z, x) 排序成行，内存里只留 z-1、z、z+1 三行从磁盘读入的原值
  逐根 Fix 第 z 行（邻居取窗口里的原值）→ 立刻写回有改动的 section → 窗口前移一行
  内存只和一行的柱子数有关，和维度总柱子数无关
  写回时保留 section 的其他 NBT 和区块的其他字段
```

写出的 Java 区块：

```text
ToJavaChunk(ConvertedChunk, 4189) → java.Chunk
  Status=minecraft:full，yPos 为维度最低 section，isLightOn=0，Heightmaps 为空
  超出目标高度的 section 丢弃（主世界 -4..19，下界和末地 0..15）
  waterlogged 原样写出；Java 不能含水的方块上的 waterlogged，游戏加载时会丢掉
ToJavaEntities(ConvertedChunk, 4189) → {DataVersion, Position, Entities} 或 nil（没有实体）
ConvertLevel(Level, 4189) → (level.dat, LevelReport, err)
  从 Bedrock 取 LevelName、SpawnX/Y/Z、RandomSeed、GameType、Difficulty、Time、commandsAllowed
  取不到或类型不对时用 Java 默认值，并列进 LevelReport.DefaultedFields
  Time 同时写进 Time 和 DayTime；WorldGenSettings 写原版三维度生成器，种子取 RandomSeed
```

没有在游戏里验证的：空 Heightmaps 和 `isLightOn=0` 是否让游戏重算高度图和光照，PostProcessing 是否按预期重算形状，`level.dat` 能否直接打开。

## 阶段 1：转换源区块

源数据中的床颜色、花盆植物和第二层水，需要在源数据仍可读取时合并。
`bedrock.ConvertChunk` 接收 `*celler/bedrock.Chunk`，返回 `*converter.ConvertedChunk`。
后者保存整根柱子的 Java 状态、Java 方块实体和诊断，可以作为阶段 1 缓存。
方块实体是附着在某个方块上的 NBT 数据，例如箱子物品。
NBT 是 Minecraft 使用的带类型的数据格式。

基础映射按调色板条目执行。
调色板让多个格子引用同一个方块状态，避免重复存储。
转换缓存以包含版本号的 Bedrock 条目为键，映射使用条目的源状态。
Java 状态用 `ir.IRBlock` 表示，即方块名与属性的规范文本。

基础映射完成后，实体规则根据源 NBT 修改状态或产出 Java 方块实体。
床的颜色进入方块名，花盆植物进入盆栽名称。
箱子用配对箱的世界坐标确定双箱左右半，并转换箱内物品、名称和战利品表路径。
告示牌和悬挂告示牌保留正反面文字、最近的染料颜色、发光状态和封蜡状态。
旧版告示牌的顶层文字作为正面，缺失的背面使用空白文字。
规则的输入同时包含 section 内坐标和世界绝对坐标。
输出实体的 NBT 坐标与其 `Pos` 一致。
规则结构见[床](bedrock/blockentities/bed/NBT.md)和[花盆](bedrock/blockentities/flower_pot/NBT.md)。

实体规则执行后，同一格第二层的 `water` 或 `flowing_water` 会让最终状态附带 `waterlogged="true"`。
`waterlogged` 表示含水，仓库将它作为可附加到方块状态的扩展属性。
实际出现的层组合共享结果，依赖单格 NBT 的实体规则结果独立处理。

section 解码失败通过 `error` 返回。
其余可恢复的问题随区块放进 `Diagnostics`：

- `unknown_block`：未知方块输出空气，相同源状态按受影响格数合并计数。
- `nbt_error`：实体列表无法解码，区块的方块状态仍可输出。
- `rule_error`：单条实体的字段或规则出错，保留基础状态。
- `unsupported_block_entity`：方块实体没有对应规则，按 id 合并计数，只保留基础状态。

没有生物群系映射的 section，其 `Biomes` 为 nil。

## 阶段 2：修复邻接状态

栅栏连接方向、楼梯拐角和门的两半属性，需要根据转换后的周围方块判断。
`fixer.New` 创建修复器。
它的 `Fix` 接收中心 `ConvertedChunk` 和 `ChunkNeighbours`，返回新的中心区块。
邻居按 `NW/N/NE/W/E/SW/S/SE` 常量提供，也就是西北、北、东北、西、东、西南、南、东南。

3×3 的区块范围提供中心周围完整的柱子，因此取格可以同时跨越区块边界和 section 边界。
中心输入和邻居都保持原值。
有改动的 section 使用新调色板，未改动的直接共享输入。
阶段 1 与修复结果分别缓存，供邻居使用的输入始终取自阶段 1。

修复由十个 pass 组成，清单见上文「能力一览」。
一个 pass 是只处理一类邻接规则的遍历。
每个 pass 先收齐整根中心柱子的改动，再统一应用。
下一个 pass 能看到上一个 pass 的结果。
规则依据和细节见 [fixer/FIXER.md](fixer/FIXER.md)。

缺少区块或 section 时，取格返回空气，并标明没有数据。
红石规则利用这个标记跳过无法排除坡道的位置。
其它规则按空气处理。

单 section 上传预览使用 `FixSection`。
它接收 Java 状态调色板和六面邻居，顺序为 west/east/down/up/north/south。
它与区块入口共用取格函数和全部 pass。
viewer 在自己的边界把 hash 还原成 Java 状态，并按渲染入口的需要转换回 hash。
hash 是由完整方块状态计算的数值标识。

## adapt：按坐标读取与缓存

浏览器和区域导出通常只需要世界中的一部分柱子。
`adapt` 把读取、转换和可选的邻接修复组织成按坐标取柱子的来源。
消费方调用 `Chunk(ctx, pos)`，取得共享只读的 `ConvertedChunk`。
不存在的柱子返回 `nil, nil`。

只需要转换结果时，使用 `Converted`。
它调用读取回调取得 Bedrock 柱子，再执行 `bedrock.ConvertChunk`。
需要邻接状态时，在它上面接一个 `Fixed`。
`Fixed` 从同一个 `Converted` 取得中心及八个邻居，再调用 `fixer.Fix`。
两种来源都实现 `ChunkSource`，消费方可以使用同一套读取代码。

修复可能改变邻居的形状，例如直楼梯变成内拐角后会多出一个完整侧面。
如果再拿这个形状去修复旁边的栅栏，结果就会依赖柱子的访问顺序。
因此 `NewFixed` 的输入类型是 `*Converted`，而不是任意 `ChunkSource`。
这使 `Fixed` 无法被误接到另一个 `Fixed` 上。
调用方仍可同时读取同一个 `Converted` 和它上面的 `Fixed`。
已有其他形式的阶段 1 数据时，可以直接调用 `fixer.Fix`。

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
var source adapt.ChunkSource = adapt.NewFixed(
    converted,
    f,
    adapt.Options{},
)
chunk, err := source.Chunk(ctx, celler.ChunkPos{X: 10, Z: -3})
```

viewer 可以把取得的柱子适配成渲染 section。
区域导出可以遍历所需坐标，把柱子传给 `export.ToJavaChunk` 和 `export.ToJavaEntities`。
两种用途共用上述来源，邻居仍从原始 Bedrock 来源读取。
转换和修复的语义及诊断含义见前面的两个阶段。

## 添加规则

先确定规则需要读取什么数据，再把它放在能够取得这些数据的位置。

| 规则依赖什么 | 例子 | 写在哪 |
|---|---|---|
| 只看方块名和属性 | 某方块映射错、新版本改名 | neomega-blocks 的 block-converts 表 |
| 调色板条目的版本号 | 旧版本状态升级 | `bedrock.ConvertChunk` 的调色板条目转换 |
| 同一格的第二层 | 含水 | `bedrock.ConvertChunk` 的层组合 |
| 同一格的源方块实体 | 床颜色、花盆、旗帜底色、箱子物品、告示牌文字 | `bedrock/blockentities/<子包>`，并在 `bedrock/convert.go` 的表里加一行 |
| 物品 | 附魔、物品变体、嵌套容器 | `bedrock/items`，由方块实体规则调用 |
| 邻居的 Java 状态 | 栅栏、楼梯、snowy、树叶距离 | `fixer/` 加一个 pass |
| 生物群系 | | `bedrock/biomes` |
| 方块实体要变成 Java 实体 | 展示框 | `bedrock/blockentities/<子包>`，在 `Result.Entity` 里返回 |
| 普通实体 | 生物、盔甲架 | `bedrock/actors` |
| 世界元数据、存档写出 | `level.dat`、region、PostProcessing 名单 | `export/` |

方块实体规则通过 `blockentities.Input` 读取基础状态和源 NBT。
规则返回的 `Result` 总是包含状态，附加实体可以为 nil。
错误返回给区块入口，入口丢弃该条输出并记录诊断。
注册表由 `bedrock` 持有，让规则子包可以依赖共享类型而不形成循环导入。

箱子物品由 `items.ConvertItem` 转换。
它支持可查到 Java 方块名的方块物品，以及药水、药水箭和书本。
附魔写入 Java 物品组件；组件是物品上的附加数据。
无法转换的条目整体跳过，所有条目都跳过时仍保留空箱实体。
范围、文字近似转换和映射数据来源见 [items 包说明](bedrock/items/doc.go)。

`bedrock/biomes`、`bedrock/actors` 只预留职责。

## 用法

下面的程序构造一个含蓝床和含水台阶的源区块，然后完成两步转换。
可将它保存为模块内的 `example/main.go`，运行 `go run ./example`。

```go
package main

import (
    "fmt"
    "log"

    "celler"
    cellerbedrock "celler/bedrock"
    "celler/layer_storage"
    "neomega-blocks/ir"
    "neomega-converter/bedrock"
    "neomega-converter/fixer"
    nbt "neomega-nbt/presets/v1"
)

func main() {
    source := cellerbedrock.NewChunk(celler.ChunkPos{}, 0)
    base := cellerbedrock.NewLayer(cellerbedrock.Block{State: "minecraft:air []"})
    water := cellerbedrock.NewLayer(cellerbedrock.Block{State: "minecraft:air []"})
    base.Blocks.Set(
        layer_storage.At(4, 4, 4),
        cellerbedrock.Block{
            State: ir.IRBlock(`minecraft:bed ["direction"=3,"head_piece_bit"=false,"occupied_bit"=false]`).Tidy(),
            Version: 18168865,
        },
    )
    base.Blocks.Set(
        layer_storage.At(2, 4, 4),
        cellerbedrock.Block{
            State: ir.IRBlock(`minecraft:oak_slab ["minecraft:vertical_half"="bottom"]`).Tidy(),
            Version: 18168865,
        },
    )
    water.Blocks.Set(
        layer_storage.At(2, 4, 4),
        cellerbedrock.Block{State: `minecraft:water ["liquid_depth"=0]`, Version: 18168865},
    )
    if err := source.SetSection(&cellerbedrock.Section{
        Y: 0,
        Version: 9,
        Layers: []*cellerbedrock.Layer{base, water},
    }); err != nil {
        log.Fatal(err)
    }
    if err := source.SetBlockEntities([]nbt.Document{{Value: nbt.Compound{
        "id": "Bed", "color": int8(11),
        "x": int32(4), "y": int32(4), "z": int32(4),
    }}}); err != nil {
        log.Fatal(err)
    }
    converted, err := bedrock.ConvertChunk(source)
    if err != nil {
        log.Fatal(err)
    }
    repair, err := fixer.New()
    if err != nil {
        log.Fatal(err)
    }
    fixed := repair.Fix(converted, fixer.ChunkNeighbours{})
    for _, x := range []int{4, 2} {
        fmt.Println(fixed.Sections[0].Blocks.Get(layer_storage.At(x, 4, 4)))
    }
    for _, diagnostic := range converted.Diagnostics {
        fmt.Println(diagnostic.Kind, diagnostic.Count, diagnostic.Err)
    }
}
```

存档回归测试使用 [bedrock/testdata/subchunk.json](bedrock/testdata/subchunk.json)。
它保存子区块 `(0, -4, 0)` 中每个非空气格子的两层源方块及 SNBT，也就是 NBT 的文本表示。
对应世界高度为 `-64..-49`。
测试将这些数据构造成 celler 区块，再调用 `ConvertChunk`。
生成程序按 version 9 的存储格式逐层读取调色板。
在联合工作区中，从本模块目录运行以下命令可重新提取 fixture。
LevelDB 在打开时可能写文件，所以先整体复制数据库：

```sh
fixture_tmp=$(mktemp -d)
cp -R ../resource/基岩版/db "$fixture_tmp/brdb"
go run ./bedrock/testdata/generate.go -db "$fixture_tmp/brdb"
go test ./bedrock -run TestSavedSubchunk -v
```

## 命令行导出

`cmd/bedrock-to-java` 把上面的两个导出步骤接成命令。
LevelDB 打开时可能写文件，所以要对 Bedrock 存档的副本运行。

```sh
go build -o bedrock-to-java ./cmd/bedrock-to-java

# 第 1 步；加 -fix 时接着跑第 2 步
./bedrock-to-java -input <Bedrock 世界副本> -output <新建或空的目录> [-dims 0,1,2] [-fix] [-strict]

# 只跑第 2 步，对已有的 Java 世界
./bedrock-to-java -fix-only -output <Java 世界> [-dims 0,1,2]
```

`-dims` 是 Bedrock 维度编号，默认只导出主世界。
命令结束时打印两步的报告，一行一个数字。

对一份 572 MB 的主世界存档（195364 根柱子），在开发机上第 1 步用 3 分 39 秒、峰值内存约 50 MB，
第 2 步用 3 分 40 秒、峰值内存约 104 MB，改动了 83558 个 section。

## 实心表从哪里来

fixer 判断「邻居那一面是不是实心墙」时查实心表：每个 Java 状态六个 bit，对应六个方向的面是否被不透明矩形完整覆盖。
它由 neomega-blocks 的几何生成器从同一套模型与材质计算，写入 `presets/java/v1/solidtable/geometry.nvsolid`。
这个 preset 与完整几何表分包，fixer 只加载实心数据。更新时先手动准备生成器的六份 assets 输入，再在 neomega-blocks 根目录运行：

```sh
go generate ./presets/java/v1/geometry
```

重新生成后重新构建本仓库。输入和验证方式见[几何生成说明](../neomega-blocks/third_party/gen_resource/geometry/solid_block_table/README.md)。
