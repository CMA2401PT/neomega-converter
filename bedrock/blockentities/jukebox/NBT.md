# 唱片机状态

Java 唱片机用 `has_record` 属性表示是否装有唱片，Bedrock 用方块实体中的物品字段表达这一信息。
转换在 `RecordItem` 存在且为 compound 时把 `has_record` 设为字符串 `"true"`，否则设为 `"false"`。

Bedrock 实体 ID 为 `Jukebox`。
源字段 `RecordItem` 是 compound（Go 的 `nbt.Compound`，按键组织的 NBT 复合结构），其存在表示有唱片。
字段缺失或类型不符时按无唱片处理。
