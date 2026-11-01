# 讲台状态

Java 讲台用 `has_book` 属性表示是否放有书，Bedrock 把这一标志存在方块实体 NBT 中。
转换在源标志非零时把 `has_book` 设为字符串 `"true"`，为零或缺失时设为 `"false"`，并保留其他属性。

Bedrock 实体 ID 为 `Lectern`。
源字段 `hasBook` 是 Byte（Go 的 `int8`），非零表示有书。
字段存在但类型不符时返回 error。
