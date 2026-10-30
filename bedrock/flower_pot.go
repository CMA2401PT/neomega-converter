package bedrock

import (
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	java "neomega-blocks/presets/java/v1"
)

func init() {
	entityRules["FlowerPot"] = flowerPot
}

func flowerPot(block ir.IRBlock, entity map[string]any) ir.IRBlock {
	// 植物的 Bedrock 名称可能与 Java 不同，先转换植物，再查询对应盆栽是否存在。
	plant, exists := entity["PlantBlock"].(map[string]any)
	if !exists {
		return block
	}
	source, err := ir.NewIRBlockFromGo(plant["name"].(string), plant["states"].(map[string]any))
	if err != nil {
		panic(err)
	}
	converted, ok := converts.ConvertToJava(source.Tidy())
	if !ok {
		return block
	}
	name := "minecraft:potted_" + converted.ShortName()
	if _, exists := java.BlockProperties(name); !exists {
		return block
	}
	return ir.IRBlock(name + " []")
}
