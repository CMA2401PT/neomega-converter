# neomega-converter 开发说明

本模块在应用层组合 Bedrock 转换与 Java 邻接修复。converter-core 产出转换结果，viewer-core 的 fixer 提供属性规则，本模块负责将调色板、缓存、存档写回和 CLI 接起来。调用入口与行为约定见 [README.md](README.md)。

工作区命令和依赖声明遵循 [根 AGENTS.md](../../AGENTS.md)。模块名为 `neomega-converter`，导入路径不含 `apps/`。引用根目录库时，本地 `replace` 使用 `../../`。

## 维护职责

| 位置 | 修改什么行为 |
|---|---|
| `fix.go` | 整根转换柱子的修复与结果共享 |
| `fixpalette` | celler 调色板到 fixer 查询、遍历及写回的适配 |
| `adapt` | 按坐标读取中心和八邻居、独立缓存修复结果 |
| `export` | 遍历已有 Java 世界并写回邻接属性 |
| `cmd/bedrock-to-java` | 参数校验、转换与修复流程、最终提交和报告 |
| `cmd/structure-to-world` | 结构文件路径、取消和结果展示；解析与落盘复用 converter-core |

方块状态或方块实体转换规则应放在 converter-core；邻接规则应放在 viewer-core/fixer。本模块不复制这些规则，两个 core 也不能反向依赖本模块或彼此依赖。

## 修复数据为什么要保留原值

修复后的楼梯可能多出一个完整侧面，继续用它修复邻居栅栏会使结果依赖访问顺序。因此所有中心与八邻居都来自修复前的转换结果。`adapt.NewFixed` 接受 `*coreadapt.Converted`，不能改成允许另一个 Fixed 作为输入的任意来源接口。

`FixChunk` 仅复制发生改变的调色板，实体、诊断、生物群系和未修改 section 继续共享输入。不要原地修改共享的转换缓存，也不要为了修复方块顺带重写其他数据。

`export.FixWorld` 将柱子按行排列，只保留相邻三行的修复前数据，逐柱修复并写回改动。
窗口不会重读已经写回的柱子，因此结果不受写回顺序影响。
写回仅替换方块状态，保留区块及 section 的其他 NBT。

`export.FixWorld` 退出时会 Flush，CLI 负责最终 Commit 和 Close。报错之前的写入可能已保留，不能把错误返回当作整图回滚。写回方块时保持其他 NBT 原值，不通过重编码整个 section 意外改变生物群系等字段。

## 缓存和生命周期

转换缓存来自 converter-core，修复缓存复用相同的 `CachedSource` 实现，各层独立计费。修改应用缓存时，应继续遵守同坐标请求合并、每个调用者独立取消、过期任务不进入缓存，以及返回值只读共享的约定。

取消等待不代表底层读取已经结束。读取回调的拥有者需要先停止新的读取并等待在途操作退出，再关闭 World。修改来源或维度时创建新的 Converted 和 Fixed 实例，不让旧缓存混入新世界。

## CLI 边界

LevelDB 打开可能写辅助文件，因此文档、测试及手工验证都使用 Bedrock 存档副本。转换输出只接受新建或空目录；`-fix-only` 修改已有 Java 世界，并与 `-input`、`-fix` 互斥。保留参数按是否出现的互斥判断，即使显式传入 `-fix=false` 也属于混用。

报告保持稳定排序，每个计数一行，以便比较转换与修复的输出。维度编号映射由 converter-core 的 `export.JavaDimension` 提供，不在命令中另建映射。

## 验收

从工作区根目录运行：

```sh
./check.sh neomega-converter-core apps/neomega-converter
```

应用测试覆盖原始邻居、取消、缓存复用、修复写回及 CLI。修改 fixer 规则时按 viewer-core 的说明另做对应验收。维护 [转换指南](CONVERSION-GUIDE.md) 时同步核对 README 的能力约定，避免支持范围出现两种解释。
