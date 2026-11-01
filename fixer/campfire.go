package fixer

import "neomega-blocks/ir"

func (f *Fixer) campfireSignal(view snapshot) map[position]ir.IRBlock {
	// 底座决定烟柱高度，转换后需从下方重新恢复。
	changes := make(map[position]ir.IRBlock)
	for pos, block := range view.blocks {
		switch block.NameWithNameSpace() {
		case "minecraft:campfire", "minecraft:soul_campfire":
		default:
			continue
		}
		below, exists := view.get(pos.x, pos.y-1, pos.z)
		signal := "false"
		if exists && below.NameWithNameSpace() == "minecraft:hay_block" {
			signal = "true"
		}
		states := block.States().Set("signal_fire", signal)
		updated := ir.IRBlock(block.NameWithNameSpace() + " " + string(states)).Tidy()
		if updated != block {
			changes[pos] = updated
		}
	}
	return changes
}
