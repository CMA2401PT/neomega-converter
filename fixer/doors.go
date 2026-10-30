package fixer

import (
	"strings"

	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
)

func (f *Fixer) doorHalves(subchunk []JavaBlockHash, neighbours Neighbours) map[int]JavaBlockHash {
	// 往返后两半保留的信息互补，统一返回改动才能让两半都读取原始伙伴状态。
	changes := make(map[int]JavaBlockHash)
	for index, hash := range subchunk {
		block, ok := java.HashToBlock(hash)
		if !ok || !strings.HasSuffix(block.ShortName(), "_door") {
			continue
		}
		states := block.States()
		side, partnerHalf := 2, "lower"
		properties := []string{"facing", "open"}
		if states.CheckKey("half", "lower") {
			side, partnerHalf = 3, "upper"
			properties = []string{"hinge"}
		}
		partnerHash, exists := neighbour(subchunk, neighbours, index, side)
		if !exists {
			continue
		}
		partner, ok := java.HashToBlock(partnerHash)
		if !ok || partner.NameWithNameSpace() != block.NameWithNameSpace() {
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
		if newHash := java.BlockToHash(updated); newHash != hash {
			changes[index] = newHash
		}
	}
	return changes
}
