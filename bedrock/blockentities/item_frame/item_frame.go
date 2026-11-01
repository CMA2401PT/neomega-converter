package item_frame

import (
	"fmt"
	"math"

	"neomega-converter/bedrock/blockentities"
	"neomega-converter/bedrock/items"

	nbt "neomega-nbt/presets/v1"
)

const (
	// 两种 Bedrock 实体共用规则，Java 实体种类由源方块决定。
	ID     = "ItemFrame"
	GlowID = "GlowItemFrame"
)

func Convert(input blockentities.Input) (blockentities.Result, error) {
	// Java 展示框独立于方块存在，朝向必须从尚未转成空气的源状态读取。
	var id string
	switch input.SourceState.NameWithNameSpace() {
	case "minecraft:frame":
		id = "minecraft:item_frame"
	case "minecraft:glow_frame":
		id = "minecraft:glow_item_frame"
	default:
		return blockentities.Result{}, fmt.Errorf("item frame requires frame or glow_frame source state")
	}
	facing, _ := input.SourceState.States().Get("facing_direction")
	direction, ok := facing.(int32)
	if !ok || direction < 0 || direction > 5 {
		return blockentities.Result{}, fmt.Errorf("item frame facing_direction must be 0..5")
	}
	offsets := [6][3]float64{
		{0, 0.46875, 0},
		{0, -0.46875, 0},
		{0, 0, 0.46875},
		{0, 0, -0.46875},
		{0.46875, 0, 0},
		{-0.46875, 0, 0},
	}
	pos := input.WorldPos
	offset := offsets[direction]
	// 维度和三个坐标各占一个 Int，避免跨维度重号；异或常量保留主世界的已有 UUID。
	uuid := [4]int32{pos.X, pos.Y, pos.Z, input.Dimension ^ 0x4e454f4d}
	entity := nbt.Compound{
		"id":     id,
		"Facing": int8(direction),
		"Pos":    []float64{float64(pos.X) + 0.5 + offset[0], float64(pos.Y) + 0.5 + offset[1], float64(pos.Z) + 0.5 + offset[2]},
		// Java 1.21.4 的悬挂实体使用三个 Int 保存挂载方块的位置。
		"TileX":  pos.X,
		"TileY":  pos.Y,
		"TileZ":  pos.Z,
		"Motion": []float64{0, 0, 0},
		// 未核定原版 yaw/pitch 时使用约定的默认值，挂载面仍由 Facing 保留。
		"Rotation": []float32{0, 0},
		"UUID":     uuid,
	}
	if item, ok := input.Source["Item"].(nbt.Compound); ok {
		if converted := items.ConvertItem(item); converted != nil {
			delete(converted, "Slot")
			entity["Item"] = converted
			rotation, _ := input.Source["ItemRotation"].(float32)
			entity["ItemRotation"] = int8(int(math.Round(float64(rotation)/45)) & 7)
			drop, ok := input.Source["ItemDropChance"].(float32)
			if !ok {
				drop = 1
			}
			entity["ItemDropChance"] = drop
		}
	}
	return blockentities.Result{State: "minecraft:air []", Entity: entity}, nil
}
