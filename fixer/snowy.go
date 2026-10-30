package fixer

import (
	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
)

// 原版 SnowyDirtBlock 只看正上方是否属于 #minecraft:snow 标签。
var snowBlocks = map[string]bool{
	"minecraft:snow":        true,
	"minecraft:snow_block":  true,
	"minecraft:powder_snow": true,
}

func (f *Fixer) snowyGround(subchunk []JavaBlockHash, neighbours Neighbours) map[int]JavaBlockHash {
	// snowy 由上方一格唯一决定，往返丢失后按原版规则重算即可。
	changes := make(map[int]JavaBlockHash)
	for index, hash := range subchunk {
		block, ok := java.HashToBlock(hash)
		if !ok {
			continue
		}
		states := block.States()
		if _, found := states.Get("snowy"); !found {
			continue
		}
		snowy := false
		if aboveHash, exists := neighbour(subchunk, neighbours, index, 3); exists {
			above, ok := java.HashToBlock(aboveHash)
			snowy = ok && snowBlocks[above.NameWithNameSpace()]
		}
		states = states.Set("snowy", map[bool]string{true: "true", false: "false"}[snowy])
		updated := ir.IRBlock(block.NameWithNameSpace() + " " + string(states)).Tidy()
		if newHash := java.BlockToHash(updated); newHash != hash {
			changes[index] = newHash
		}
	}
	return changes
}
