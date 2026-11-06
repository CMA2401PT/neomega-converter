package export

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"celler"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	repair "neomega-converter"
	"neomega-converter-core"
	nbt "neomega-nbt/presets/v1"
	"neomega-viewer-core/fixer/java-postprocess"
)

type FixOptions struct {
	// Context 为 nil 时使用 Background；取消只停止后续区块，已写出的修改仍保留。
	Context context.Context
	// 空列表选择主世界；独立修复入口也允许 Java 自定义维度目录。
	Dimensions []cellerjava.Dimension
}

type FixReport struct {
	// 柱子统计包含只有光照的柱子，改动统计只包含实际替换的方块 section。
	Chunks          uint64
	ChangedSections uint64
}

type fixRow struct {
	// 行只保存坐标，方块数据由三行窗口按需持有。
	z         int32
	positions []celler.ChunkPos
}

func FixWorld(world *cellerjava.World, f *fixer.Fixer, options FixOptions) (report FixReport, err error) {
	// 窗口保留写回前的原值，使逐柱写回不会改变后续修复的邻居。
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	defer func() { err = errors.Join(err, world.Flush()) }()
	dimensions := options.Dimensions
	if len(dimensions) == 0 {
		dimensions = []cellerjava.Dimension{cellerjava.Overworld}
	}
	seen := make(map[cellerjava.Dimension]bool)
	for _, dim := range dimensions {
		if seen[dim] {
			continue
		}
		seen[dim] = true
		var positions []celler.ChunkPos
		err = world.Scan(dim, cellerjava.Terrain, func(pos celler.ChunkPos) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			positions = append(positions, pos)
			return nil
		})
		if err != nil {
			return report, err
		}
		slices.SortFunc(positions, func(a, b celler.ChunkPos) int {
			if order := cmp.Compare(a.Z, b.Z); order != 0 {
				return order
			}
			return cmp.Compare(a.X, b.X)
		})
		var rows []fixRow
		for start := 0; start < len(positions); {
			end := start + 1
			for end < len(positions) && positions[end].Z == positions[start].Z {
				end++
			}
			rows = append(rows, fixRow{z: positions[start].Z, positions: positions[start:end]})
			start = end
		}
		window := make(map[int32]map[celler.ChunkPos]*converter.ConvertedChunk)
		nextRow := 0
		for _, row := range rows {
			// 稀疏存档可能跳过若干 z；先淘汰过期行，加载游标只向前，绝不重读已写回行。
			for z := range window {
				if z < row.z-1 {
					delete(window, z)
				}
			}
			for nextRow < len(rows) && rows[nextRow].z <= row.z+1 {
				incoming := rows[nextRow]
				chunks := make(map[celler.ChunkPos]*converter.ConvertedChunk, len(incoming.positions))
				window[incoming.z] = chunks
				for _, pos := range incoming.positions {
					if err := ctx.Err(); err != nil {
						return report, err
					}
					blocks, err := readFixBlocks(world, dim, pos)
					if err != nil {
						return report, err
					}
					chunks[pos] = blocks
					report.Chunks++
				}
				nextRow++
			}
			for _, pos := range row.positions {
				if err := ctx.Err(); err != nil {
					return report, err
				}
				center := window[row.z][pos]
				var neighbours [8]*converter.ConvertedChunk
				for side, offset := range [8][2]int32{
					{-1, -1}, {0, -1}, {1, -1}, {-1, 0},
					{1, 0}, {-1, 1}, {0, 1}, {1, 1},
				} {
					neighbours[side] = window[pos.Z+offset[1]][celler.ChunkPos{
						X: pos.X + offset[0], Z: pos.Z + offset[1],
					}]
				}
				fixed := repair.FixChunk(f, center, neighbours)
				changed, err := writeFixedBlocks(world, dim, center, fixed)
				if err != nil {
					return report, err
				}
				report.ChangedSections += changed
			}
			delete(window, row.z-1)
		}
	}
	return report, ctx.Err()
}

func readFixBlocks(
	world *cellerjava.World,
	dim cellerjava.Dimension,
	pos celler.ChunkPos,
) (*converter.ConvertedChunk, error) {
	// 窗口只持有方块调色板，完整 NBT 留到实际写回时重新读取。
	chunk, err := world.ReadChunk(dim, pos)
	if err != nil {
		return nil, err
	}
	ys, err := chunk.SectionYs()
	if err != nil {
		return nil, err
	}
	blocks := &converter.ConvertedChunk{Pos: pos, Sections: make(map[int32]*converter.ConvertedSection)}
	for _, y := range ys {
		section, err := chunk.Section(y)
		if err != nil {
			return nil, err
		}
		if section.Blocks != nil {
			blocks.Sections[y] = &converter.ConvertedSection{Blocks: section.Blocks}
		}
	}
	return blocks, nil
}

func writeFixedBlocks(
	world *cellerjava.World,
	dim cellerjava.Dimension,
	center *converter.ConvertedChunk,
	fixed *converter.ConvertedChunk,
) (uint64, error) {
	// 惰性重读保留未修改的 NBT，写回对象不进入原值窗口。
	var chunk *cellerjava.Chunk
	var changed uint64
	for y, section := range fixed.Sections {
		if section.Blocks == center.Sections[y].Blocks {
			continue
		}
		if chunk == nil {
			var err error
			chunk, err = world.ReadChunk(dim, center.Pos)
			if err != nil {
				return 0, err
			}
		}
		if err := replaceBlocks(chunk, y, section.Blocks); err != nil {
			return 0, err
		}
		changed++
	}
	if chunk != nil {
		if err := world.WriteChunk(dim, chunk); err != nil {
			return 0, err
		}
	}
	return changed, nil
}

func replaceBlocks(chunk *cellerjava.Chunk, y int32, blocks *layer_storage.Palette[ir.IRBlock]) error {
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
