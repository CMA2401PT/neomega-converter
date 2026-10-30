package flower_pot

import (
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	java "neomega-blocks/presets/java/v1"
)

const ID = "FlowerPot"

var pottedNameExceptions = map[string]string{
	// 杜鹃盆栽的 Java 名称多出 bush，不能只加前缀。
	"azalea":           "potted_azalea_bush",
	"flowering_azalea": "potted_flowering_azalea_bush",
}

func Convert(block ir.IRBlock, entity map[string]any) ir.IRBlock {
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
	plantName := converted.ShortName()
	pottedName, exists := pottedNameExceptions[plantName]
	if !exists {
		pottedName = "potted_" + plantName
	}
	name := "minecraft:" + pottedName
	if _, exists := java.BlockProperties(name); !exists {
		return block
	}
	return ir.IRBlock(name + " []")
}
