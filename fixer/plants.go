package fixer

import (
	"strings"

	"neomega-blocks/ir"
)

func (f *Fixer) plantHeads(view snapshot) map[position]ir.IRBlock {
	changes := make(map[position]ir.IRBlock)
	for pos, block := range view.blocks {
		kind := strings.TrimSuffix(block.NameWithNameSpace(), "_plant")
		dy := 1
		switch kind {
		case "minecraft:kelp", "minecraft:twisting_vines":
		case "minecraft:weeping_vines", "minecraft:cave_vines":
			dy = -1
		default:
			continue
		}
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
