package export

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/java"
	"celler/java/region"
	"neomega-blocks/ir"
	"neomega-converter"
	"neomega-converter/bedrock"
	nbt "neomega-nbt/presets/v1"
)

type ConvertOptions struct {
	// 空维度列表选择主世界；版本零值选择本包目标版本，Strict 只控制转换失败的柱子。
	Dimensions  []int32
	Strict      bool
	DataVersion int32
}

type FailedChunk struct {
	// 错误与位置一起保存，调用方可以定位未写出的柱子。
	Dimension int32
	Pos       celler.ChunkPos
	Err       error
}

type StateCount struct {
	// 未知状态按影响格数排序，便于优先补充影响最大的映射。
	State ir.IRBlock
	Count uint64
}

type Report struct {
	// 计数以实际写出为准；诊断同时保留逐类合计和最常见的十个未知状态。
	Chunks                   map[int32]uint64
	FailedChunks             []FailedChunk
	Diagnostics              map[converter.DiagnosticKind]uint64
	UnknownStates            []StateCount
	UnsupportedBlockEntities map[string]uint64
	DroppedSections          map[int32]uint64
	BlockEntities            uint64
	Entities                 uint64
}

func ConvertWorld(source *cellerbedrock.World, target *java.World, options ConvertOptions) (report Report, err error) {
	// 扫描回调只持有一根柱子，转换和可选修复之间通过磁盘解耦。
	defer func() { err = errors.Join(err, target.Flush()) }()
	report = Report{
		Chunks: make(map[int32]uint64), Diagnostics: make(map[converter.DiagnosticKind]uint64),
		UnsupportedBlockEntities: make(map[string]uint64), DroppedSections: make(map[int32]uint64),
	}
	dims := options.Dimensions
	if len(dims) == 0 {
		dims = []int32{0}
	}
	selected := make(map[int32]java.Dimension)
	for _, dimension := range dims {
		dim, ok := JavaDimension(dimension)
		if !ok {
			return report, fmt.Errorf("unsupported Bedrock dimension %d", dimension)
		}
		selected[dimension] = dim
		report.Chunks[dimension], report.DroppedSections[dimension] = 0, 0
	}
	version := options.DataVersion
	if version == 0 {
		version = DataVersion
	}
	unknown := make(map[ir.IRBlock]uint64)
	defer func() {
		for state, count := range unknown {
			report.UnknownStates = append(report.UnknownStates, StateCount{State: state, Count: count})
		}
		sort.Slice(report.UnknownStates, func(i, j int) bool {
			a, b := report.UnknownStates[i], report.UnknownStates[j]
			if a.Count != b.Count {
				return a.Count > b.Count
			}
			return a.State < b.State
		})
		if len(report.UnknownStates) > 10 {
			report.UnknownStates = report.UnknownStates[:10]
		}
	}()
	err = source.ScanChunks(func(sourceChunk *cellerbedrock.Chunk) error {
		dim, included := selected[sourceChunk.Dimension]
		if !included {
			return nil
		}
		chunk, err := bedrock.ConvertChunk(sourceChunk)
		if err != nil {
			report.FailedChunks = append(report.FailedChunks, FailedChunk{
				Dimension: sourceChunk.Dimension,
				Pos:       sourceChunk.Pos,
				Err:       err,
			})
			if options.Strict {
				return fmt.Errorf("dimension %d chunk %v: %w", sourceChunk.Dimension, sourceChunk.Pos, err)
			}
			return nil
		}
		for _, diagnostic := range chunk.Diagnostics {
			count := diagnostic.Count
			if count == 0 {
				count = 1
			}
			report.Diagnostics[diagnostic.Kind] += count
			switch diagnostic.Kind {
			case converter.UnknownBlock:
				unknown[diagnostic.State] += count
			case converter.UnsupportedBlockEntity:
				report.UnsupportedBlockEntities[diagnostic.BlockEntityID] += count
			}
		}
		low, high := sectionBounds(chunk.Dimension)
		for y := range chunk.Sections {
			if y < low || y > high {
				report.DroppedSections[chunk.Dimension]++
			}
		}
		terrain, err := ToJavaChunk(chunk, version)
		if err != nil {
			return err
		}
		if err = target.WriteChunk(dim, terrain); err != nil {
			return err
		}
		if document := ToJavaEntities(chunk, version); document != nil {
			data, err := nbt.EncodeDocument(*document, nbt.EncodingBigEndian)
			if err != nil {
				return err
			}
			record, err := region.Compress(data, uint32(time.Now().Unix()))
			if err != nil {
				return err
			}
			if err = target.WriteRaw(dim, java.Entities, chunk.Pos, record); err != nil {
				return err
			}
			report.Entities += uint64(len(chunk.Entities))
		}
		entities, _ := nbt.Compounds(terrain.NBT.Value["block_entities"])
		report.BlockEntities += uint64(len(entities))
		report.Chunks[chunk.Dimension]++
		return nil
	})
	return report, err
}
