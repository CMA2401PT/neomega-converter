package fixer

import (
	"neomega-blocks/ir"
)

// 原版 SnowyDirtBlock 只看正上方是否属于 #minecraft:snow 标签。
var snowBlocks = map[string]bool{
	"minecraft:snow":        true,
	"minecraft:snow_block":  true,
	"minecraft:powder_snow": true,
}

func (f *Fixer) snowyGround(view snapshot) map[position]ir.IRBlock {
	// snowy 由上方一格唯一决定，往返丢失后按原版规则重算即可。
	changes := make(map[position]ir.IRBlock)
	for pos, block := range view.blocks {
		states := block.States()
		if _, found := states.Get("snowy"); !found {
			continue
		}
		above, _ := neighbour(view.get, pos, 3)
		snowy := snowBlocks[above.NameWithNameSpace()]
		states = states.Set("snowy", map[bool]string{true: "true", false: "false"}[snowy])
		updated := ir.IRBlock(block.NameWithNameSpace() + " " + string(states)).Tidy()
		if updated != block {
			changes[pos] = updated
		}
	}
	return changes
}
