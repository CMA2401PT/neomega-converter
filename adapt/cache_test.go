package adapt

import (
	"context"
	"errors"
	"io/fs"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"celler"
	cellerbedrock "celler/bedrock"
	"neomega-converter-core"
	coreadapt "neomega-converter-core/be2je/adapt"
	"neomega-viewer-core/fixer/java-postprocess"
)

type chunkReply struct {
	chunk *converter.JavaChunk
	err   error
}

func requestChunk(ctx context.Context, src coreadapt.ChunkSource) <-chan chunkReply {
	// 用结果通道观察取消，而不依赖真实时钟或 goroutine 调度速度。
	done := make(chan chunkReply, 1)
	go func() {
		chunk, err := src.Chunk(ctx, celler.ChunkPos{})
		done <- chunkReply{chunk: chunk, err: err}
	}()
	return done
}

func sourceForStage(read coreadapt.ReadChunk, stage string, f *fixer.Fixer) coreadapt.ChunkSource {
	converted := coreadapt.NewConverted(read, coreadapt.Options{})
	if stage == "fixed" {
		return NewFixed(converted, f, coreadapt.Options{})
	}
	return converted
}

func TestCancellationKeepsOtherWaiters(t *testing.T) {
	raw := memoryChunk(t, celler.ChunkPos{}, nil)
	f := newFixer(t)
	for _, stage := range []string{"converted", "fixed"} {
		for _, test := range []struct {
			name        string
			cancelFirst bool
		}{
			{name: "initiator", cancelFirst: true},
			{name: "follower"},
		} {
			t.Run(stage+"/"+test.name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					gate := make(chan struct{})
					var reads, cancelled atomic.Int32
					source := sourceForStage(func(ctx context.Context, pos celler.ChunkPos) (*cellerbedrock.Chunk, error) {
						if pos != (celler.ChunkPos{}) {
							return nil, fs.ErrNotExist
						}
						reads.Add(1)
						select {
						case <-gate:
							return raw, nil
						case <-ctx.Done():
							cancelled.Add(1)
							return nil, ctx.Err()
						}
					}, stage, f)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					firstCtx, secondCtx := context.Background(), context.Background()
					if test.cancelFirst {
						firstCtx = ctx
					} else {
						secondCtx = ctx
					}
					first := requestChunk(firstCtx, source)
					synctest.Wait()
					second := requestChunk(secondCtx, source)
					synctest.Wait()
					cancelledReply, remainingReply := first, second
					if !test.cancelFirst {
						cancelledReply, remainingReply = second, first
					}
					cancel()
					synctest.Wait()
					if reply := <-cancelledReply; !errors.Is(reply.err, context.Canceled) {
						t.Fatalf("cancelled waiter = %v", reply.err)
					}
					if cancelled.Load() != 0 {
						t.Fatal("一个等待者取消了其他等待者共用的任务")
					}
					close(gate)
					reply := <-remainingReply
					if reply.err != nil || reply.chunk == nil || reads.Load() != 1 {
						t.Fatalf("remaining = %+v, reads=%d", reply, reads.Load())
					}
					if got := getChunk(t, source, celler.ChunkPos{}); got != reply.chunk {
						t.Fatal("共享任务的成功结果未缓存")
					}
				})
			})
		}
	}
}

func TestLastWaiterDeadlineCancelsRead(t *testing.T) {
	f := newFixer(t)
	for _, stage := range []string{"converted", "fixed"} {
		t.Run(stage, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var reads atomic.Int32
				stopped := make(chan struct{})
				source := sourceForStage(func(ctx context.Context, pos celler.ChunkPos) (*cellerbedrock.Chunk, error) {
					reads.Add(1)
					<-ctx.Done()
					close(stopped)
					return nil, ctx.Err()
				}, stage, f)
				cancelled, cancel := context.WithCancel(context.Background())
				cancel()
				if _, err := source.Chunk(cancelled, celler.ChunkPos{}); !errors.Is(err, context.Canceled) || reads.Load() != 0 {
					t.Fatal("已取消的请求仍启动了读取")
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_, err := source.Chunk(ctx, celler.ChunkPos{})
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("deadline = %v", err)
				}
				<-stopped
				if reads.Load() != 1 {
					t.Fatalf("reads=%d", reads.Load())
				}
			})
		})
	}
}

func TestAbandonedWorkCannotReplaceRetry(t *testing.T) {
	// 模拟不支持中途取消的底层读取，让旧任务晚于重试启动完成。
	raw := memoryChunk(t, celler.ChunkPos{}, nil)
	f := newFixer(t)
	for _, stage := range []string{"converted", "fixed"} {
		t.Run(stage, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				oldGate, newGate := make(chan struct{}), make(chan struct{})
				var reads atomic.Int32
				source := sourceForStage(func(ctx context.Context, pos celler.ChunkPos) (*cellerbedrock.Chunk, error) {
					if pos != (celler.ChunkPos{}) {
						return nil, fs.ErrNotExist
					}
					if reads.Add(1) == 1 {
						<-oldGate
					} else {
						<-newGate
					}
					return raw, nil
				}, stage, f)
				ctx, cancel := context.WithCancel(context.Background())
				first := requestChunk(ctx, source)
				synctest.Wait()
				cancel()
				synctest.Wait()
				if reply := <-first; !errors.Is(reply.err, context.Canceled) {
					t.Fatalf("first = %+v", reply)
				}
				second := requestChunk(context.Background(), source)
				synctest.Wait()
				close(oldGate)
				synctest.Wait()
				third := requestChunk(context.Background(), source)
				synctest.Wait()
				close(newGate)
				a, b := <-second, <-third
				if a.err != nil || b.err != nil || a.chunk == nil || a.chunk != b.chunk || reads.Load() != 2 {
					t.Fatalf("retry=%+v follower=%+v reads=%d", a, b, reads.Load())
				}
				if getChunk(t, source, celler.ChunkPos{}) != a.chunk {
					t.Fatal("废弃任务覆盖了重试结果")
				}
			})
		})
	}
}

func TestFiveFixedColumnsReadTwentyOneConvertedColumns(t *testing.T) {
	chunks := make(map[celler.ChunkPos]*cellerbedrock.Chunk)
	for x := int32(-2); x <= 2; x++ {
		for z := int32(-2); z <= 2; z++ {
			pos := celler.ChunkPos{X: x, Z: z}
			chunks[pos] = memoryChunk(t, pos, nil)
		}
	}
	var reads int
	read := memoryRead(chunks)
	converted := coreadapt.NewConverted(func(ctx context.Context, pos celler.ChunkPos) (*cellerbedrock.Chunk, error) {
		reads++
		return read(ctx, pos)
	}, coreadapt.Options{})
	fixed := NewFixed(converted, newFixer(t), coreadapt.Options{})
	results := make(map[celler.ChunkPos]*converter.JavaChunk)
	for range 2 {
		for _, pos := range []celler.ChunkPos{{}, {X: -1}, {X: 1}, {Z: -1}, {Z: 1}} {
			got := getChunk(t, fixed, pos)
			if before := results[pos]; before != nil && before != got {
				t.Fatal("修复结果未复用")
			}
			results[pos] = got
		}
		if reads != 21 {
			t.Fatalf("reads=%d", reads)
		}
	}
}
