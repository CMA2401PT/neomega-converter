package fixer

import (
	"strings"

	"neomega-blocks/ir"
)

func plantTarget(block ir.IRBlock) bool {
	// 头部和茎都需参与，方向表同时决定是否属于这四种植物。
	return plantDirection(block) != 0
}

func plantDirection(block ir.IRBlock) int {
	// 名称可在修复中互换，去掉茎后缀后共用生长方向。
	switch strings.TrimSuffix(block.NameWithNameSpace(), "_plant") {
	case "minecraft:kelp", "minecraft:twisting_vines":
		return 1
	case "minecraft:weeping_vines", "minecraft:cave_vines":
		return -1
	default:
		return 0
	}
}

func (f *Fixer) plantHeads(view snapshot) map[position]ir.IRBlock {
	changes := make(map[position]ir.IRBlock)
	for pos, block := range view.targetBlocks() {
		kind := strings.TrimSuffix(block.NameWithNameSpace(), "_plant")
		dy := plantDirection(block)
		other, exists := view.get(pos.x, pos.y+dy, pos.z)
		name := kind
		if exists && strings.TrimSuffix(other.NameWithNameSpace(), "_plant") == kind {
			name += "_plant"
		}
		if name == block.NameWithNameSpace() {
			continue
		}
		// 茎没有 age；新头部补初始年龄，已有头部则保留原年龄。
		states := ir.IRStates("[]")
		if name == kind {
			states = states.Set("age", "0")
		}
		if kind == "minecraft:cave_vines" {
			if berries, found := block.States().Get("berries"); found {
				states = states.Set("berries", berries)
			}
		}
		changes[pos] = ir.IRBlock(name + " " + string(states)).Tidy()
	}
	return changes
}
