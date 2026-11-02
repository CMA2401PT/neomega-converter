package fixer

import (
	"strings"

	"neomega-blocks/ir"
)

func doorTarget(block ir.IRBlock) bool {
	// 活板门没有上下伙伴，不能参与门的互补。
	return strings.HasSuffix(block.ShortName(), "_door")
}

func (f *Fixer) doorHalves(view snapshot) map[position]ir.IRBlock {
	// 往返后两半保留的信息互补，统一返回改动才能让两半都读取原始伙伴状态。
	changes := make(map[position]ir.IRBlock)
	for pos, block := range view.targetBlocks() {
		states := block.States()
		side, partnerHalf := 2, "lower"
		properties := []string{"facing", "open"}
		if states.CheckKey("half", "lower") {
			side, partnerHalf = 3, "upper"
			properties = []string{"hinge"}
		}
		partner, _ := neighbour(view.get, pos, side)
		if partner.NameWithNameSpace() != block.NameWithNameSpace() {
			continue
		}
		partnerStates := partner.States()
		if !partnerStates.CheckKey("half", partnerHalf) {
			continue
		}
		for _, property := range properties {
			value, _ := partnerStates.Get(property)
			states = states.Set(property, value)
		}
		updated := ir.IRBlock(block.NameWithNameSpace() + " " + string(states)).Tidy()
		if updated != block {
			changes[pos] = updated
		}
	}
	return changes
}
