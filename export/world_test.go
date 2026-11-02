package export

import (
	"errors"
	"io/fs"
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter"
	nbt "neomega-nbt/presets/v1"
)

func openBedrock(t *testing.T) *cellerbedrock.World {
	t.Helper()
	world, err := cellerbedrock.Open(t.TempDir(), cellerbedrock.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := world.Close(); err != nil {
			t.Error(err)
		}
	})
	return world
}

func sourceChunk(t *testing.T, pos celler.ChunkPos, dim int32, ys ...int32) *cellerbedrock.Chunk {
	t.Helper()
	chunk := cellerbedrock.NewChunk(pos, dim)
	for _, y := range ys {
		layer := cellerbedrock.NewLayer(cellerbedrock.Block{State: "minecraft:stone []", Version: 18168865})
		if err := chunk.SetSection(&cellerbedrock.Section{Y: y, Version: 9, Layers: []*cellerbedrock.Layer{layer}}); err != nil {
			t.Fatal(err)
		}
	}
	return chunk
}

func TestConvertWorld(t *testing.T) {
	source, target := openBedrock(t), openJava(t)
	chunk := sourceChunk(t, celler.ChunkPos{}, 0, 0, 20)
	section, err := chunk.Section(0)
	if err != nil {
		t.Fatal(err)
	}
	for index, state := range []ir.IRBlock{"minecraft:chest []", `minecraft:frame ["facing_direction"=3]`, "minecraft:furnace []", "minecraft:not_a_block []"} {
		section.Layers[0].Blocks.Set(layer_storage.At(index, 0, 0), cellerbedrock.Block{State: state.Tidy(), Version: 18168865})
	}
	if err = chunk.SetSection(section); err != nil {
		t.Fatal(err)
	}
	if err = chunk.SetBlockEntities([]nbt.Document{
		{Value: nbt.Compound{"id": "Chest", "x": int32(0), "y": int32(0), "z": int32(0), "Items": []nbt.Compound{}}},
		{Value: nbt.Compound{"id": "ItemFrame", "x": int32(1), "y": int32(0), "z": int32(0)}},
		{Value: nbt.Compound{"id": "Furnace", "x": int32(2), "y": int32(0), "z": int32(0)}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*cellerbedrock.Chunk{chunk, sourceChunk(t, celler.ChunkPos{}, 1, -1, 0, 16), sourceChunk(t, celler.ChunkPos{}, 2, 0)} {
		if err = source.WriteChunk(c, nil); err != nil {
			t.Fatal(err)
		}
	}
	report, err := ConvertWorld(source, target, ConvertOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Chunks[0] != 1 || len(report.Chunks) != 1 || report.DroppedSections[0] != 1 || report.Entities != 1 || report.BlockEntities != 1 {
		t.Fatalf("report=%+v", report)
	}
	if report.Diagnostics[converter.UnknownBlock] != 1 || report.Diagnostics[converter.UnsupportedBlockEntity] != 1 || report.UnsupportedBlockEntities["Furnace"] != 1 || len(report.UnknownStates) != 1 || report.UnknownStates[0].Count != 1 {
		t.Fatalf("diagnostics=%+v", report)
	}
	got, err := target.ReadChunk(java.Overworld, celler.ChunkPos{})
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := got.Section(0)
	if err != nil {
		t.Fatal(err)
	}
	if blocks.Blocks.Get(layer_storage.At(3, 0, 0)) != "minecraft:air []" {
		t.Fatal("unknown not air")
	}
	entities, issues, err := got.BlockEntities()
	if err != nil || len(issues) != 0 || len(entities) != 1 {
		t.Fatalf("entities=%v issues=%v err=%v", entities, issues, err)
	}
	if _, err = target.ReadRaw(java.Overworld, java.Entities, celler.ChunkPos{}); err != nil {
		t.Fatal(err)
	}
	if _, err = target.ReadChunk(java.Nether, celler.ChunkPos{}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unselected dimension: %v", err)
	}
	all := openJava(t)
	report, err = ConvertWorld(source, all, ConvertOptions{Dimensions: []int32{0, 1, 2}})
	if err != nil || report.Chunks[1] != 1 || report.Chunks[2] != 1 || report.DroppedSections[1] != 2 {
		t.Fatalf("all=%+v err=%v", report, err)
	}
	if _, err = all.ReadRaw(java.Nether, java.Entities, celler.ChunkPos{}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("empty entity record: %v", err)
	}
}

func TestConvertWorldStrict(t *testing.T) {
	source := openBedrock(t)
	for x := range int32(3) {
		chunk := sourceChunk(t, celler.ChunkPos{X: x}, 0, 0)
		if x == 1 {
			chunk.Records[string([]byte{cellerbedrock.TagSubChunk, 0})] = []byte{99}
		}
		if err := source.WriteChunk(chunk, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, strict := range []bool{false, true} {
		target := openJava(t)
		report, err := ConvertWorld(source, target, ConvertOptions{Strict: strict})
		if (err != nil) != strict || len(report.FailedChunks) != 1 || report.FailedChunks[0].Pos.X != 1 || report.FailedChunks[0].Err == nil {
			t.Fatalf("strict=%v report=%+v err=%v", strict, report, err)
		}
		want := uint64(2)
		if strict {
			want = 1
		}
		if report.Chunks[0] != want {
			t.Fatal(report)
		}
		if _, err = target.ReadChunk(java.Overworld, celler.ChunkPos{}); err != nil {
			t.Fatal(err)
		}
		if _, err = target.ReadChunk(java.Overworld, celler.ChunkPos{X: 1}); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("bad chunk written: %v", err)
		}
		// 另开读句柄，确认包括严格模式提前退出在内的所有返回路径均已 Flush。
		reader, err := java.Open(target.Files, java.Options{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		_, err = reader.ReadChunk(java.Overworld, celler.ChunkPos{})
		closeErr := reader.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("flush: %v %v", err, closeErr)
		}
	}
}
