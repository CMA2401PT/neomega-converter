package export

import (
	"errors"

	"celler"
	"celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter"
	"neomega-converter/fixer"
	nbt "neomega-nbt/presets/v1"
)

type FixOptions struct {
	// 空列表选择主世界；独立修复入口也允许 Java 自定义维度目录。
	Dimensions []java.Dimension
}

type FixReport struct {
	// 柱子统计包含只有光照的柱子，改动统计只包含实际替换的方块 section。
	Chunks          uint64
	ChangedSections uint64
}

type chunkChanges struct {
	// 延迟写回只保留改动调色板，原始世界始终是所有修复调用的邻居来源。
	dimension java.Dimension
	pos       celler.ChunkPos
	sections  map[int32]*layer_storage.Palette[ir.IRBlock]
}

func FixWorld(world *java.World, f *fixer.Fixer, options FixOptions) (report FixReport, err error) {
	// 全部修复成功前不写回，避免某个柱子的结果影响后面的邻居判断。
	defer func() { err = errors.Join(err, world.Flush()) }()
	dimensions := options.Dimensions
	if len(dimensions) == 0 {
		dimensions = []java.Dimension{java.Overworld}
	}
	var changes []chunkChanges
	seen := make(map[java.Dimension]bool)
	for _, dim := range dimensions {
		if seen[dim] {
			continue
		}
		seen[dim] = true
		chunks := make(map[celler.ChunkPos]*converter.ConvertedChunk)
		var positions []celler.ChunkPos
		err = world.Scan(dim, java.Terrain, func(pos celler.ChunkPos) error {
			chunk, err := world.ReadChunk(dim, pos)
			if err != nil {
				return err
			}
			ys, err := chunk.SectionYs()
			if err != nil {
				return err
			}
			blocks := &converter.ConvertedChunk{Pos: pos, Sections: make(map[int32]*converter.ConvertedSection)}
			for _, y := range ys {
				section, err := chunk.Section(y)
				if err != nil {
					return err
				}
				if section.Blocks != nil {
					blocks.Sections[y] = &converter.ConvertedSection{Blocks: section.Blocks}
				}
			}
			chunks[pos] = blocks
			positions = append(positions, pos)
			report.Chunks++
			return nil
		})
		if err != nil {
			return report, err
		}
		for _, pos := range positions {
			center := chunks[pos]
			var neighbours fixer.ChunkNeighbours
			for side, offset := range [8][2]int32{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}} {
				neighbours[side] = chunks[celler.ChunkPos{X: pos.X + offset[0], Z: pos.Z + offset[1]}]
			}
			fixed := f.Fix(center, neighbours)
			changed := chunkChanges{dimension: dim, pos: pos, sections: make(map[int32]*layer_storage.Palette[ir.IRBlock])}
			for y, section := range fixed.Sections {
				if section.Blocks != center.Sections[y].Blocks {
					changed.sections[y] = section.Blocks
				}
			}
			if len(changed.sections) > 0 {
				changes = append(changes, changed)
			}
		}
	}
	for _, change := range changes {
		chunk, err := world.ReadChunk(change.dimension, change.pos)
		if err != nil {
			return report, err
		}
		for y, blocks := range change.sections {
			if err = replaceBlocks(chunk, y, blocks); err != nil {
				return report, err
			}
		}
		if err = world.WriteChunk(change.dimension, chunk); err != nil {
			return report, err
		}
		report.ChangedSections += uint64(len(change.sections))
	}
	return report, nil
}

func replaceBlocks(chunk *java.Chunk, y int32, blocks *layer_storage.Palette[ir.IRBlock]) error {
	// SetSection 会重编码生物群系；只取它编码出的方块字段，其他 NBT 保持磁盘原值。
	section, err := chunk.Section(y)
	if err != nil {
		return err
	}
	section.Blocks = blocks
	if err = chunk.SetSection(section); err != nil {
		return err
	}
	sections, _ := nbt.Compounds(chunk.NBT.Value["sections"])
	for index, value := range sections {
		if value["Y"] == int8(y) {
			section.NBT["block_states"] = value["block_states"]
			sections[index] = section.NBT
			break
		}
	}
	return nil
}
