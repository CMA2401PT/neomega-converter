# 床的 NBT 结构与转换

Bedrock 用 `minecraft:bed` 表示所有颜色的床，颜色存在 block entity（附着在格子上的 NBT 数据）中。
Java 把颜色写进方块名，例如 `minecraft:blue_bed`，因此转换时需要读出 Bedrock 的 NBT。

## Bedrock 的 NBT 结构

下面的样本来自测试用 Bedrock 存档，取自 [subchunk.json](../../testdata/subchunk.json)，仅省略坐标字段 `x`、`y`、`z`：

```text
{
  "BlockEntityVersion": 0,
  "color": 11b,
  "id": "Bed"
}
```

`id` 是 string（字符串），值为 `Bed` 时由入口选择床的转换规则。
`color` 是 byte（单字节整数，解码后为 Go 的 `int8`），使用以下染料编号：

| `color` | 颜色 | Java 名称中的颜色部分 |
|---|---|---|
| 0 | 白色 | `white` |
| 1 | 橙色 | `orange` |
| 2 | 品红色 | `magenta` |
| 3 | 淡蓝色 | `light_blue` |
| 4 | 黄色 | `yellow` |
| 5 | 黄绿色 | `lime` |
| 6 | 粉红色 | `pink` |
| 7 | 灰色 | `gray` |
| 8 | 淡灰色 | `light_gray` |
| 9 | 青色 | `cyan` |
| 10 | 紫色 | `purple` |
| 11 | 蓝色 | `blue` |
| 12 | 棕色 | `brown` |
| 13 | 绿色 | `green` |
| 14 | 红色 | `red` |
| 15 | 黑色 | `black` |

## 转换规则

入口先把 Bedrock 方块状态转换成 Java 方块状态，查不到对应 Java 方块时，该格输出空气。
基础转换成功后，床规则根据 `color` 把方块名改为 `minecraft:<颜色>_bed`。
例如样本中的 `11b` 得到 `minecraft:blue_bed`。

改名时保留基础转换得到的全部状态，包括朝向 `facing`、床头尾 `part` 和占用状态 `occupied`。
床头和床尾所在的格子各自使用其 NBT 中的颜色。

## 产出的 Java NBT

床的颜色已经写入 Java 方块名，转换不产出 Java NBT。
