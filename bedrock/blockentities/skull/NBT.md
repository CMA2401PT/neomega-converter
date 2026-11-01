# 头颅旋转

Bedrock 落地头颅以角度保存旋转，Java 的 `rotation` 属性使用一圈 16 格的编号。
转换按 `int(math.Round(rot / 22.5)) & 15` 把角度四舍五入到最近的格，再以十进制字符串写入 `rotation`。

Bedrock 实体 ID 为 `Skull`。
源字段 `Rotation` 是 Float（Go 的 `float32`），表示 0..360 度的旋转角。
落地头颅的字段缺失或类型不符时返回 error。

基础方块名含 `_wall_` 的头颅使用方块状态中的朝向，因此直接保留基础状态。
墙上头颅无需提供 `Rotation` 字段。
