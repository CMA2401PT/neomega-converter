package fixer

import (
	"strings"

	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
)

var horizontal = [...]struct {
	name string
	side int
}{
	{name: "north", side: 4},
	{name: "east", side: 1},
	{name: "south", side: 5},
	{name: "west", side: 0},
}

func (f *Fixer) connections(subchunk []JavaBlockHash, neighbours Neighbours) map[int]JavaBlockHash {
	changes := make(map[int]JavaBlockHash)
	for index, hash := range subchunk {
		block, ok := java.HashToBlock(hash)
		if !ok {
			continue
		}
		name := block.ShortName()
		fence := strings.HasSuffix(name, "_fence")
		if !fence && !isPane(name) {
			continue
		}
		properties, _ := java.BlockProperties(block.NameWithNameSpace())
		states := block.States()
		for _, direction := range horizontal {
			if _, exists := properties[direction.name]; !exists {
				continue
			}
			attached := false
			if otherHash, exists := neighbour(subchunk, neighbours, index, direction.side); exists {
				other, _ := java.HashToBlock(otherHash)
				// 实心位取邻居朝回当前格的面，成对方向的最低位正好相反。
				attached = f.solid[otherHash]&(1<<(direction.side^1)) != 0 || connects(name, fence, other, direction.name)
			}
			value := "false"
			if attached {
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

func isPane(name string) bool {
	return strings.HasSuffix(name, "_glass_pane") || name == "glass_pane" || name == "iron_bars"
}

func connects(name string, fence bool, neighbour ir.IRBlock, side string) bool {
	// 沿用原连接规则，保留木质与地狱砖栅栏的分组和栅栏门的横向连接。
	other := neighbour.ShortName()
	if fence {
		sameKind := strings.HasSuffix(other, "_fence") && (name == "nether_brick_fence") == (other == "nether_brick_fence")
		gate := strings.HasSuffix(other, "_fence_gate")
		facing, _ := neighbour.States().Get("facing")
		acrossGate := ((facing == "north" || facing == "south") && (side == "east" || side == "west")) ||
			((facing == "east" || facing == "west") && (side == "north" || side == "south"))
		return sameKind || (gate && acrossGate)
	}
	return isPane(other) || other == "glass" || other == "tinted_glass" || strings.HasSuffix(other, "_stained_glass") || strings.HasSuffix(other, "_wall")
}
