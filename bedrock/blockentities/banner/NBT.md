# 旗帜底色

Java 旗帜把底色写在方块名中，Bedrock 把底色存在方块实体 NBT 中。
转换根据源底色替换 Java 方块名中的颜色，并保留落地或挂墙形式及已有属性。

Bedrock 实体 ID 为 `Banner`。
源字段 `Base` 是 Int（Go 的 `int32`），表示 0..15 的染料颜色编号。

| Base | Java 颜色名 |
|---|---|
| 0 | black |
| 1 | red |
| 2 | green |
| 3 | brown |
| 4 | blue |
| 5 | purple |
| 6 | cyan |
| 7 | light_gray |
| 8 | gray |
| 9 | pink |
| 10 | lime |
| 11 | yellow |
| 12 | light_blue |
| 13 | magenta |
| 14 | orange |
| 15 | white |

基础方块名含 `wall` 时生成 `minecraft:<颜色>_wall_banner`，否则生成 `minecraft:<颜色>_banner`。
`Base` 缺失、类型不符或越界时返回 error。
