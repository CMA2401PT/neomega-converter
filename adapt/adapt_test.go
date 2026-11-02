package adapt

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	"neomega-converter"
	"neomega-converter/fixer"
)

func memoryChunk(t *testing.T, pos celler.ChunkPos, blocks map[layer_storage.Position]ir.IRBlock) *cellerbedrock.Chunk {
	// 测试入口使用真实 Bedrock 记录，覆盖解码、转换和修复的组合。
	t.Helper()
	chunk := cellerbedrock.NewChunk(pos, 2)
	layer := cellerbedrock.NewLayer(cellerbedrock.Block{State: "minecraft:air []", Version: 18168865})
	for local, state := range blocks {
		layer.Blocks.Set(local, cellerbedrock.Block{State: state, Version: 18168865})
	}
	if err := chunk.SetSection(&cellerbedrock.Section{
		Y: 0, Version: 9, Layers: []*cellerbedrock.Layer{layer},
	}); err != nil {
		t.Fatal(err)
	}
	return chunk
}

func memoryRead(chunks map[celler.ChunkPos]*cellerbedrock.Chunk) ReadChunk {
	return func(ctx context.Context, pos celler.ChunkPos) (*cellerbedrock.Chunk, error) {
		if chunk := chunks[pos]; chunk != nil {
			return chunk, nil
		}
		return nil, fmt.Errorf("chunk %v: %w", pos, fs.ErrNotExist)
	}
}

func newFixer(t *testing.T) *fixer.Fixer {
	t.Helper()
	f, err := fixer.New()
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func getChunk(t *testing.T, src ChunkSource, pos celler.ChunkPos) *converter.ConvertedChunk {
	t.Helper()
	chunk, err := src.Chunk(context.Background(), pos)
	if err != nil || chunk == nil {
		t.Fatalf("Chunk(%v) = %v, %v", pos, chunk, err)
	}
	return chunk
}

func blockAt(chunk *converter.ConvertedChunk, x, z int) ir.IRBlock {
	return chunk.Sections[0].Blocks.Get(layer_storage.At(x, 4, z))
}

func assertState(t *testing.T, block ir.IRBlock, name, want string) {
	t.Helper()
	got, _ := block.States().Get(name)
	if got != want {
		t.Fatalf("%s: %s = %v, want %s", block, name, got, want)
	}
}

func fenceChunks(t *testing.T) map[celler.ChunkPos]*cellerbedrock.Chunk {
	t.Helper()
	west, east := celler.ChunkPos{X: -1, Z: 2}, celler.ChunkPos{Z: 2}
	return map[celler.ChunkPos]*cellerbedrock.Chunk{
		west: memoryChunk(t, west, map[layer_storage.Position]ir.IRBlock{
			layer_storage.At(15, 4, 8): "minecraft:oak_fence []",
		}),
		east: memoryChunk(t, east, map[layer_storage.Position]ir.IRBlock{
			layer_storage.At(0, 4, 8): "minecraft:oak_fence []",
		}),
	}
}

func TestConvertedAndFixedAssembly(t *testing.T) {
	chunks := fenceChunks(t)
	var reads atomic.Int32
	read := memoryRead(chunks)
	converted := NewConverted(func(ctx context.Context, pos celler.ChunkPos) (*cellerbedrock.Chunk, error) {
		reads.Add(1)
		return read(ctx, pos)
	}, Options{})
	west, east := celler.ChunkPos{X: -1, Z: 2}, celler.ChunkPos{Z: 2}
	var source ChunkSource = converted
	raw := getChunk(t, source, west)
	assertState(t, blockAt(raw, 15, 8), "east", "false")
	if reads.Load() != 1 || raw.Pos != west || raw.Dimension != 2 {
		t.Fatal("只转换应只读中心，并保留来源坐标和维度")
	}
	source = NewFixed(converted, newFixer(t), Options{})
	assertState(t, blockAt(getChunk(t, source, west), 15, 8), "east", "true")
	assertState(t, blockAt(getChunk(t, source, east), 0, 8), "west", "true")
	assertState(t, blockAt(raw, 15, 8), "east", "false")
	if getChunk(t, converted, west) != raw {
		t.Fatal("修复覆盖了转换缓存")
	}
}

func TestFixedUsesUnfixedNeighbours(t *testing.T) {
	// 东柱楼梯修复为内拐角后才有完整西面；西柱栅栏必须仍按原始直楼梯判断。
	west, east := celler.ChunkPos{X: -1}, celler.ChunkPos{}
	stairs := func(facing string) ir.IRBlock {
		java, ok := converts.JavaCanon(ir.IRBlock(fmt.Sprintf(
			`minecraft:oak_stairs ["facing"="%s","half"="bottom","shape"="straight"]`,
			facing,
		)))
		if !ok {
			t.Fatal("楼梯状态无法补全")
		}
		state, ok := converts.ConvertFromJava(java)
		if !ok {
			t.Fatal("楼梯状态无法转成 Bedrock")
		}
		return state
	}
	chunks := map[celler.ChunkPos]*cellerbedrock.Chunk{
		west: memoryChunk(t, west, map[layer_storage.Position]ir.IRBlock{
			layer_storage.At(15, 4, 8): "minecraft:oak_fence []",
		}),
		east: memoryChunk(t, east, map[layer_storage.Position]ir.IRBlock{
			layer_storage.At(0, 4, 8): stairs("north"),
			layer_storage.At(0, 4, 9): stairs("west"),
		}),
	}
	converted := NewConverted(memoryRead(chunks), Options{})
	f := newFixer(t)
	fixed := NewFixed(converted, f, Options{})
	rawWest, rawEast := getChunk(t, converted, west), getChunk(t, converted, east)
	fixedEast := getChunk(t, fixed, east)
	assertState(t, blockAt(rawEast, 0, 8), "shape", "straight")
	assertState(t, blockAt(fixedEast, 0, 8), "shape", "inner_left")
	wrong := f.Fix(rawWest, fixer.ChunkNeighbours{fixer.E: fixedEast})
	assertState(t, blockAt(wrong, 15, 8), "east", "true")
	got := getChunk(t, fixed, west)
	assertState(t, blockAt(got, 15, 8), "east", "false")
	assertState(t, blockAt(rawEast, 0, 8), "shape", "straight")
}

func TestConcurrentRequestsShareBothStages(t *testing.T) {
	chunks := fenceChunks(t)
	f := newFixer(t)
	synctest.Test(t, func(t *testing.T) {
		gate := make(chan struct{})
		var mu sync.Mutex
		reads := make(map[celler.ChunkPos]int)
		read := memoryRead(chunks)
		converted := NewConverted(func(ctx context.Context, pos celler.ChunkPos) (*cellerbedrock.Chunk, error) {
			mu.Lock()
			reads[pos]++
			mu.Unlock()
			<-gate
			return read(ctx, pos)
		}, Options{})
		fixed := NewFixed(converted, f, Options{})
		var conversions, repairs atomic.Int32
		convertLoad, fixLoad := converted.cache.load, fixed.cache.load
		converted.cache.load = func(ctx context.Context, pos celler.ChunkPos) (*converter.ConvertedChunk, error) {
			chunk, err := convertLoad(ctx, pos)
			if chunk != nil {
				conversions.Add(1)
			}
			return chunk, err
		}
		fixed.cache.load = func(ctx context.Context, pos celler.ChunkPos) (*converter.ConvertedChunk, error) {
			repairs.Add(1)
			return fixLoad(ctx, pos)
		}
		const callers = 12
		var raw, repaired [callers]*converter.ConvertedChunk
		var wg sync.WaitGroup
		pos := celler.ChunkPos{X: -1, Z: 2}
		for i := range callers {
			wg.Go(func() { raw[i] = getChunk(t, converted, pos) })
			wg.Go(func() { repaired[i] = getChunk(t, fixed, pos) })
		}
		synctest.Wait()
		close(gate)
		wg.Wait()
		for i := range callers {
			if raw[i] != raw[0] || repaired[i] != repaired[0] {
				t.Fatal("并发调用没有共享同一次计算的结果")
			}
		}
		if conversions.Load() != 2 || repairs.Load() != 1 || len(reads) != 9 {
			t.Fatalf("conversions=%d repairs=%d read positions=%d", conversions.Load(), repairs.Load(), len(reads))
		}
		for pos, count := range reads {
			if count != 1 {
				t.Fatalf("read %v: %d", pos, count)
			}
		}
	})
}

func TestEvictionPreservesResults(t *testing.T) {
	// 让两层预算都只容纳一根有内容的柱子，跨区读取后必须能重新生成相同方块。
	chunks := fenceChunks(t)
	read := memoryRead(chunks)
	baseline := NewConverted(read, Options{})
	west, east := celler.ChunkPos{X: -1, Z: 2}, celler.ChunkPos{Z: 2}
	f := newFixer(t)
	want := getChunk(t, NewFixed(baseline, f, Options{}), west)
	limit := max(chunkBytes(getChunk(t, baseline, west)), chunkBytes(want))
	var reads atomic.Int32
	converted := NewConverted(func(ctx context.Context, pos celler.ChunkPos) (*cellerbedrock.Chunk, error) {
		if pos == west {
			reads.Add(1)
		}
		return read(ctx, pos)
	}, Options{CacheBytes: limit})
	fixed := NewFixed(converted, f, Options{CacheBytes: limit})
	first := getChunk(t, fixed, west)
	getChunk(t, fixed, east)
	again := getChunk(t, fixed, west)
	if first == again || reads.Load() < 2 {
		t.Fatal("没有实际淘汰并重新计算")
	}
	if !reflect.DeepEqual(first, want) || !reflect.DeepEqual(again, want) {
		t.Fatal("缓存淘汰改变了结果")
	}
	for _, cache := range []*chunkCache{converted.cache, fixed.cache} {
		if cache.bytes > limit {
			t.Fatal("缓存超过预算")
		}
	}
}

func TestMissingChunkAndErrors(t *testing.T) {
	var reads int
	converted := NewConverted(func(ctx context.Context, pos celler.ChunkPos) (*cellerbedrock.Chunk, error) {
		reads++
		return nil, fmt.Errorf("missing: %w", fs.ErrNotExist)
	}, Options{})
	fixed := NewFixed(converted, newFixer(t), Options{})
	for range 2 {
		for _, source := range []ChunkSource{converted, fixed} {
			got, err := source.Chunk(context.Background(), celler.ChunkPos{})
			if got != nil || err != nil {
				t.Fatalf("missing = %v, %v", got, err)
			}
		}
	}
	if reads != 1 {
		t.Fatal("缺柱未缓存，或不存在的中心仍读取了邻居")
	}
	broken := memoryChunk(t, celler.ChunkPos{}, nil)
	broken.Records[string([]byte{cellerbedrock.TagSubChunk, 0})] = []byte{255}
	failure := errors.New("read failed")
	for _, test := range []struct {
		name string
		raw  *cellerbedrock.Chunk
		err  error
	}{
		{name: "read", err: failure},
		{name: "decode", raw: broken},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			converted := NewConverted(func(context.Context, celler.ChunkPos) (*cellerbedrock.Chunk, error) {
				calls++
				return test.raw, test.err
			}, Options{})
			fixed := NewFixed(converted, newFixer(t), Options{})
			for range 2 {
				_, err := fixed.Chunk(context.Background(), celler.ChunkPos{})
				if err == nil || (test.err != nil && !errors.Is(err, test.err)) {
					t.Fatalf("error = %v", err)
				}
			}
			if calls != 2 {
				t.Fatal("错误被缓存，阻止了重试")
			}
		})
	}
}
