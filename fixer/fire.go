package fixer

import (
	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
)

func fireTarget(block ir.IRBlock) bool {
	// 灵魂火没有这组附着属性。
	return block.NameWithNameSpace() == "minecraft:fire"
}

func (f *Fixer) fireConnections(view snapshot) map[position]ir.IRBlock {
	// 火的附着方向依赖邻居朝回当前格的面，保留状态中的燃烧年龄。
	changes := make(map[position]ir.IRBlock)
	properties, _ := java.BlockProperties("minecraft:fire")
	for pos, block := range view.targetBlocks() {
		states := block.States()
		for _, direction := range [...]struct {
			name string
			side int
		}{
			{name: "north", side: 4},
			{name: "east", side: 1},
			{name: "south", side: 5},
			{name: "west", side: 0},
			{name: "up", side: 3},
		} {
			if _, exists := properties[direction.name]; !exists {
				continue
			}
			value := "false"
			// 用实心面近似附着；side^1 是邻居朝回火的那一面。
			other, _ := neighbour(view.get, pos, direction.side)
			if f.faces(other, view.solid)&(1<<(direction.side^1)) != 0 {
				value = "true"
			}
			states = states.Set(direction.name, value)
		}
		updated := ir.IRBlock(block.NameWithNameSpace() + " " + string(states)).Tidy()
		if updated != block {
			changes[pos] = updated
		}
	}
	return changes
}
