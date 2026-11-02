package adapt

import (
	"context"

	"celler"
	repair "neomega-converter"
	"neomega-converter-core"
	coreadapt "neomega-converter-core/adapt"
	"neomega-viewer-core/fixer/java-postprocess"
)

type Fixed struct {
	// 修复结果单独缓存，不能覆盖供其他中心使用的阶段 1 数据。
	cache *coreadapt.CachedSource
}

func NewFixed(src *coreadapt.Converted, f *fixer.Fixer, options coreadapt.Options) *Fixed {
	// 具体输入类型在组装时排除 Fixed，保证八邻居始终来自未修复的转换结果。
	load := func(
		ctx context.Context,
		pos celler.ChunkPos,
	) (*converter.ConvertedChunk, error) {
		center, err := src.Chunk(ctx, pos)
		if err != nil || center == nil {
			return nil, err
		}
		var neighbours [8]*converter.ConvertedChunk
		for side, offset := range [8]celler.ChunkPos{
			fixer.NW: {X: -1, Z: -1},
			fixer.N:  {Z: -1},
			fixer.NE: {X: 1, Z: -1},
			fixer.W:  {X: -1},
			fixer.E:  {X: 1},
			fixer.SW: {X: -1, Z: 1},
			fixer.S:  {Z: 1},
			fixer.SE: {X: 1, Z: 1},
		} {
			neighbours[side], err = src.Chunk(ctx, celler.ChunkPos{
				X: pos.X + offset.X,
				Z: pos.Z + offset.Z,
			})
			if err != nil {
				return nil, err
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return repair.FixChunk(f, center, neighbours), nil
	}
	return &Fixed{cache: coreadapt.NewCachedSource(options, load)}
}

func (f *Fixed) Chunk(ctx context.Context, pos celler.ChunkPos) (*converter.ConvertedChunk, error) {
	// 整柱修复只执行一次，多个高度窗口与邻柱请求可以复用同一结果。
	return f.cache.Chunk(ctx, pos)
}
