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
ConvertFromBedrock + ConvertToJava     单个 Bedrock 状态 → Java 状态, ok
items.ConvertItem(item)                Bedrock 物品 compound → Java 物品 compound 或 nil
<规则子包>.Convert(Input)              单格 Java 基础状态 + 源 NBT → Java 状态 + 可选 Java 方块实体, err
bedrock.ConvertChunk(chunk)            Bedrock 整根柱子 → ConvertedChunk, err      （阶段 1）
fixer.New()                            → *Fixer, err
fixer.Fix(center, 8 个邻居)            ConvertedChunk → 修复后的 ConvertedChunk    （阶段 2）
fixer.FixSection(palette, 6 个邻居)    单 section 的 Java 调色板 → 修复后的调色板
```

单个方块状态的两个函数来自 `neomega-blocks/presets/block-converts/v1`，其余都在本模块。
单格和单 section 的阶段 1 没有独立入口。
需要 NBT 时，把它们放进只含一个 section 的临时柱子，再调用 `ConvertChunk`，见下文「组合方式」。

### 单个方块状态

存档里的状态可能来自任意 Bedrock 版本，而 Java 转出表只认目标版本的状态。
所以转换分两步：`ConvertFromBedrock` 先把旧方块名、旧属性和缺失的属性对齐到目标版本，`ConvertToJava` 再转成 Java。
本模块内部的 `ConvertChunk`、花盆规则和物品转换都按这个顺序调用。

```text
nemc, ok = ConvertFromBedrock(state.Tidy())
java, ok = ConvertToJava(nemc)
  目标集合：NEMC 调色板（参考 Bedrock 1.21.50，共 15156 个状态），全部能转出
  输出：Java 1.21.4 的完整状态，已经 tidy
  ok=false：方块名不认识；或属性值不在取值域内，例如 weirdo_direction=99
  降级（ok=true）：
    缺少的属性补默认值        oak_stairs []          → 朝东的直楼梯
    179 个方块名映射成空气    教育版元素与化合物、hard_*glass、colored_torch_*、
                              underwater_torch、border_block、invisible_bedrock、micro_block、
                              旧版 PE 方块（glowingobsidian、netherreactor、info_update 等）
    frame / glow_frame 映射成空气，因为 Java 的展示框是实体，不是方块（见下文 ConvertChunk）
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

```text
<子包>.Convert(Input{Base, Source, LocalPos, WorldPos}) → (Result{State, BlockEntity}, err)
  Base 必须是已转换的 Java 状态，规则在它的基础上改名或改属性
  err != nil：调用方应保留 Base，丢弃本条输出
```

按 id 选规则的注册表在 `bedrock` 包内部，没有导出。
单独使用时，调用方自己按 `ID` 选子包。

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

只有 chest 和 sign 产出 Java 方块实体，其余规则只改方块状态。

### bedrock.ConvertChunk：整根柱子（阶段 1）

```text
ConvertChunk(*cellerbedrock.Chunk) → (*ConvertedChunk, err)
  第 0 层：按调色板条目做上面的两步转换，失败的条目输出空气
  方块实体：对上表注册的 id 调用规则，产出的方块实体坐标改写为世界坐标
  第 1 层：water / flowing_water 让最终状态附带 waterlogged="true"
  err（整根柱子失败）：任一 section 解码失败
        不支持的 subchunk 版本（只支持 1、8、9）、运行时调色板、
        调色板条目缺 states 或 Int version（展平前的 name+val 条目属于此类）、
        索引越界、尾部多余字节
  Diagnostics（柱子照常输出）：
    unknown_block  两步转换失败的源状态，输出空气；同一状态合并，Count 为格数，没有坐标
    nbt_error      方块实体记录整体解码失败；整根柱子跳过实体规则，方块照常
    rule_error     实体缺 Int x/y/z、不在本柱子或所在 section 不存在、缺字符串 id（这几种没有坐标）
                   或规则返回 err（有坐标和基础状态）；该实体的效果丢弃，保留基础状态
  降级（没有诊断）：
    单个方块状态的所有降级，包括 179 个映射成空气的方块名
    没有注册规则的方块实体整条忽略，只剩基础状态
      例如熔炉、木桶、漏斗、发射器、投掷器、酿造台的物品，刷怪笼、命令方块、信标、蜂巢的数据
    展示框整体消失：方块是空气，ItemFrame / GlowItemFrame 没有规则，框里的物品（包括地图）一起丢失
      Java 的展示框是实体，修复要先给 ConvertedChunk 加实体输出，见 CONVERSION-GUIDE 3.4
    基础状态转不出的格子不执行实体规则（已计入 unknown_block）
    第 1 层只识别水；waterlogged 会附加到任意基础状态上，包括 Java 不能含水的方块和空气
      waterlogged 是本仓库 IR 的扩展属性，不在 Java 属性表里，写出存档时由写出方决定怎么处理
    Biomes 恒为 nil；普通实体不读取
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
  nemc, ok = ConvertFromBedrock(state.Tidy())
  java, ok = ConvertToJava(nemc)

单格或单 section，需要 NBT 和含水（阶段 1）：
  chunk = cellerbedrock.NewChunk(pos, dim)
  chunk.SetSection(只含这一个 section 的源数据)
  chunk.SetBlockEntities(落在这个 section 里的实体)
  converted, err = bedrock.ConvertChunk(chunk)
  palette = converted.Sections[y].Blocks

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

还没有的能力：生物群系、普通实体、Java 存档写出适配和 `level.dat` 转换。
导出这一侧的计划见 [HANDOFF-export.md](HANDOFF-export.md)。

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

没有生物群系映射的 section，其 `Biomes` 为 nil。

## 阶段 2：修复邻接状态

栅栏连接方向、楼梯拐角和门的两半属性，需要根据转换后的周围方块判断。
`fixer.New` 创建修复器。
它的 `Fix` 接收中心 `ConvertedChunk` 和 `ChunkNeighbours`，返回新的中心区块。
邻居按 `NW/N/NE/W/E/SW/S/SE` 常量提供，也就是西北、北、东北、西、东、西南、南、东南。

3×3 的区块范围提供中心周围完整的柱子，因此取格可以同时跨越区块边界和 section 边界。
中心输入和邻居都保持原值。
有改动的 section 使用新调色板，未改动的直接共享输入。
缓存应保存阶段 1 结果，修复结果单独交给消费者。

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
| 方块实体要变成 Java 实体 | 展示框 | `bedrock/blockentities/<子包>`，需要先给规则和 `ConvertedChunk` 加实体输出 |
| 普通实体 | 生物、盔甲架 | `bedrock/actors` |
| 世界元数据、存档写出 | `level.dat`、region | `cmd/bedrock-to-java` |

方块实体规则通过 `blockentities.Input` 读取基础状态和源 NBT。
规则返回的 `Result` 总是包含状态，附加实体可以为 nil。
错误返回给区块入口，入口丢弃该条输出并记录诊断。
注册表由 `bedrock` 持有，让规则子包可以依赖共享类型而不形成循环导入。

箱子物品由 `items.ConvertItem` 转换。
它支持可查到 Java 方块名的方块物品，以及药水、药水箭和书本。
附魔写入 Java 物品组件；组件是物品上的附加数据。
无法转换的条目整体跳过，所有条目都跳过时仍保留空箱实体。
范围、文字近似转换和映射数据来源见 [items 包说明](bedrock/items/doc.go)。

需要按请求读取并缓存区块时，`adapt.OnDemandConverter` 是预留的适配类型。
`bedrock/biomes`、`bedrock/actors` 和存档命令入口也只预留职责。

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

## 实心表从哪里来

fixer 判断「邻居那一面是不是实心墙」时查实心表：每个 Java 状态六个 bit，对应六个方向的面是否被不透明矩形完整覆盖。
它由 neomega-blocks 的几何生成器从同一套模型与材质计算，写入 `presets/java/v1/solidtable/geometry.nvsolid`。
这个 preset 与完整几何表分包，fixer 只加载实心数据。更新时先手动准备生成器的六份 assets 输入，再在 neomega-blocks 根目录运行：

```sh
go generate ./presets/java/v1/geometry
```

重新生成后重新构建本仓库。输入和验证方式见[几何生成说明](../neomega-blocks/third_party/gen_resource/geometry/solid_block_table/README.md)。
