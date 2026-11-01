package bedrock

import (
	"fmt"
	"maps"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	"neomega-converter"
	"neomega-converter/bedrock/blockentities"
	"neomega-converter/bedrock/blockentities/banner"
	"neomega-converter/bedrock/blockentities/bed"
	"neomega-converter/bedrock/blockentities/chest"
	"neomega-converter/bedrock/blockentities/flower_pot"
	"neomega-converter/bedrock/blockentities/jukebox"
	"neomega-converter/bedrock/blockentities/lectern"
	"neomega-converter/bedrock/blockentities/note_block"
	"neomega-converter/bedrock/blockentities/shulker_box"
	"neomega-converter/bedrock/blockentities/sign"
	"neomega-converter/bedrock/blockentities/skull"
)

var entityRules = map[string]blockentities.ConvertFunc{
	// 注册表由调用方持有，让规则子包只依赖共享的输入输出类型。
	bed.ID:         bed.Convert,
	flower_pot.ID:  flower_pot.Convert,
	banner.ID:      banner.Convert,
	shulker_box.ID: shulker_box.Convert,
	jukebox.ID:     jukebox.Convert,
	lectern.ID:     lectern.Convert,
	note_block.ID:  note_block.Convert,
	skull.ID:       skull.Convert,
	chest.ID:       chest.Convert,
	sign.ID:        sign.Convert,
	sign.HangingID: sign.Convert,
}

const air ir.IRBlock = "minecraft:air []"

type blockResult struct {
	// 成功标记与状态一起缓存，未知方块的实际格数由调色板索引另行统计。
	state ir.IRBlock
	ok    bool
}

func ConvertChunk(src *cellerbedrock.Chunk) (*converter.ConvertedChunk, error) {
	// 先转换整根柱子的调色板，再合并源实体与第二层水，结果可供多个邻居修复共享。
	result := &converter.ConvertedChunk{
		Pos:       src.Pos,
		Dimension: src.Dimension,
		Sections:  make(map[int32]*converter.ConvertedSection),
	}
	cache := make(map[cellerbedrock.Block]blockResult)
	convert := func(source cellerbedrock.Block) ir.IRBlock {
		if cached, exists := cache[source]; exists {
			return cached.state
		}
		state, ok := converts.ConvertToJava(source.State)
		if ok {
			state = state.Tidy()
		} else {
			state = air
		}
		cache[source] = blockResult{state: state, ok: ok}
		return state
	}
	sources := make(map[int32]*cellerbedrock.Section)
	unknowns := make(map[ir.IRBlock]int)
	for _, y := range src.SectionYs() {
		section, err := src.Section(y)
		if err != nil {
			return nil, fmt.Errorf("section %d: %w", y, err)
		}
		sources[y] = section
		blocks := layer_storage.MakePaletteStorage(air)
		if len(section.Layers) > 0 {
			layer := section.Layers[0].Blocks
			blocks = layer_storage.ConvertPalette(layer, convert)
			counts := make([]uint64, len(layer.GetUsedPalette()))
			for index := range 4096 {
				counts[layer.GetIndex(layer_storage.At(index>>8, index&15, index>>4&15))]++
			}
			for index, source := range layer.GetUsedPalette() {
				if cache[source].ok || counts[index] == 0 {
					continue
				}
				diagnostic, exists := unknowns[source.State]
				if !exists {
					diagnostic = len(result.Diagnostics)
					unknowns[source.State] = diagnostic
					result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
						Kind:  converter.UnknownBlock,
						State: source.State,
						Err:   fmt.Errorf("unknown Bedrock block: %s", source.State),
					})
				}
				result.Diagnostics[diagnostic].Count += counts[index]
			}
		}
		result.Sections[y] = &converter.ConvertedSection{Blocks: blocks}
	}
	entityCells := convertEntities(src, result, sources, cache)
	for _, y := range src.SectionYs() {
		section := sources[y]
		if len(section.Layers) > 1 {
			applyWater(section, result.Sections[y].Blocks, entityCells[y], cache)
		}
	}
	return result, nil
}

func convertEntities(
	src *cellerbedrock.Chunk,
	result *converter.ConvertedChunk,
	sources map[int32]*cellerbedrock.Section,
	cache map[cellerbedrock.Block]blockResult,
) map[int32]map[layer_storage.Position]bool {
	// 实体规则依赖单格 NBT，改过的格子单独标记，避免混入层组合的调色板缓存。
	changed := make(map[int32]map[layer_storage.Position]bool)
	entities, err := src.BlockEntities()
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{Kind: converter.NBTError, Err: err})
		return changed
	}
	for _, document := range entities {
		entity := document.Value
		x, xOK := entity["x"].(int32)
		y, yOK := entity["y"].(int32)
		z, zOK := entity["z"].(int32)
		world := celler.BlockPos{X: x, Y: y, Z: z}
		section := result.Sections[y>>4]
		if !xOK || !yOK || !zOK || world.Chunk() != src.Pos || section == nil {
			result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
				Kind: converter.RuleError,
				Err:  fmt.Errorf("block entity requires Int x/y/z in a decoded section of its chunk"),
			})
			continue
		}
		local := layer_storage.At(x, y, z)
		base := section.Blocks.Get(local)
		id, ok := entity["id"].(string)
		if !ok {
			result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
				Kind: converter.RuleError, Pos: &world, State: base,
				Err: fmt.Errorf("block entity requires string id"),
			})
			continue
		}
		rule := entityRules[id]
		if rule == nil {
			continue
		}
		source := sources[y>>4]
		if len(source.Layers) > 0 && !cache[source.Layers[0].Blocks.Get(local)].ok {
			continue
		}
		output, err := rule(blockentities.Input{
			Base:     base,
			Source:   entity,
			LocalPos: [3]uint8{local.X, local.Y, local.Z},
			WorldPos: world,
		})
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, converter.Diagnostic{
				Kind: converter.RuleError, Pos: &world, State: base, Err: err,
			})
			continue
		}
		section.Blocks.Set(local, output.State.Tidy())
		if changed[y>>4] == nil {
			changed[y>>4] = make(map[layer_storage.Position]bool)
		}
		changed[y>>4][local] = true
		if output.BlockEntity != nil {
			value := maps.Clone(output.BlockEntity)
			value["x"], value["y"], value["z"] = x, y, z
			result.BlockEntities = append(result.BlockEntities, java.BlockEntity{Pos: world, NBT: value})
		}
	}
	return changed
}

func applyWater(
	source *cellerbedrock.Section,
	blocks *layer_storage.Palette[ir.IRBlock],
	entityCells map[layer_storage.Position]bool,
	cache map[cellerbedrock.Block]blockResult,
) {
	// 只缓存实际出现的层组合；实体规则产出的状态逐格附加含水属性。
	base, overlay := source.Layers[0].Blocks, source.Layers[1].Blocks
	combinations := make(map[[2]int]ir.IRBlock)
	for index := range 4096 {
		pos := layer_storage.At(index>>8, index&15, index>>4&15)
		if !cache[base.Get(pos)].ok {
			continue
		}
		name := overlay.Get(pos).State.NameWithNameSpace()
		if name != "minecraft:water" && name != "minecraft:flowing_water" {
			continue
		}
		key := [2]int{base.GetIndex(pos), overlay.GetIndex(pos)}
		state, exists := combinations[key]
		if !exists || entityCells[pos] {
			state = blocks.Get(pos)
			state = ir.IRBlock(state.NameWithNameSpace() + " " + string(state.States().Set("waterlogged", "true"))).Tidy()
			if !entityCells[pos] {
				combinations[key] = state
			}
		}
		blocks.Set(pos, state)
	}
}
