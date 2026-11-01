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
	// 版本对齐后的源状态保留 Java 基础状态无法表达的属性，例如展示框朝向。
	SourceState ir.IRBlock
	// 相同坐标在不同维度属于不同实体，生成身份时需要区分。
	Dimension int32
}

type Result struct {
	// 状态总是明确给出；附加数据可留在方块实体中，也可转成独立实体。
	State       ir.IRBlock
	BlockEntity nbt.Compound
	Entity      nbt.Compound
}

type ConvertFunc func(Input) (Result, error)

// 规则失败时由调用方保留基础状态、丢弃输出并记录诊断。
