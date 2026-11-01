# 音符盒音高

Bedrock 音符盒把音高存在方块实体 NBT 中，Java 则使用 `note` 属性。
转换把源音高写成十进制字符串并覆盖 `note`，其余属性保留基础转换的值。

Bedrock 实体 ID 为 `Music`。
源字段 `note` 是 Byte（Go 的 `int8`），取值 0..24，表示音高。
字段缺失、类型不符或超出 0..24 时返回 error。

Java 的 `instrument` 属性由正下方的 Java 方块决定。
实体规则只能访问当前格子的输入，因此乐器修正需要由能读取邻居的 fixer pass（遍历方块的修复步骤）承担。
