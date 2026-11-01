# 花盆的 NBT 结构与转换

Bedrock 用 `minecraft:flower_pot` 表示花盆，盆中的植物存在 block entity（附着在格子上的 NBT 数据）中。
Java 用不同方块名区分盆栽，例如 `minecraft:potted_dandelion`，因此转换时需要读出 Bedrock 的植物 NBT。

## Bedrock 的 NBT 结构

下面的样本来自测试用 Bedrock 存档，取自 [subchunk.json](../../testdata/subchunk.json)，仅省略坐标字段 `x`、`y`、`z`：

```text
{
  "BlockEntityVersion": 0,
  "PlantBlock": {
    "name": "minecraft:dandelion",
    "states": {},
    "version": 18168865
  },
  "id": "FlowerPot"
}
```

`id` 是 string（字符串），值为 `FlowerPot` 时由入口选择花盆的转换规则。
`PlantBlock` 是 compound（按键组织的 NBT 复合结构），描述盆中的植物；没有植物时没有 `PlantBlock`。

| `PlantBlock` 字段 | NBT 类型 | 含义 |
|---|---|---|
| `name` | string | 植物的 Bedrock 方块名，带命名空间 |
| `states` | compound | 植物的 Bedrock 方块状态，键和值随植物而变；样本中为空 |
| `version` | int（32 位整数） | 植物方块数据的版本标记 |

转换使用 `name` 和 `states` 构造植物方块，再交给基础转换表得到 Java 植物名。

## 转换规则

规则接收基础转换得到的 Java 空花盆。
没有 `PlantBlock` 时保留空花盆；有植物时，先把植物从 Bedrock 转为 Java，再根据 Java 植物名查找对应盆栽。
例如 Bedrock 的 `minecraft:deadbush` 转为 Java 的 `minecraft:dead_bush`，盆栽名为 `minecraft:potted_dead_bush`。

一般的盆栽名为 `minecraft:potted_<Java 植物名>`，杜鹃使用以下例外：

| Bedrock 植物名 | Java 盆栽名 |
|---|---|
| `minecraft:azalea` | `minecraft:potted_azalea_bush` |
| `minecraft:flowering_azalea` | `minecraft:potted_flowering_azalea_bush` |

植物无法转为 Java 方块，或合成的盆栽名在 Java 方块表中不存在时，保留输入的空花盆。
成功转换的盆栽状态为空，即 `<盆栽名> []`；植物的状态用于识别植物，最终信息由盆栽名表达。

## 产出的 Java NBT

花盆中的植物已经写入 Java 方块名，转换不产出 Java NBT。
