package fixer

import (
	"strings"

	"neomega-blocks/ir"
)

type stairDirection struct {
	// 方向向量用于判定左右转向，面编号用于读取前后邻居。
	x, z int
	side int
}

var stairDirections = map[string]stairDirection{
	"west":  {x: -1, z: 0, side: 0},
	"east":  {x: 1, z: 0, side: 1},
	"north": {x: 0, z: -1, side: 4},
	"south": {x: 0, z: 1, side: 5},
}

func (f *Fixer) stairShapes(view snapshot) map[position]ir.IRBlock {
	// 楼梯朝向和上下半部保持原值，只根据前后伙伴补全拐角。
	changes := make(map[position]ir.IRBlock)
	for pos, block := range view.blocks {
		if !strings.HasSuffix(block.ShortName(), "_stairs") {
			continue
		}
		states := block.States()
		facing, _ := states.Get("facing")
		facingName, ok := facing.(string)
		direction, known := stairDirections[facingName]
		if !ok || !known {
			continue
		}
		half, _ := states.Get("half")
		shape := stairShape(view.get, pos, direction, half)
		states = states.Set("shape", shape)
		updated := ir.IRBlock(block.NameWithNameSpace() + " " + string(states)).Tidy()
		if updated != block {
			changes[pos] = updated
		}
	}
	return changes
}

func stairShape(get blockGetter, pos position, facing stairDirection, half any) string {
	// X 向东、Z 向南；面对 F=(x,z)，左为 (z,-x)，右为 (-z,x)。
	// 例如 north 左为 west，east 左为 north；与原版一致，邻居朝向为左转方向时取 *_left。
	// TODO: 尚未实现 vanilla 的额外邻居抵消检查，仅计算前后楼梯的主干规则。
	for _, candidate := range [...]struct {
		side        int
		left, right string
	}{
		{side: facing.side, left: "outer_left", right: "outer_right"},
		{side: facing.side ^ 1, left: "inner_left", right: "inner_right"},
	} {
		other, _ := neighbour(get, pos, candidate.side)
		if !strings.HasSuffix(other.ShortName(), "_stairs") {
			continue
		}
		otherHalf, _ := other.States().Get("half")
		if otherHalf != half {
			continue
		}
		otherFacing, _ := other.States().Get("facing")
		facingName, ok := otherFacing.(string)
		direction, known := stairDirections[facingName]
		if !ok || !known {
			continue
		}
		if direction.x == facing.z && direction.z == -facing.x {
			return candidate.left
		}
		if direction.x == -facing.z && direction.z == facing.x {
			return candidate.right
		}
	}
	return "straight"
}
