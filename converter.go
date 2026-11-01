package converter

import (
	"celler"
	"celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"

	nbt "neomega-nbt/presets/v1"
)

type ConvertedChunk struct {
	// 整根区块保留转换后的状态，供缓存和邻居修复共享同一份输入。
	Pos           celler.ChunkPos
	Dimension     int32
	Sections      map[int32]*ConvertedSection
	BlockEntities []java.BlockEntity
	Diagnostics   []Diagnostic
	Entities      []nbt.Compound
}

type ConvertedSection struct {
	// 方块按调色板共享状态；生物群系尚无映射时用 nil 表达缺失。
	Blocks *layer_storage.Palette[ir.IRBlock]
	Biomes *[64]string
}

type DiagnosticKind string

// 诊断种类让调用方分别汇总映射缺失、NBT 解码和实体规则问题。

const (
	UnknownBlock           DiagnosticKind = "unknown_block"
	NBTError               DiagnosticKind = "nbt_error"
	RuleError              DiagnosticKind = "rule_error"
	UnsupportedBlockEntity DiagnosticKind = "unsupported_block_entity"
)

type Diagnostic struct {
	// 可恢复的问题随区块一起返回；合并计数的未知状态可以没有单格坐标。
	Kind          DiagnosticKind
	Pos           *celler.BlockPos
	State         ir.IRBlock
	Count         uint64
	Err           error
	BlockEntityID string
}
