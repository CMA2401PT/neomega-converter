# neomega-converter

neomega-converter 把 Bedrock 区块转换为 Java 方块状态，供渲染或存档写出使用。
这里的区块是一根横向 16×16 的柱子，由多个 section 组成。
一个 section 是 16×16×16 的方块区域。

转换分两步。
第一步只读一个源区块，得到可缓存的 Java 状态。
第二步读取中心和周围八个区块，补全依赖邻居的状态。
邻居必须保持第一步的结果，这样修复中心时不会受到其它区块修复顺序的影响。

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

修复由六个 pass 组成。
一个 pass 是只处理一类邻接规则的遍历。
每个 pass 先收齐整根中心柱子的改动，再统一应用。
下一个 pass 能看到上一个 pass 的结果。
规则依据和细节见 [fixer/FIXER.md](fixer/FIXER.md)。

缺少区块或 section 时，取格返回空气，并标明没有数据。
红石规则利用这个标记跳过无法排除坡道的位置。
其它规则按空气处理。

单 section 上传预览使用 `FixSection`。
它接收 Java 状态调色板和六面邻居，顺序为 west/east/down/up/north/south。
它与区块入口共用取格函数和六个 pass。
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
| 物品 | 附魔、耐久、嵌套容器 | `bedrock/items`，由方块实体规则调用 |
| 邻居的 Java 状态 | 栅栏、楼梯、snowy、树叶距离 | `fixer/` 加一个 pass |
| 生物群系 | | `bedrock/biomes` |
| 普通实体 | 生物、展示框 | `bedrock/actors` |
| 世界元数据、存档写出 | `level.dat`、region | `cmd/bedrock-to-java` |

方块实体规则通过 `blockentities.Input` 读取基础状态和源 NBT。
规则返回的 `Result` 总是包含状态，附加实体可以为 nil。
错误返回给区块入口，入口丢弃该条输出并记录诊断。
注册表由 `bedrock` 持有，让规则子包可以依赖共享类型而不形成循环导入。

箱子物品由 `items.ConvertItem` 转换。
它支持可查到 Java 方块名的方块物品，以及药水、药水箭和书本。
附魔和耐久写入 Java 物品组件；组件是物品上的附加数据。
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
