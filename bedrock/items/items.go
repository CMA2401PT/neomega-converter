package items

import (
	"strings"

	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	"neomega-converter/bedrock/internal/blockstate"
	"neomega-converter/bedrock/internal/textcomponent"
	nbt "neomega-nbt/presets/v1"
)

var enchantmentIDs = []string{
	"protection", "fire_protection", "feather_falling", "blast_protection", "projectile_protection",
	"thorns", "respiration", "depth_strider", "aqua_affinity", "sharpness", "smite", "bane_of_arthropods",
	"knockback", "fire_aspect", "looting", "efficiency", "silk_touch", "unbreaking", "fortune", "power",
	"punch", "flame", "infinity", "luck_of_the_sea", "lure", "frost_walker", "mending", "binding_curse",
	"vanishing_curse", "impaling", "riptide", "loyalty", "channeling", "multishot", "piercing",
	"quick_charge", "soul_speed", "swift_sneak", "wind_burst", "density", "breach",
}

var potionIDs = []string{
	"water", "mundane", "long_mundane", "thick", "awkward", "night_vision", "long_night_vision",
	"invisibility", "long_invisibility", "leaping", "long_leaping", "strong_leaping", "fire_resistance",
	"long_fire_resistance", "swiftness", "long_swiftness", "strong_swiftness", "slowness", "long_slowness",
	"water_breathing", "long_water_breathing", "healing", "strong_healing", "harming", "strong_harming",
	"poison", "long_poison", "strong_poison", "regeneration", "long_regeneration", "strong_regeneration",
	"strength", "long_strength", "strong_strength", "weakness", "long_weakness", "decay", "turtle_master",
	"long_turtle_master", "strong_turtle_master", "slow_falling", "long_slow_falling", "strong_slowness",
	"wind_charged", "weaving", "oozing", "infested",
}

// ConvertItem 把一个 Bedrock 物品 compound 转成 Java 物品 compound。
// 无法完整转换时返回 nil，调用方跳过整个物品。
func ConvertItem(src nbt.Compound) nbt.Compound {
	name, _ := src["Name"].(string)
	count, _ := src["Count"].(int8)
	if name == "" || name == "minecraft:air" || count <= 0 {
		return nil
	}
	tag, _ := src["tag"].(nbt.Compound)
	if _, nested := tag["Items"]; nested {
		// 容器内容尚不能递归转换，保留空壳会丢失物品。
		return nil
	}
	aux, _ := number(src["Damage"])
	shortName := strings.TrimPrefix(name, "minecraft:")
	var javaID string
	if block, ok := itemBlock(src, name, aux); ok {
		javaID = block.NameWithNameSpace()
	} else {
		switch shortName {
		case "potion", "splash_potion", "lingering_potion", "written_book", "writable_book":
			javaID = "minecraft:" + shortName
		case "arrow":
			if aux <= 0 {
				return nil
			}
			javaID = "minecraft:tipped_arrow"
		default:
			return nil
		}
	}

	comps := nbt.Compound{}
	switch shortName {
	case "potion", "splash_potion", "lingering_potion", "arrow":
		index := aux
		if shortName == "arrow" {
			index--
		}
		if index < 0 || int(index) >= len(potionIDs) || potionIDs[index] == "decay" {
			return nil
		}
		comps["minecraft:potion_contents"] = nbt.Compound{"potion": "minecraft:" + potionIDs[index]}
	}

	if raw, exists := tag["ench"]; exists {
		enchants, ok := nbt.Compounds(raw)
		if !ok {
			return nil
		}
		levels := nbt.Compound{}
		for _, enchant := range enchants {
			id, idOK := enchant["id"].(int16)
			level, levelOK := enchant["lvl"].(int16)
			if !idOK || !levelOK || id < 0 || int(id) >= len(enchantmentIDs) {
				return nil
			}
			levels["minecraft:"+enchantmentIDs[id]] = int32(level)
		}
		if len(levels) > 0 {
			comps["minecraft:enchantments"] = nbt.Compound{"levels": levels}
		}
	}

	if shortName == "written_book" || shortName == "writable_book" {
		content := bookContent(tag, shortName == "written_book")
		if content == nil {
			return nil
		}
		comps["minecraft:"+shortName+"_content"] = content
	}
	out := nbt.Compound{"id": javaID, "count": int32(count)}
	if len(comps) > 0 {
		out["components"] = comps
	}
	if slot, ok := src["Slot"].(int8); ok {
		out["Slot"] = slot
	}
	return out
}

func itemBlock(src nbt.Compound, name string, aux int32) (ir.IRBlock, bool) {
	// Block 保存了明确的变体；缺少它时才解释 aux，最后用名称补齐默认状态。
	block, _ := src["Block"].(nbt.Compound)
	blockName, nameOK := block["name"].(string)
	states, statesOK := block["states"].(nbt.Compound)
	var source ir.IRBlock
	var ok bool
	if nameOK && statesOK {
		var err error
		source, err = ir.NewIRBlockFromGo(blockName, states)
		ok = err == nil
	} else {
		source, ok = converts.BedrockDataValueToBlock(name, uint32(aux))
	}
	if ok {
		if converted, ok := blockstate.ToJava(source); ok {
			return converted, true
		}
	}
	return blockstate.ToJava(ir.IRBlock(name + " []"))
}

func bookContent(tag nbt.Compound, written bool) nbt.Compound {
	pages := []nbt.Compound{}
	if raw, exists := tag["pages"]; exists {
		source, ok := nbt.Compounds(raw)
		if !ok {
			return nil
		}
		for _, page := range source {
			text, ok := page["text"].(string)
			if !ok {
				return nil
			}
			if written {
				text = textcomponent.Section(text)
			}
			pages = append(pages, nbt.Compound{"raw": text})
		}
	}
	content := nbt.Compound{"pages": pages}
	if written {
		title, _ := tag["title"].(string)
		author, _ := tag["author"].(string)
		generation, _ := number(tag["generation"])
		content["title"] = nbt.Compound{"raw": title}
		content["author"] = author
		content["generation"] = generation
		content["resolved"] = int8(1)
	}
	return content
}

func number(value any) (int32, bool) {
	// Bedrock 的辅助值和书本代数使用不同整数宽度。
	switch value := value.(type) {
	case int16:
		return int32(value), true
	case int32:
		return value, true
	default:
		return 0, false
	}
}
