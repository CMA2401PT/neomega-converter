package fixer

import (
	"strings"

	"neomega-blocks/ir"
)

var headInstruments = map[string]string{
	"zombie_head":           "zombie",
	"skeleton_skull":        "skeleton",
	"creeper_head":          "creeper",
	"dragon_head":           "dragon",
	"wither_skeleton_skull": "wither_skeleton",
	"piglin_head":           "piglin",
	"player_head":           "custom_head",
}

func noteBlockTarget(block ir.IRBlock) bool {
	// 乐器属性只属于音符盒，底座仅作为邻居读取。
	return block.NameWithNameSpace() == "minecraft:note_block"
}

func (f *Fixer) noteBlockInstrument(view snapshot) map[position]ir.IRBlock {
	// 头颅优先于底座，单格实体转换无法得到这两处邻接信息。
	changes := make(map[position]ir.IRBlock)
	for pos, block := range view.targetBlocks(noteBlockTarget) {
		instrument := ""
		if above, exists := view.get(pos.x, pos.y+1, pos.z); exists {
			instrument = headInstruments[above.ShortName()]
		}
		if instrument == "" {
			instrument = "harp"
			if below, exists := view.get(pos.x, pos.y-1, pos.z); exists {
				instrument = noteInstrument(below.ShortName())
			}
		}
		states := block.States().Set("instrument", instrument)
		updated := ir.IRBlock(block.NameWithNameSpace() + " " + string(states)).Tidy()
		if updated != block {
			changes[pos] = updated
		}
	}
	return changes
}

func noteInstrument(name string) string {
	// 专用底座必须先于材质匹配，否则羊毛、沙砾等会被更宽的规则覆盖。
	switch name {
	case "gold_block":
		return "bell"
	case "clay":
		return "flute"
	case "packed_ice":
		return "chime"
	case "bone_block":
		return "xylophone"
	case "iron_block":
		return "iron_xylophone"
	case "soul_sand":
		return "cow_bell"
	case "pumpkin":
		return "didgeridoo"
	case "emerald_block":
		return "bit"
	case "hay_block":
		return "banjo"
	case "glowstone":
		return "pling"
	}
	switch {
	case strings.Contains(name, "_wool"):
		return "guitar"
	case name == "sand", name == "red_sand", name == "gravel",
		strings.Contains(name, "concrete_powder"), strings.HasPrefix(name, "suspicious_"):
		return "snare"
	case strings.Contains(name, "glass"), name == "sea_lantern", name == "beacon":
		return "hat"
	case isWoodish(name):
		return "bass"
	case isStoneish(name):
		return "basedrum"
	default:
		return "harp"
	}
}

func isWoodish(name string) bool {
	// 门和活板门也有金属版本，须先排除再匹配木质名称。
	if strings.Contains(name, "iron") || strings.Contains(name, "copper") {
		return false
	}
	for _, part := range [...]string{
		"planks", "_log", "_wood", "_stem", "_hyphae", "bookshelf", "crafting_table",
		"chest", "note_block", "jukebox", "_fence", "_sign", "barrel", "loom",
		"cartography_table", "fletching_table", "smithing_table", "lectern", "composter",
		"beehive", "bee_nest", "bamboo", "daylight_detector", "_trapdoor", "_door", "_pressure_plate",
	} {
		if strings.Contains(name, part) {
			return true
		}
	}
	return false
}

func isStoneish(name string) bool {
	for _, part := range [...]string{
		"stone", "cobble", "_ore", "brick", "netherrack", "obsidian", "quartz",
		"sandstone", "concrete", "terracotta", "deepslate", "basalt", "tuff", "calcite",
		"prismarine", "purpur", "granite", "diorite", "andesite", "nylium", "bedrock",
		"magma", "observer", "dispenser", "dropper", "furnace", "smoker", "coral_block",
		"end_stone", "dripstone", "respawn_anchor", "lodestone", "enchanting_table", "ender_chest", "spawner",
	} {
		if strings.Contains(name, part) {
			return true
		}
	}
	return false
}
