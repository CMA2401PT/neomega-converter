package export

import (
	"sort"
	"strings"

	"celler/java"
	"celler/layer_storage"
	"neomega-converter"
	nbt "neomega-nbt/presets/v1"
)

const DataVersion int32 = 4189

// 目标状态表对应 Java 1.21.4，区块、实体和世界元数据必须使用同一版本。

func ToJavaChunk(chunk *converter.ConvertedChunk, dataVersion int32) (*java.Chunk, error) {
	// 写出层只补加载所需的容器字段，不改变转换产生的方块状态。
	result := java.NewChunk(chunk.Pos, dataVersion)
	low, high := sectionBounds(chunk.Dimension)
	ys := make([]int32, 0, len(chunk.Sections))
	for y := range chunk.Sections {
		if y >= low && y <= high {
			ys = append(ys, y)
		}
	}
	sort.Slice(ys, func(i, j int) bool { return ys[i] < ys[j] })
	// Java 按维度最低 section 解释列表下标，缺失的 section 也必须保留空列表占位。
	post := make([][]int16, high-low+1)
	for i := range post {
		post[i] = []int16{}
	}
	for _, y := range ys {
		source := chunk.Sections[y]
		if err := result.SetSection(&java.Section{
			Y:      y,
			Blocks: source.Blocks,
			Biomes: source.Biomes,
		}); err != nil {
			return nil, err
		}
		if source.Blocks == nil {
			continue
		}
		palette := source.Blocks.GetUsedPalette()
		needs := make([]bool, len(palette))
		for i, block := range palette {
			needs[i] = needsShapeUpdate(block.NameWithNameSpace())
		}
		for index := range 4096 {
			x, ly, z := index&15, index>>8, index>>4&15
			if needs[source.Blocks.GetIndex(layer_storage.At(x, ly, z))] {
				post[y-low] = append(post[y-low], int16(x|ly<<4|z<<8))
			}
		}
	}
	entities := make([]nbt.Compound, 0, len(chunk.BlockEntities))
	for _, entity := range chunk.BlockEntities {
		if entity.Pos.Y>>4 >= low && entity.Pos.Y>>4 <= high {
			entities = append(entities, entity.NBT)
		}
	}
	root := result.NBT.Value
	root["yPos"] = low
	root["Status"] = "minecraft:full"
	root["LastUpdate"], root["InhabitedTime"] = int64(0), int64(0)
	root["isLightOn"] = int8(0)
	root["Heightmaps"] = nbt.Compound{}
	root["PostProcessing"] = post
	root["block_entities"] = entities
	root["block_ticks"], root["fluid_ticks"] = []nbt.Compound{}, []nbt.Compound{}
	root["structures"] = nbt.Compound{"References": nbt.Compound{}, "starts": nbt.Compound{}}
	return result, nil
}

func needsShapeUpdate(name string) bool {
	// 只登记不会因缺支撑而被邻接重算拆掉的形状方块；登记全部方块会使悬空装饰掉落。
	name = strings.TrimPrefix(name, "minecraft:")
	return strings.HasSuffix(name, "_fence") || strings.HasSuffix(name, "_fence_gate") ||
		name == "glass_pane" || strings.HasSuffix(name, "_stained_glass_pane") || name == "iron_bars" ||
		strings.HasSuffix(name, "_stairs") || strings.HasSuffix(name, "_wall") ||
		name == "redstone_wire" || name == "tripwire" || name == "chorus_plant"
}

func sectionBounds(dimension int32) (int32, int32) {
	// 目标维度的建筑高度决定可保留的 section，源世界可能包含更高的柱子。
	if dimension == 0 {
		return -4, 19
	}
	return 0, 15
}

func JavaDimension(dimension int32) (java.Dimension, bool) {
	// Bedrock 的编号与 Java 目录名不同，映射集中在写出边界。
	switch dimension {
	case 0:
		return java.Overworld, true
	case 1:
		return java.Nether, true
	case 2:
		return java.End, true
	default:
		return "", false
	}
}
