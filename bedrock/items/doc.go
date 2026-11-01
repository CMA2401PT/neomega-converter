// Package items 为方块实体转换可完整表达的物品堆叠。
// ConvertItem 先用 ConvertToJava 查找方块物品的 Java 方块名，作为物品 ID。
// 查表失败时，药水、药水箭和书本通过专门分支转换。
// 调用方把返回 nil 的条目从输出列表中省略，避免留下缺少内容的物品。
//
// 药水种类和附魔名称使用内嵌顺序表，把 Bedrock 编号对应到 Java 名称。
// 药水的顶层 Damage 是种类编号，不是耐久；药水箭使用编号减一查表。
// 越界编号和 Java 无对应效果的 decay 会使整个物品被跳过。
// 其他物品的耐久优先取 tag.Damage，再取顶层 Damage。
//
// 成书保留标题、作者、代数和所有书页，可编辑书保留原文书页。
// 成书与告示牌共用简单 JSON 文本组件，各行保留 section 格式码原文。
// 格式码尚未解析成颜色或字体属性，因此文本样式仍是近似转换。
// 书页保留完整换行，告示牌的四行限制由告示牌规则处理。
//
// 工具、武器、盔甲、食物等非方块普通物品需要 Bedrock→Java 物品名映射表。
// 这张表的来源是 GeyserMC/mappings，当前模块没有这张表，因此跳过这些物品。
// 普通箭也属于此范围，药水箭由专门分支处理。
// 带 tag.Items 的嵌套容器递归依赖物品转换，须等完整物品映射接入后再处理，
// 因而整个容器条目被跳过，避免把有内容的容器转换成空容器。
//
// mcconv 的对应实现是 internal/convert/items.go 中的 bedrockToJava，
// 依赖 Tables.ItemBedrockToJava(name, aux)。
// 该表由 GeyserMC mappings 生成，入口见 mcconv 的 tools/gendata/main.go。
// 接入完整映射时，可在 neomega-blocks 或 neomega-converter 内生成物品名表，
// 从 GeyserMC mappings 自行复现生成流程，而不是复制 mcconv 的生成数据。
package items
