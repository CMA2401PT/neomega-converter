package bedrock

import (
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	"neomega-converter/bedrock/nbt_converts/bed"
	"neomega-converter/bedrock/nbt_converts/flower_pot"
	nbt "neomega-nbt/presets/v1"
)

// 新增 NBT 规则时，新建一个子目录，并在表中加一行。
var entityRules = map[string]func(ir.IRBlock, map[string]any) ir.IRBlock{
	bed.ID:        bed.Convert,
	flower_pot.ID: flower_pot.Convert,
}

func Convert(layers [][]ir.IRBlock, entities map[[3]uint8]string) (blocks []uint32, javaNBT map[[3]uint8]string) {
	// 保留源状态交给 ConvertToJava 规范化，才能处理新增属性的 Bedrock 存档。
	// 每层 4096 格，索引为 (x<<8)|(z<<4)|y；第一层必有，第二层可省略或为 nil。方块须已 Tidy。
	blocks = make([]uint32, len(layers[0]))
	javaNBT = make(map[[3]uint8]string)
	air := ir.IRBlock("minecraft:air []").Hash()
	for index, source := range layers[0] {
		block, ok := converts.ConvertToJava(source)
		if !ok {
			blocks[index] = air
			continue
		}
		position := [3]uint8{uint8(index >> 8), uint8(index & 15), uint8((index >> 4) & 15)}
		if text, exists := entities[position]; exists {
			var entity map[string]any
			if err := nbt.DecodeFromSNBT(text, &entity, nbt.OptDecodeFromSNBT{}); err != nil {
				panic(err)
			}
			if rule := entityRules[entity["id"].(string)]; rule != nil {
				block = rule(block, entity)
			}
		}
		if len(layers) > 1 && layers[1] != nil {
			name := layers[1][index].NameWithNameSpace()
			if name == "minecraft:water" || name == "minecraft:flowing_water" {
				states := block.States().Set("waterlogged", "true")
				block = ir.IRBlock(block.NameWithNameSpace() + " " + string(states))
			}
		}
		blocks[index] = block.Tidy().Hash()
	}
	return blocks, javaNBT
}

func ConvertHashes(layers [][]uint32, entities map[[3]uint8]string) (blocks []uint32, javaNBT map[[3]uint8]string) {
	// hash 只携带表内可还原的状态；未知值交给同一转换流程输出空气。
	decoded := make([][]ir.IRBlock, len(layers))
	for layer, hashes := range layers {
		if hashes == nil {
			continue
		}
		decoded[layer] = make([]ir.IRBlock, len(hashes))
		for index, hash := range hashes {
			decoded[layer][index], _ = converts.BedrockHashToBlockStrict(hash)
		}
	}
	return Convert(decoded, entities)
}
