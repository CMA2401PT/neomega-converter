# neomega-converter

neomega-viewer 只认一种输入：完整的 Java 方块状态（Java block hash），带 block entity 的格子再附一份 Java 的 SNBT。
方块数据如果来自别的表达，比如 Bedrock（NEMC）存档，就要先转换成这种形式。`neomega-converter` 负责这一步，让
viewer 只面对 Java。

转换会丢信息。丢掉的信息分成两种，处理方式不同：

- **能从 Java 这一侧补回来**：比如栅栏的连接方向、楼梯拐角、门上半格的朝向，看周围的方块就能重算。`fixer` 负责这件事。
- **只存在源数据里**：比如 Bedrock 床的颜色、花盆里种的植物，记在 Bedrock 的 block entity NBT 中，Java 这一侧推不出来。
  `bedrock` 在转换时直接读这些数据，并把第二方块层中的水编进 Java 的 `waterlogged` 属性。

问题的完整分类见 neomega-viewer 的 `TBD.md`。

## 从 Bedrock 转换

Bedrock 把一部分外观信息放在方块状态之外。床的颜色和花盆的植物存在 block entity 中，也就是附着在格子上的 NBT 数据。
含水则存在同一格的第二方块层。`bedrock.Convert` 把这些信息合并进 Java 方块状态，返回 viewer 使用的完整状态 hash。
典型顺序是先调用 `Convert`，再把当前子区块和已转换的邻居交给 `fixer.Fix`，补回连接方向、楼梯拐角等邻接状态。

`Convert` 接收 Bedrock 的 `ir.IRBlock`，即方块名和属性的文本表示。基础转换使用 `converts.ConvertToJava`，由它规范化
源属性，因此存档新增的属性也能交给现有转换表处理。转换失败的格子输出 Java 空气。
已经持有 Bedrock 规范化 hash 的调用方可以用 `ConvertHashes`；它先通过 `BedrockHashToBlockStrict` 还原方块，
再调用同一转换流程。表里查不到的 hash 输出 Java 空气。

每层包含 4096 格，索引为 `(x << 8) | (z << 4) | y`。第一层必有，第二层可省略或为 nil。
SNBT 按子区块内的 `[3]uint8{x, y, z}` 坐标提供。床根据 `color` 改为对应颜色的 Java 床，保留朝向、床头尾和占用状态。
花盆先把 `PlantBlock` 转成 Java 植物，再查找对应的 `potted_` 方块；没有对应盆栽时保留空花盆。
第二层是任意水位的 `water` 或 `flowing_water` 时，输出状态附带 `waterlogged="true"`，其余格子沿用基础状态。

返回的 Java SNBT map 是给 viewer 的 NBT 通道。床和花盆的信息已经进入方块名，所以这些转换的 map 为空。
每种 block entity 的规则在自己的文件中注册；增加一种规则只需增加一个文件。

下面的完整程序可保存为 `example/main.go`，在本仓库运行 `go run ./example`：

```go
package main

import (
    "fmt"

    "neomega-blocks/ir"
    java "neomega-blocks/presets/java/v1"
    "neomega-converter/bedrock"
    "neomega-converter/fixer"
)

func main() {
    layers := [][]ir.IRBlock{make([]ir.IRBlock, 4096), make([]ir.IRBlock, 4096)}
    for _, layer := range layers {
        for index := range layer {
            layer[index] = "minecraft:air []"
        }
    }
    bedIndex := (4 << 8) | (4 << 4) | 4
    stairIndex := (2 << 8) | (2 << 4) | 4
    layers[0][bedIndex] = `minecraft:bed ["direction"=3,"head_piece_bit"=false,"occupied_bit"=false]`
    layers[0][stairIndex] = `minecraft:smooth_quartz_stairs ["minecraft:corner"="none","upside_down_bit"=false,"weirdo_direction"=3]`
    layers[1][stairIndex] = `minecraft:water ["liquid_depth"=0]`
    entities := map[[3]uint8]string{
        {4, 4, 4}: `{id:"Bed",color:11b}`,
    }
    blocks, javaNBT := bedrock.Convert(layers, entities)
    repair, err := fixer.New()
    if err != nil {
        panic(err)
    }
    clean := repair.Fix(blocks, fixer.Neighbours{})
    for _, index := range []int{bedIndex, stairIndex} {
        block, _ := java.HashToBlock(clean[index])
        fmt.Println(block)
    }
    fmt.Println("Java NBT 条数:", len(javaNBT))
}
```

存档回归测试使用 [bedrock/testdata/subchunk.json](bedrock/testdata/subchunk.json)。它保留子区块 `(0, -4, 0)`
中每个非空气格子的两层源方块及 SNBT，对应世界高度 `-64..-49`。生成程序按 version 9 的存储格式逐层读取调色板。
在联合工作区中，从本仓库目录运行以下命令可重新提取 fixture。LevelDB 在打开时可能写文件，所以先整体复制数据库：

```sh
fixture_tmp=$(mktemp -d)
cp -R ../resource/基岩版/db "$fixture_tmp/brdb"
go run ./bedrock/testdata/generate.go -db "$fixture_tmp/brdb"
go test ./bedrock -run TestSavedSubchunk -v
```

## 修复转换丢失的状态

`fixer.New()` 加载 `neomega-assets/solidtable` 内嵌的实心表，多个实例共享同一份只读解码结果。`Fix` 返回独立切片，
输入层和邻居层都保持原值。状态的查询与改写通过 `neomega-blocks/presets/java/v1` 完成。

当前层和每个非 nil 邻居层都应包含 4096 个 Java hash，索引为 `(x << 8) | (z << 4) | y`，与 neomega-viewer 相同。
`Neighbours` 按 west/east/down/up/north/south 排列，即 -X/+X/-Y/+Y/-Z/+Z；nil 按该侧为空处理。无法还原成 Java
状态的 hash 保持原值。

```go
repair, err := fixer.New()
if err != nil {
    return err
}
clean := repair.Fix(blocks, fixer.Neighbours{})
// clean 是完整的 Java 状态，可以交给 neomega-viewer 渲染。
```

修复规则分成若干 pass，每个 pass 修一类错误。当前内置栅栏/玻璃板/铁栏杆连接、fire 连接、楼梯拐角、门的上下两半互补、
含雪方块、红石线六个 pass。哪些方块需要修及其依据、每个 pass 的具体规则、以及尚未修复的部分和原因，见
[fixer/FIXER.md](fixer/FIXER.md)。

## 实心表从哪里来

fixer 判断「邻居那一面是不是实心墙」时查实心表：每个 Java 状态六个 bit，对应六个方向的面是否被不透明矩形完整覆盖。
它要从方块几何算出来，所以由 neomega-viewer 的几何生成器产出，写到 `neomega-assets/solidtable/geometry.nvsolid`，
再由 `neomega-assets/solidtable` 内嵌。几何表更新后，按 neomega-viewer README 的《生成和更新几何表》重新生成，
再重新构建本仓库。
