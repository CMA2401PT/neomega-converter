package fixer

import (
	"strconv"
	"strings"

	"neomega-blocks/ir"
)

func leafTarget(block ir.IRBlock) bool {
	// 原木即使独占一个 section 也必须激活，才能作为跨 section 传播的起点。
	return isLeaf(block) || isLog(block.ShortName())
}

func isLeaf(block ir.IRBlock) bool {
	// 玩家树叶也参与距离传播，persistent 只在写回时决定是否修改。
	return strings.HasSuffix(block.ShortName(), "_leaves")
}

func (f *Fixer) leafDistances(view snapshot) map[position]ir.IRBlock {
	// 只收集中心柱子的原木和树叶，让传播自然止于已有 section。
	leaves := make(map[position]ir.IRBlock)
	distances := make(map[position]int)
	var queue []position
	for pos, block := range view.targetBlocks() {
		name := block.ShortName()
		if isLeaf(block) {
			leaves[pos] = block
		} else if isLog(name) {
			distances[pos] = 0
			queue = append(queue, pos)
		}
	}
	// 多源 BFS 首次到达即为最近距离，玩家树叶也可传递原木距离。
	for head := 0; head < len(queue); head++ {
		pos := queue[head]
		distance := distances[pos]
		if distance == 6 {
			continue
		}
		for side := range 6 {
			delta := sideOffset(side)
			next := position{x: pos.x + delta.x, y: pos.y + delta.y, z: pos.z + delta.z}
			if _, found := leaves[next]; !found {
				continue
			}
			if _, visited := distances[next]; visited {
				continue
			}
			distances[next] = distance + 1
			queue = append(queue, next)
		}
	}
	changes := make(map[position]ir.IRBlock)
	for pos, block := range leaves {
		if !block.States().CheckKey("persistent", "false") {
			continue
		}
		distance, covered := distances[pos]
		if !covered {
			distance = 7
			// 边缘可能有区块外的原木，留到游戏加载后重算以免误掉落。
			if pos.x <= 5 || pos.x >= 10 || pos.z <= 5 || pos.z >= 10 {
				distance = 6
			}
		}
		states := block.States().Set("distance", strconv.Itoa(distance))
		updated := ir.IRBlock(block.NameWithNameSpace() + " " + string(states)).Tidy()
		if updated != block {
			changes[pos] = updated
		}
	}
	return changes
}

func isLog(name string) bool {
	// 蘑菇柄同样带 stem 后缀，但不能维持树叶。
	return name != "mushroom_stem" && (strings.HasSuffix(name, "_log") ||
		strings.HasSuffix(name, "_wood") || strings.HasSuffix(name, "_stem") ||
		strings.HasSuffix(name, "_hyphae"))
}
