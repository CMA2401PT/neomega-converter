package fixer

import (
	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
)

func (f *Fixer) redstoneConnections(subchunk []JavaBlockHash, neighbours Neighbours) map[int]JavaBlockHash {
	changes := make(map[int]JavaBlockHash)
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
	for index, hash := range subchunk {
		block, ok := java.HashToBlock(hash)
		if !ok || block.NameWithNameSpace() != "minecraft:redstone_wire" {
			continue
		}
		x, y, z := index>>8, index&15, (index>>4)&15
		var connected [4]bool
		count, single := 0, 0
		for i, direction := range directions {
			otherHash, exists := neighbour(subchunk, neighbours, index, direction.side)
			if !exists {
				continue wireLoop
			}
			other, ok := java.HashToBlock(otherHash)
			if !ok {
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
				verticalHash, exists := redstoneBlockAt(subchunk, neighbours, x+direction.x, y+dy, z+direction.z)
				if !exists {
					continue wireLoop
				}
				vertical, ok := java.HashToBlock(verticalHash)
				if !ok || vertical.NameWithNameSpace() == "minecraft:redstone_wire" {
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
		if newHash := java.BlockToHash(updated); newHash != hash {
			changes[index] = newHash
		}
	}
	return changes
}

// 坐标始终相对当前子区块，先合成水平和垂直偏移，再选择所属子区块。
func redstoneBlockAt(subchunk []JavaBlockHash, neighbours Neighbours, x, y, z int) (JavaBlockHash, bool) {
	position := [3]int{x, y, z}
	outsideSide := -1
	for axis, coordinate := range position {
		if coordinate >= 0 && coordinate < 16 {
			continue
		}
		if outsideSide != -1 {
			// 同时越过两轴需要斜对角子区块，六面 neighbours 无法提供。
			return 0, false
		}
		outsideSide = axis * 2
		if coordinate >= 16 {
			outsideSide++
		}
		position[axis] &= 15
	}
	if outsideSide != -1 {
		subchunk = neighbours[outsideSide]
		if subchunk == nil {
			return 0, false
		}
	}
	return subchunk[(position[0]<<8)|(position[2]<<4)|position[1]], true
}
