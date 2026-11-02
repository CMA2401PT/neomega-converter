package adapt

import (
	"container/list"
	"context"
	"sync"

	"celler"
	"neomega-converter"
)

type chunkLoader func(context.Context, celler.ChunkPos) (*converter.ConvertedChunk, error)

type cacheEntry struct {
	pos   celler.ChunkPos
	chunk *converter.ConvertedChunk
	bytes int64
}

type flight struct {
	// 结果通过 done 发布；等待者计数和取消状态由所属缓存的锁保护。
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	chunk   *converter.ConvertedChunk
	err     error
}

type chunkCache struct {
	// 两阶段共享调度实现，各自拥有条目和在途任务，锁内不执行读取、转换或修复。
	mu      sync.Mutex
	limit   int64
	bytes   int64
	order   *list.List
	entries map[celler.ChunkPos]*list.Element
	flights map[celler.ChunkPos]*flight
	load    chunkLoader
}

func newChunkCache(options Options, load chunkLoader) *chunkCache {
	limit := options.CacheBytes
	if limit == 0 {
		limit = 64 << 20
	}
	return &chunkCache{
		limit:   limit,
		order:   list.New(),
		entries: make(map[celler.ChunkPos]*list.Element),
		flights: make(map[celler.ChunkPos]*flight),
		load:    load,
	}
}

func (c *chunkCache) get(ctx context.Context, pos celler.ChunkPos) (*converter.ConvertedChunk, error) {
	// 每个调用只取消自己的等待；最后一个等待者退出时才撤销共享任务。
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if entry, ok := c.entries[pos]; ok {
		c.order.MoveToFront(entry)
		chunk := entry.Value.(cacheEntry).chunk
		c.mu.Unlock()
		return chunk, nil
	}
	work, ok := c.flights[pos]
	if !ok {
		workCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		work = &flight{done: make(chan struct{}), cancel: cancel}
		c.flights[pos] = work
		go c.run(workCtx, pos, work)
	}
	work.waiters++
	c.mu.Unlock()

	select {
	case <-work.done:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return work.chunk, work.err
	case <-ctx.Done():
		c.mu.Lock()
		work.waiters--
		if work.waiters == 0 && c.flights[pos] == work {
			delete(c.flights, pos)
			work.cancel()
		}
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (c *chunkCache) run(ctx context.Context, pos celler.ChunkPos, work *flight) {
	// 已撤销任务可能仍在不可中断的计算中；它结束时不能覆盖同坐标的新任务或缓存。
	defer work.cancel()
	var chunk *converter.ConvertedChunk
	err := ctx.Err()
	if err == nil {
		chunk, err = c.load(ctx, pos)
	}
	var size int64
	if err == nil && ctx.Err() == nil {
		size = chunkBytes(chunk)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil {
		chunk, err = nil, ctx.Err()
	}
	if c.flights[pos] == work {
		delete(c.flights, pos)
		if err == nil {
			c.put(pos, chunk, size)
		}
	}
	work.chunk, work.err = chunk, err
	close(work.done)
}

func (c *chunkCache) put(pos celler.ChunkPos, chunk *converter.ConvertedChunk, size int64) {
	// 缺柱也有条目开销，避免空区域无限积累；超预算的单根结果只交给当前等待者。
	if size > c.limit {
		return
	}
	for c.bytes+size > c.limit {
		oldest := c.order.Back()
		entry := oldest.Value.(cacheEntry)
		delete(c.entries, entry.pos)
		c.order.Remove(oldest)
		c.bytes -= entry.bytes
	}
	c.entries[pos] = c.order.PushFront(cacheEntry{pos: pos, chunk: chunk, bytes: size})
	c.bytes += size
}
