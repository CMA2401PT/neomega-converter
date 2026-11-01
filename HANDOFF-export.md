# neomega-converter 下一步：整图导出基础实现

这份文档给下一个接手「整图导出」的会话用。读完它，按「实施步骤」从第 1 步开始。
工作方式沿用 codex-collab skill：出简报给 Sol，验收自己跑。

## 已完成的状态（2026-11-01）

转换能力见 `CONVERSION-GUIDE.md`，这里只列与导出直接相关的部分：

- `bedrock.ConvertChunk`：`*cellerbedrock.Chunk` → `*converter.ConvertedChunk`，含方块实体。
- `fixer.Fix`：区块级邻接修复，`ChunkNeighbours [8]*ConvertedChunk`。
- celler/bedrock：`World.ScanChunks` 顺序扫描快照，`ReadChunk` 随机读取。
- celler/java：`NewChunk`、`SetSection`、`WriteChunk`、`WriteLevel`，region 格式已就绪。
- 留位：`cmd/bedrock-to-java/main.go`（只有 package main）。

还没实现（这一轮留位，不强求）：生物群系、普通实体、展示框、旗帜图案。

## 目标

这一轮先做**不追求速度和并发的基础实现**，以验证转换正确性为主。
先让单格、单子区块、整图都跑通，再考虑并行和性能优化。

fixer 可能很慢（每根柱子要读 8 个邻居），因此包装代码把它作为独立选项：
可以跑「只转换」（快，邻接状态近似），也可以跑「转换 + 修复」（慢，邻接状态精确）。

## 设计决策

### 核心转换 vs 包装代码的边界

neomega-converter 的核心能力（`bedrock.ConvertChunk`、`fixer.Fix`）已经分离。
`cmd/bedrock-to-java` 是包装层，负责：
- 读 Bedrock world，遍历区块
- 决定是否启用 fixer，以及缓存策略
- 构造 Java Chunk，写出 region
- 转换 level.dat

核心能力本身不依赖「是否导出整图」的决策，不改动 converter.go / bedrock/ / fixer/。

### 包装层的调用模式

三种粒度，都通过核心能力组合：

**单格**（调试/验证用）：
```go
state, _ := converts.ConvertToJava(bedrockState)
// 不走 ConvertChunk，直接用 block-converts 的 ConvertToJava
```

**单子区块**（viewer/调试）：
```go
// 构造只含一个 section 的临时 Chunk
chunk := cellerbedrock.NewChunk(pos, dim)
// 把 section 数据填进去（或用 ScanChunks 取到的真实 Chunk 截一个 section）
converted, _ := bedrock.ConvertChunk(chunk)
section := converted.Sections[sectionY]
// 可选：fixed := fixer.Fix(converted, fixer.ChunkNeighbours{})
```

**整图**：
```go
world.ScanChunks(func(c *cellerbedrock.Chunk) error {
    stage1[c.Pos] = bedrock.ConvertChunk(c)
    return nil
})
// fixer 可选，加一轮扫描
for pos, converted := range stage1 {
    neighbours := lookup8(stage1, pos)
    fixed := fixer.Fix(converted, neighbours)
    writeJavaChunk(fixed)
}
```

### fixer 作为独立选项

包装代码暴露一个 bool 或 flag `--fix`：
- false（默认）：只跑 ConvertChunk，直接写出。速度快，邻接状态不精确。
- true：先全量 ConvertChunk 存内存，再全量 Fix 写出。内存多，结果精确。

基础实现不做流式：先全量阶段 1，再全量阶段 2。
并行版本留到验证正确性之后。

## 实施步骤

### 第 1 步：Java Chunk 适配层

把 `ConvertedChunk` 转成 celler 的 `*java.Chunk`，供写出使用。

需要实现：
```go
// 在 cmd/bedrock-to-java/ 或新建 adapt/ 子包里
func ToJavaChunk(c *converter.ConvertedChunk, dataVersion int32) (*java.Chunk, error)
```

逻辑：
- 调 `java.NewChunk(c.Pos, dataVersion)`。
- 对每个 `ConvertedSection`，构造 `java.Section`：
  - `Blocks`：`Palette[ir.IRBlock]` 直接用（java.Section 的 Blocks 就是这个类型）。
  - `Biomes`：nil（留位）。
- 调 `chunk.SetSection(section)`。
- 方块实体：`ConvertedChunk.BlockEntities` 是 `[]java.BlockEntity`，调 `chunk` 对应的写入方法（先读 java/chunk.go 确认接口）。

验收：`./check.sh neomega-converter`（如果放在 converter 里）或独立编译通过。

### 第 2 步：level.dat 转换

把 Bedrock 的 `level.dat`（celler/bedrock 的 `World.ReadLevel` 返回 `Level` struct）转成 Java 的 level.dat NBT。

需要的字段（只转这几个，其余用 Java 默认值）：
- 世界名称：`Level.LevelName` → `Data.LevelName`
- 出生点：`Level.SpawnX/Y/Z` → `Data.SpawnX/Y/Z`
- DataVersion：写死目标版本（Java 1.21.4 = 4189）
- Version：`{Id: 4189, Name: "1.21.4", Series: "main", Snapshot: 0}`

其余字段（游戏规则、难度、时间等）暂时写 Java 默认值，不从 Bedrock 读。
先读 `celler/java/world.go` 的 `WriteLevel` 签名确认输入类型。

### 第 3 步：cmd/bedrock-to-java/main.go

完整命令行入口。flag：
- `-input <path>`：Bedrock 世界目录
- `-output <path>`：Java 世界目录（自动创建）
- `-dim <int>`：维度（默认 0，主世界）
- `-fix`：启用 fixer（默认关）

流程：
```
1. 打开 Bedrock world（ReadOnly）
2. 创建 Java world（OpenDirectory）
3. 读 level.dat → 转换 → 写 Java level.dat
4. ScanChunks → ConvertChunk → 存 stage1 map
5. 如果 --fix：遍历 stage1，Fix，存 stage2 map；否则 stage2 = stage1
6. 遍历 stage2 → ToJavaChunk → WriteChunk
7. 打印统计：区块数、诊断汇总（unknown_block 总数、nbt_error 数）
```

不写进度条，不并行，不做增量导出。

### 验收标准

1. `./check.sh neomega-converter`（如果有新包）全通过。
2. 对 `resource/基岩版` 跑命令，输出目录能被 Java celler 读回（`java.World.ReadChunk` 不报错）。
3. 读回几个区块，section 里的方块状态能用 `java.HashToBlock` 查到。
4. 开 `--fix` 和不开，两种都能跑完不 panic。

## 参考

- `分析mcconv/06-区块与存档存储/README.md`：celler 的 region 格式细节。
- `分析mcconv/08-实现路线/README.md`：mcconv 的整体流程。
- `celler/bedrock/world.go`：`ScanChunks`、`ReadLevel`。
- `celler/java/world.go`：`WriteChunk`、`WriteLevel`、`Flush`。
- `celler/java/chunk.go`：`NewChunk`、`SetSection`、`Section` 类型。

## 给新会话的提醒

- Sol 简报要转述 codex-collab 的代码和文档规范。
- 改动主要在 `cmd/bedrock-to-java/`，如果需要 adapt 层，放在 `adapt/` 里。
  不改动 `bedrock/`、`fixer/`、`converter.go` 的公开接口。
- celler 已在 go.mod 里有 require/replace，不需要再加。
- Java DataVersion 目标版本是 4189（Java 1.21.4），和 neomega-viewer 用的版本保持一致。
- 这份文件是交接用的临时文件，完成验收后删除它。
