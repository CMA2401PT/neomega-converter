package blockentities

import (
	"celler"
	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
)

type Input struct {
	// 基础状态和源 NBT 一起提供，规则既能改外观，也能用绝对坐标关联实体。
	Base     ir.IRBlock
	Source   nbt.Compound
	LocalPos [3]uint8
	WorldPos celler.BlockPos
}

type Result struct {
	// 状态总是明确给出；只有需要保留附加数据时才产出 Java 方块实体。
	State       ir.IRBlock
	BlockEntity nbt.Compound
}

type ConvertFunc func(Input) (Result, error)

// 规则失败时由调用方保留基础状态、丢弃输出并记录诊断。
