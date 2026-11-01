# 潜影盒朝向

Bedrock 潜影盒的朝向存在方块实体 NBT 中，因此 Java 的基础转换状态需要补入这一信息。
转换根据源朝向覆盖 Java 的 `facing` 属性，并保留方块名和其他属性。

Bedrock 实体 ID 为 `ShulkerBox`。
源字段 `facing` 是 Byte（Go 的 `int8`），表示开口朝向。

| facing | Java 朝向 |
|---|---|
| 0 | down |
| 1 | up |
| 2 | north |
| 3 | south |
| 4 | west |
| 5 | east |

`facing` 缺失、类型不符或越界时返回 error。
