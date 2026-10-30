package fixer

import (
	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
)

func (f *Fixer) fireConnections(subchunk []JavaBlockHash, neighbours Neighbours) map[int]JavaBlockHash {
	changes := make(map[int]JavaBlockHash)
	properties, _ := java.BlockProperties("minecraft:fire")
	for index, hash := range subchunk {
		block, ok := java.HashToBlock(hash)
		if !ok || block.NameWithNameSpace() != "minecraft:fire" {
			continue
		}
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
			if other, exists := neighbour(subchunk, neighbours, index, direction.side); exists && f.solid[other]&(1<<(direction.side^1)) != 0 {
				value = "true"
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
