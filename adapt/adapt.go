package adapt

import (
	"context"
	"errors"
	"io/fs"

	"celler"
	cellerbedrock "celler/bedrock"
	"neomega-converter"
	"neomega-converter/bedrock"
	"neomega-converter/fixer"
)

type ChunkSource interface {
	// 消费方只需按坐标取柱子；缺失返回 nil, nil，结果及其引用的数据共享只读。
	Chunk(ctx context.Context, pos celler.ChunkPos) (*converter.ConvertedChunk, error)
}

type ReadChunk func(context.Context, celler.ChunkPos) (*cellerbedrock.Chunk, error)

// 回调绑定一个来源和维度，缺失沿用 celler 的 fs.ErrNotExist。
// 不同坐标可能并发读取，回调负责底层句柄的并发与关闭，并在返回后保持记录只读。

type Options struct {
	// 每层按估算字节独立淘汰最久未使用的结果；零值使用 64 MiB，负值只合并在途请求。
	CacheBytes int64
}

type Converted struct {
	// 缓存只保存阶段 1，让多个中心修复时共享相同的原始邻居。
	cache *chunkCache
}

func NewConverted(read ReadChunk, options Options) *Converted {
	// 来源内容在实例存活期间保持稳定；重新打开或修改来源后创建新实例。
	load := func(
		ctx context.Context,
		pos celler.ChunkPos,
	) (*converter.ConvertedChunk, error) {
		raw, err := read(ctx, pos)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return bedrock.ConvertChunk(raw)
	}
	return &Converted{cache: newChunkCache(options, load)}
}

func (c *Converted) Chunk(ctx context.Context, pos celler.ChunkPos) (*converter.ConvertedChunk, error) {
	// 单个等待者退出不取消其他调用者正在共用的读取和转换。
	return c.cache.get(ctx, pos)
}

type Fixed struct {
	// 修复结果单独缓存，不能覆盖供其他中心使用的阶段 1 数据。
	cache *chunkCache
}

func NewFixed(src *Converted, f *fixer.Fixer, options Options) *Fixed {
	// 具体输入类型在组装时排除 Fixed，保证八邻居始终来自未修复的转换结果。
	load := func(
		ctx context.Context,
		pos celler.ChunkPos,
	) (*converter.ConvertedChunk, error) {
		center, err := src.Chunk(ctx, pos)
		if err != nil || center == nil {
			return nil, err
		}
		var neighbours fixer.ChunkNeighbours
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
		return f.Fix(center, neighbours), nil
	}
	return &Fixed{cache: newChunkCache(options, load)}
}

func (f *Fixed) Chunk(ctx context.Context, pos celler.ChunkPos) (*converter.ConvertedChunk, error) {
	// 整柱修复只执行一次，多个高度窗口与邻柱请求可以复用同一结果。
	return f.cache.get(ctx, pos)
}
