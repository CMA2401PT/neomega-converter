package flower_pot

import (
	"fmt"

	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	java "neomega-blocks/presets/java/v1"
	"neomega-converter/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

const ID = "FlowerPot"

var pottedNameExceptions = map[string]string{
	// 杜鹃盆栽的 Java 名称多出 bush，不能只加前缀。
	"azalea":           "potted_azalea_bush",
	"flowering_azalea": "potted_flowering_azalea_bush",
}

func Convert(input blockentities.Input) (blockentities.Result, error) {
	// 植物的 Bedrock 名称可能与 Java 不同，先转换植物，再查询对应盆栽是否存在。
	result := blockentities.Result{State: input.Base}
	value, exists := input.Source["PlantBlock"]
	if !exists {
		return result, nil
	}
	plant, ok := value.(nbt.Compound)
	if !ok {
		return blockentities.Result{}, fmt.Errorf("PlantBlock must be a compound")
	}
	name, nameOK := plant["name"].(string)
	states, statesOK := plant["states"].(nbt.Compound)
	if !nameOK || !statesOK {
		return blockentities.Result{}, fmt.Errorf("PlantBlock requires string name and compound states")
	}
	source, err := ir.NewIRBlockFromGo(name, states)
	if err != nil {
		return blockentities.Result{}, err
	}
	converted, ok := converts.ConvertToJava(source.Tidy())
	if !ok {
		return result, nil
	}
	plantName := converted.ShortName()
	pottedName, exists := pottedNameExceptions[plantName]
	if !exists {
		pottedName = "potted_" + plantName
	}
	name = "minecraft:" + pottedName
	if _, exists := java.BlockProperties(name); !exists {
		return result, nil
	}
	return blockentities.Result{State: ir.IRBlock(name + " []")}, nil
}
