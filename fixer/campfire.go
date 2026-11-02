package fixer

import "neomega-blocks/ir"

func campfireTarget(block ir.IRBlock) bool {
	// 普通营火和灵魂营火共用干草底座规则。
	name := block.NameWithNameSpace()
	return name == "minecraft:campfire" || name == "minecraft:soul_campfire"
}

func (f *Fixer) campfireSignal(view snapshot) map[position]ir.IRBlock {
	// 底座决定烟柱高度，转换后需从下方重新恢复。
	changes := make(map[position]ir.IRBlock)
	for pos, block := range view.targetBlocks(campfireTarget) {
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
