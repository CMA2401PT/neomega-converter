# neomega-converter

neomega-viewer 只认一种输入：完整的 Java 方块状态（Java block hash），带 block entity 的格子再附一份 Java 的 SNBT。
方块数据如果来自别的表达，比如 Bedrock（NEMC）存档，就要先转换成这种形式。`neomega-converter` 负责这一步，让
viewer 只面对 Java。

转换会丢信息。丢掉的信息分成两种，处理方式不同：

- **能从 Java 这一侧补回来**：比如栅栏的连接方向、楼梯拐角、门上半格的朝向，看周围的方块就能重算。`fixer` 负责这件事。
- **只存在源数据里**：比如 Bedrock 床的颜色、花盆里种的植物，记在 Bedrock 的 block entity NBT 中，Java 这一侧推不出来。
  只能在转换时直接读源数据。这部分还在开发。

问题的完整分类见 neomega-viewer 的 `TBD.md`。

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
