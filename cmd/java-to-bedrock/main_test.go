package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	cellerjava "celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
)

func TestRun(t *testing.T) {
	root := t.TempDir()
	input, output := filepath.Join(root, "java"), filepath.Join(root, "bedrock")
	source, err := cellerjava.OpenDirectory(input, cellerjava.Options{})
	if err != nil {
		t.Fatal(err)
	}
	chunk := cellerjava.NewChunk(celler.ChunkPos{}, 4189)
	blocks := layer_storage.MakePaletteStorage(ir.IRBlock(`minecraft:chest ["facing"="north","type"="single","waterlogged"="false"]`))
	if err = chunk.SetSection(&cellerjava.Section{Y: 0, Blocks: blocks}); err != nil {
		t.Fatal(err)
	}
	if err = source.WriteChunk(cellerjava.Overworld, chunk); err != nil {
		t.Fatal(err)
	}
	if err = source.Close(); err != nil {
		t.Fatal(err)
	}

	var buffer bytes.Buffer
	if err = run([]string{"-input", input, "-output", output}, &buffer); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buffer.String(), "convert.dimension.0.chunks=1\n") {
		t.Fatalf("report: %s", buffer.String())
	}
	if _, err = os.Stat(filepath.Join(output, "level.dat")); err != nil {
		t.Fatalf("level.dat: %v", err)
	}
	world, err := cellerbedrock.Open(output, cellerbedrock.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer world.Close()
	if _, err = world.ReadChunk(0, celler.ChunkPos{}); err != nil {
		t.Fatalf("chunk: %v", err)
	}
}
