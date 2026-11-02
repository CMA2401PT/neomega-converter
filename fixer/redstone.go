package fixer

import (
	"neomega-blocks/ir"
)

func redstoneTarget(block ir.IRBlock) bool {
	// 其他红石元件只能作为邻居，不能套用连线规则。
	return block.NameWithNameSpace() == "minecraft:redstone_wire"
}

func (f *Fixer) redstoneConnections(view snapshot) map[position]ir.IRBlock {
	// 只修能排除元件和坡道的平面连线，缺失数据时保留原有形状。
	changes := make(map[position]ir.IRBlock)
	directions := [...]struct {
		name string
		side int
		x, z int
	}{
		{name: "west", side: 0, x: -1},
		{name: "east", side: 1, x: 1},
		{name: "north", side: 4, z: -1},
		{name: "south", side: 5, z: 1},
	}
wireLoop:
	for pos, block := range view.targetBlocks(redstoneTarget) {
		x, y, z := pos.x, pos.y, pos.z
		var connected [4]bool
		count, single := 0, 0
		for i, direction := range directions {
			other, exists := neighbour(view.get, pos, direction.side)
			if !exists {
				continue wireLoop
			}
			switch other.NameWithNameSpace() {
			case "minecraft:redstone_wire":
				connected[i] = true
				count++
				single = i
				continue
			case "minecraft:air", "minecraft:cave_air", "minecraft:void_air",
				"minecraft:stone", "minecraft:cobblestone", "minecraft:dirt":
				// 只接受明确的普通方块；白名单外整根跳过，避免误判元件或未知方块。
			default:
				continue wireLoop
			}
			// 邻格上下有线就可能形成坡道；缺格时也无法排除坡道。
			for _, dy := range [...]int{-1, 1} {
				vertical, exists := view.get(x+direction.x, y+dy, z+direction.z)
				if !exists {
					continue wireLoop
				}
				if vertical.NameWithNameSpace() == "minecraft:redstone_wire" {
					continue wireLoop
				}
			}
		}
		if count == 0 {
			continue
		}
		if count == 1 {
			connected[single^1] = true
		}
		states := block.States()
		for i, direction := range directions {
			value := "none"
			if connected[i] {
				value = "side"
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
