# fixer：修复 NEMC 丢失的 Java 方块状态

neomega-viewer 只认 Java block hash（完整 Java 状态经 Tidy 后取哈希）。方块若来自
NEMC，要经 `neomega-blocks` 的 NEMC→Java 转换才得到 Java 状态，这一步会把一部分属性
丢成默认值——最典型的是连接方向：一段本该四向相连的栅栏，转换后每个方向都成了
`false`。几何表按 hash 查形状，拿到的就是一段孤立的栅栏柱，渲染出来是错的。

`fixer` 是转换之后的修正步骤，属于转换这一侧；Java 本来的数据不需要它。
`Fix(center *converter.ConvertedChunk, neighbours ChunkNeighbours) *converter.ConvertedChunk`
接收中心柱子与周围八根柱子的阶段 1 结果。
`ChunkNeighbours` 按 NW/N/NE/W/E/SW/S/SE 命名常量提供。
连接属性按邻居关系重算，门的属性从上下伙伴格补全。
处理后返回新的中心区块，有改动的 section 换用新调色板，未改动的共享输入。

单 section 预览使用
`FixSection(section *layer_storage.Palette[ir.IRBlock], neighbours [6]*layer_storage.Palette[ir.IRBlock]) *layer_storage.Palette[ir.IRBlock]`。
六面邻居顺序为 west/east/down/up/north/south。
两个入口共用取格函数和 pass，内部直接处理 `ir.IRBlock`。
viewer 在调用边界处理 hash 与状态的转换。
它只改方块状态，不碰几何，也不依赖几何运行时。
状态属性通过 `ir.IRBlock` 查询与改写，属性表和实心表的 hash 查询使用 `neomega-blocks/presets/java/v1`。

这份文档讲三件 README 的使用说明不讲的事：**哪些方块需要修、依据是什么**，**已经实现
了哪些修复**，以及**哪些还没搞定、难在哪**。想知道怎么调用，看本仓库
[README](../README.md) 的《阶段 2：修复邻接状态》一节。

## 哪些方块需要修，依据是什么

「需要修」不是凭印象定的，而是来自一个往返分析工具：
`neomega-blocks/third_party/gen_resource/java-roundtrip`。它枚举全部 Java 状态，
逐个做 Java→NEMC→Java 的往返，把回来的状态和原状态对比——对不上的，就是转换链在这个
方块上丢了信息。结论写入该目录的 `tmp/REPORT.md`；生成命令见 [生成工具说明](../../neomega-blocks/third_party/gen_resource/README.md)。

报告本身只说明「转换后对不上」，不单独证明 Bedrock 表达不了，也不保证 fixer 一定能
恢复。能不能恢复，取决于丢掉的信息能否从邻居关系推导，或从伙伴格保留的状态中取得。
据此把受影响的方块分成三类：

- **邻居能补、已经修**：连接类（栅栏、玻璃板、铁栏杆、fire）、楼梯拐角、门的两半和含雪方块。
  连接方向、拐角形状和 `snowy` 由周围方块推导。门的 `facing`、`open`、`hinge` 从伙伴格保留的状态补全。
- **邻居推不出、没修**：通电状态（`powered`，涉及门、栅栏门、活板门共 72 种方块）要红石
  模拟才知道。门的两半都丢失了这个属性，伙伴格也无法提供。详见下面「还没搞定的」。
- **往返一致、不用修**：墙（`*_wall`，0/4212 个状态不一致）和铁轨（`*rail`，0/46）在
  报告里完全一致——最初凭印象以为它们要修，验证后推翻了。

## 修复框架：多 pass 串行

错误不止一种。今天发现栏杆连接丢了，明天可能发现别的属性也丢了，所以 fixer 不是一个
大函数，而是一串 pass——每个 pass 负责修一类错误。

一个 pass 的签名是 `func(snapshot) map[position]ir.IRBlock`：它只读中心柱子和
邻居，返回「位置 → 新状态」的改动，自己不落笔。
取格坐标的 x、z 相对中心区块原点，y 是世界绝对高度。
取格返回状态和数据是否存在的标记，缺失时返回空气与 false。
两个入口把所有 pass 串起来，一轮收齐整根中心柱子的改动后统一应用，下一轮才看得到上一轮的结果。这样每个 pass 内部永远在读一个一致的
快照，不会出现「同一轮里 A 格改了、B 格读到的是 A 的新值还是旧值」这种顺序依赖。邻居
层在整个过程里始终是调用者传入的原始快照，不随 pass 变化。

加一类新修复，就写一个 `pass` 函数，在 `New()` 的 `f.passes` 列表里登记。列表顺序就是
执行顺序。

pass 判断「邻居那一面算不算实心」时查的是**实心表**（`neomega-blocks/presets/java/v1/solidtable` 内嵌的
`geometry.nvsolid`，由 neomega-blocks 的几何生成器产出，`New()` 默认加载）。表里每个 hash 存六个 bit，对应 west/east/
down/up/north/south 六个方向该面是否被不透明矩形完整覆盖。栅栏和 fire 要靠它判断「这面
是实心墙，该附着上去」。注意它是**渲染语义**上的实心，不等于红石意义上的导电——这个区别
是红石线只能做保守版的原因之一。

## 已经实现的 pass

十个 pass 都在本目录下，按 `New()` 里的登记顺序执行：

### connections.go — 栅栏、玻璃板、铁栏杆的横向连接

重算四个水平方向，既补上该连的、也清掉错连的。规则沿用 Minecraft 本身的分组：木质
栅栏之间互连，地狱砖栅栏只跟同类连；栅栏门按它的 `facing` 判断是否横跨相连。玻璃板和
铁栏杆能连玻璃板、铁栏杆、玻璃和墙。这两类还会连「邻居朝向当前格的那一面是完整不透明
面」的方块——实心表缺记录时按不实心处理。

### fire.go — 火的四向加向上附着

火贴在实心面上。重算 north/east/south/west 加 up 五个方向，某一面的邻居朝回火的那一面
是实心，就置 `true`。用实心面近似附着关系。

### stairs.go — 楼梯拐角形状

楼梯的 `shape`（straight / inner / outer × left / right）由它前后方向上是否有朝向垂直的
楼梯决定。pass 按 facing 找前后两格，是同 `half` 的楼梯且朝向构成拐角时，算出对应的
inner/outer 和 left/right。坐标约定 X 向东、Z 向南，左右的推导见源码注释。

> 注意：实现只做了「前后有楼梯就拐角」的主干规则，**没有**实现 vanilla 里「如果拐角的
> 另一侧也有楼梯顶住，就退回 straight」的抵消检查（源码里标了 TODO）。

left/right 的约定与原版相同：邻居楼梯的朝向是本块朝向左转 90° 时取 `*_left`。例如朝北的楼梯，前方是朝西的楼梯时
取 `outer_left`，高出的一角落在西北，与两侧楼梯相接。`special_java` 的 L 形台阶按这个规则摆放，`special_nemc`
开启修复后应与它一致。

### doors.go — 门的上下半格互补属性

门的两半在 Java→NEMC→Java 往返后保留了不同的信息。
下半格保留 `facing`（朝向）和 `open`（开关状态），但会丢失 `hinge`（合页方向）。
上半格保留 `hinge`，但会丢失 `facing` 和 `open`。
因此，上半格从正下方的下半格取得 `facing` 和 `open`。
下半格从正上方的上半格取得 `hinge`。

只有同名的门、且 `half` 为相对的上下半格时，才能作为伙伴补全属性。
伙伴跨越 section 边界时，区块入口读取同一柱子的相邻 section，单 section 入口读取 up 或 down 邻居层。
伙伴缺失或不匹配时保留原值。
两半都读取同一轮的原始快照，再由框架统一应用改动。
`powered` 在两半中都已丢失，无法据此恢复，pass 保留其输入值。

真实往返用例位于 `doors_test.go`。
在 neomega-converter 模块目录运行 `go test ./fixer -run TestDoor`，可验证朝向、合页、开关组合与跨子区块补全。

### snowy.go — 草方块、菌丝、灰化土的 `snowy`

`snowy` 只表示正上方是不是雪，往返后丢成了 `false`。原版的规则是：上方一格属于 `#minecraft:snow` 标签
（`snow`、`snow_block`、`powder_snow`）时为 `true`，否则为 `false`。pass 按这条规则重算。
上方格在 section 外时经统一取格函数读取，缺失按空气处理。

### redstone.go — 红石线连接（保守版）

只是保守版，不是精确实现，原因见下一节。它只处理「纯平面上铺开的红石线」：四个方向的
邻居必须是红石线或白名单里的普通方块（air / stone / cobblestone / dirt 等），遇到任何
红石元件、未知方块、或上下可能形成坡道的情况，整根线跳过不改。连到的方向置 `side`，
其余 `none`；只有单边连接时补上对侧（Minecraft 里单连会显示成一条直线而非点）。

### leaves.go — 树叶的 `distance`

从中心柱子里的原木（`*_log`、`*_wood`、`*_stem`、`*_hyphae`，不含 `mushroom_stem`）做多源 BFS，
只沿树叶传播，最远到 6。只改 `persistent=false` 的树叶。
BFS 到不了的树叶取 7，游戏里会凋落。
但离柱子边缘 6 格以内的树叶可能由邻居柱子里的原木维持，这些取 6，留给游戏加载后重算。

### plants.go — 植物的头部与茎

`kelp`、`twisting_vines` 看上方，`weeping_vines`、`cave_vines` 看下方。
生长方向上还是同种植物时用 `*_plant`（茎），否则用头部。
新头部的 `age` 取 0，洞穴藤保留 `berries`。

### campfire.go — 营火的 `signal_fire`

正下方是干草块时为 `true`，否则为 `false`。

### note_block.go — 音符盒的 `instrument`

上方是头颅时取对应乐器，否则按下方方块的名称归类。
名称归类是近似规则，未覆盖的材质取 `harp`。

## 还没搞定的，以及难在哪

这些是往返报告里也丢了、但 fixer 现在不碰或碰不好的。记在这里是为了让后来者不用重新
踩一遍，也知道边界画在哪。

### powered：门、栅栏门、活板门的通电状态

报告里 `powered` 影响 72 种方块、1943 个状态，是丢得最多的单个属性。它表示方块有没有
被红石供能，而这**无法从邻居几何推出来**——要知道通没通电，得真的做一遍红石信号传播
模拟。几何查询给不了这个信息，所以没做。

### 红石线：为什么只能做保守版

精确还原红石线的连接，要的东西远超「邻居是什么方块」：

- 需要红石专用的「导电性 / 支撑面 / 信号源」判定。这跟渲染用的实心表**不是一回事**——
  一个方块渲染上实心，不代表它红石意义上导电，反之亦然。要做精确版得单独准备这份数据。
- 红石线会沿方块侧面向上下爬（坡道），判断坡道要看斜对角和上下格。区块入口可读取邻居柱子的相邻 section；
  单 section 入口的六面邻居仍无法提供同时跨两轴的格子。数据缺失时整根线跳过不改。
- 最关键的：一根**四面都不连**的红石线，在 Java 里到底显示成一个点还是一个十字，这个
  信息在转换时已经彻底丢了，没有任何邻居关系能把它还原回来。

所以现在只做了能确定的那部分（纯平面铺线、白名单邻居、遇元件或坡道就跳过），覆盖率不
保证。想做精确版，先得补上红石导电数据这块地基。

### 床：颜色不在方块状态里，不归 fixer

报告里 15 种颜色的床有 240 个状态「方块名变化」，往返回来都成了 `red_bed`。
Bedrock 把床的颜色存在方块实体 NBT 里，方块状态本身没有颜色。
fixer 只改状态属性，也读不到 NBT，所以这类不归它。
从存档转换时，阶段 1 的 `bed` 规则按 NBT 恢复颜色；只有状态、没有 NBT 的输入仍得到 `red_bed`。
