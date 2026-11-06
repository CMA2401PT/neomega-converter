package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	cellerjava "celler/java"
	"celler/java/region"
	"celler/layer_storage"
	"neomega-blocks/ir"
	coreexport "neomega-converter-core/be2je/export"
	nbt "neomega-nbt/presets/v1"
)

// TestRoundTripWorld 走完整条链路：Java 世界 → Bedrock 世界 → Java 世界。
func TestRoundTripWorld(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "java-source")
	source, err := cellerjava.OpenDirectory(sourceDir, cellerjava.Options{})
	if err != nil {
		t.Fatal(err)
	}
	chunk := cellerjava.NewChunk(celler.ChunkPos{}, 4189)
	blocks := layer_storage.MakePaletteStorage(ir.IRBlock(`minecraft:chest ["facing"="north","type"="single","waterlogged"="false"]`))
	if err = chunk.SetSection(&cellerjava.Section{Y: 0, Blocks: blocks}); err != nil {
		t.Fatal(err)
	}
	chunk.NBT.Value["block_entities"] = []nbt.Compound{{
		"id": "minecraft:chest", "x": int32(0), "y": int32(0), "z": int32(0),
		"Items": []nbt.Compound{{"id": "minecraft:diamond_sword", "count": int32(1), "Slot": int8(0)}},
	}}
	if err = source.WriteChunk(cellerjava.Overworld, chunk); err != nil {
		t.Fatal(err)
	}
	entityDocument := nbt.Document{Value: nbt.Compound{
		"DataVersion": int32(4189), "Position": nbt.Array([]int32{0, 0}),
		"Entities": []nbt.Compound{{"id": "minecraft:zombie", "Pos": []float64{0, 65, 0}}},
	}}
	data, err := nbt.EncodeDocument(entityDocument, nbt.EncodingBigEndian)
	if err != nil {
		t.Fatal(err)
	}
	record, err := region.Compress(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = source.WriteRaw(cellerjava.Overworld, cellerjava.Entities, celler.ChunkPos{}, record); err != nil {
		t.Fatal(err)
	}
	if err = source.Close(); err != nil {
		t.Fatal(err)
	}

	bedrockDir := filepath.Join(root, "bedrock")
	var output bytes.Buffer
	if err = run([]string{"-input", sourceDir, "-output", bedrockDir}, &output); err != nil {
		t.Fatal(err)
	}

	bedrockWorld, err := cellerbedrock.Open(bedrockDir, cellerbedrock.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer bedrockWorld.Close()
	targetDir := filepath.Join(root, "java-target")
	target, err := cellerjava.OpenDirectory(targetDir, cellerjava.Options{})
	if err != nil {
		t.Fatal(err)
	}
	report, err := coreexport.ConvertWorld(bedrockWorld, target, coreexport.ConvertOptions{Dimensions: []int32{0}})
	if err != nil {
		t.Fatal(err)
	}
	if err = target.Close(); err != nil {
		t.Fatal(err)
	}
	if report.Chunks[0] != 1 {
		t.Fatalf("report: %+v", report)
	}

	reader, err := cellerjava.OpenDirectory(targetDir, cellerjava.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	back, err := reader.ReadChunk(cellerjava.Overworld, celler.ChunkPos{})
	if err != nil {
		t.Fatal(err)
	}
	section, err := back.Section(0)
	if err != nil {
		t.Fatal(err)
	}
	if state := section.Blocks.Get(layer_storage.At(0, 0, 0)); state.NameWithNameSpace() != "minecraft:chest" {
		t.Fatalf("block: %s", state)
	}
	entities, _, err := back.BlockEntities()
	if err != nil || len(entities) != 1 || entities[0].NBT["id"] != "minecraft:chest" {
		t.Fatalf("block entities: %#v, %v", entities, err)
	}
	items, _ := nbt.Compounds(entities[0].NBT["Items"])
	if len(items) != 1 || items[0]["id"] != "minecraft:diamond_sword" {
		t.Fatalf("items: %#v", entities[0].NBT["Items"])
	}
	actors, err := reader.ReadChunkEntities(cellerjava.Overworld, celler.ChunkPos{})
	if err != nil || len(actors) != 1 || actors[0]["id"] != "minecraft:zombie" {
		t.Fatalf("actors: %#v, %v", actors, err)
	}
}
