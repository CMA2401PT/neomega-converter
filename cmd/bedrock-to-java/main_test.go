package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	"neomega-converter/export"
	nbt "neomega-nbt/presets/v1"
)

func TestRun(t *testing.T) {
	input := filepath.Join(t.TempDir(), "bedrock")
	source, err := cellerbedrock.Open(input, cellerbedrock.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err = source.WriteLevel(cellerbedrock.Level{
		Version: 10,
		NBT:     nbt.Document{Value: nbt.Compound{"LevelName": "cli fixture"}},
	}); err != nil {
		t.Fatal(err)
	}
	chunk := cellerbedrock.NewChunk(celler.ChunkPos{}, 0)
	if err = chunk.SetSection(&cellerbedrock.Section{
		Y:       0,
		Version: 9,
		Layers: []*cellerbedrock.Layer{cellerbedrock.NewLayer(cellerbedrock.Block{
			State:   "minecraft:stone []",
			Version: 18168865,
		})},
	}); err != nil {
		t.Fatal(err)
	}
	if err = source.WriteChunk(chunk, nil); err != nil {
		t.Fatal(err)
	}
	if err = source.Close(); err != nil {
		t.Fatal(err)
	}
	for _, fix := range []bool{false, true} {
		target := filepath.Join(t.TempDir(), "java")
		args := []string{"-input", input, "-output", target}
		if fix {
			args = append(args, "-fix")
		}
		var output bytes.Buffer
		if err = run(args, &output); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(output.String(), "convert.dimension.0.chunks=1\n") || strings.Contains(output.String(), "fix.chunks=1\n") != fix {
			t.Fatal(output.String())
		}
		world, err := java.OpenDirectory(target, java.Options{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		level, err := world.ReadLevel()
		if err != nil {
			t.Fatal(err)
		}
		if level.Value["Data"].(nbt.Compound)["DataVersion"] != export.DataVersion {
			t.Fatal(level)
		}
		if _, err = world.ReadChunk(java.Overworld, celler.ChunkPos{}); err != nil {
			t.Fatal(err)
		}
		if err = world.Close(); err != nil {
			t.Fatal(err)
		}
		// 第二次运行必须在打开输入世界前拒绝覆盖，原有文件应保持原值。
		sentinel := filepath.Join(target, "keep")
		if err = os.WriteFile(sentinel, []byte("keep"), 0644); err != nil {
			t.Fatal(err)
		}
		if err = run(args, &output); err == nil {
			t.Fatal("nonempty output accepted")
		}
		data, err := os.ReadFile(sentinel)
		if err != nil || string(data) != "keep" {
			t.Fatalf("sentinel=%s err=%v", data, err)
		}
	}

	if err = os.Remove(filepath.Join(input, "level.dat")); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = run([]string{"-input", input, "-output", filepath.Join(t.TempDir(), "java")}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "level.defaulted_fields=9\n") {
		t.Fatal(output.String())
	}
}

func TestRunFixOnly(t *testing.T) {
	// 直接构造 Java 世界，保证独立修复不依赖第一步的输入或元数据。
	for _, test := range []struct {
		name      string
		dims      string
		selected  map[java.Dimension]bool
		withLevel bool
	}{
		{
			name:      "default overworld",
			selected:  map[java.Dimension]bool{java.Overworld: true},
			withLevel: true,
		},
		{
			name:     "nether and end without level",
			dims:     "1,2",
			selected: map[java.Dimension]bool{java.Nether: true, java.End: true},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			world, err := java.OpenDirectory(directory, java.Options{})
			if err != nil {
				t.Fatal(err)
			}
			fence, ok := converts.JavaCanon("minecraft:oak_fence []")
			if !ok {
				t.Fatal("missing fence state")
			}
			for _, dim := range []java.Dimension{java.Overworld, java.Nether, java.End} {
				blocks := layer_storage.MakePaletteStorage(ir.IRBlock("minecraft:air []"))
				blocks.Set(layer_storage.At(5, 5, 5), fence)
				blocks.Set(layer_storage.At(6, 5, 5), fence)
				chunk := java.NewChunk(celler.ChunkPos{}, export.DataVersion)
				if err = chunk.SetSection(&java.Section{Y: 0, Blocks: blocks}); err != nil {
					t.Fatal(err)
				}
				if err = world.WriteChunk(dim, chunk); err != nil {
					t.Fatal(err)
				}
			}
			if err = world.Close(); err != nil {
				t.Fatal(err)
			}
			levelPath := filepath.Join(directory, "level.dat")
			levelBytes := []byte("level.dat must not be read or rewritten")
			if test.withLevel {
				if err = os.WriteFile(levelPath, levelBytes, 0644); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-fix-only", "-output", directory}
			if test.dims != "" {
				args = append(args, "-dims", test.dims)
			}
			var output bytes.Buffer
			if err = run(args, &output); err != nil {
				t.Fatal(err)
			}
			wantReport := fmt.Sprintf("fix.chunks=%d\nfix.changed_sections=%d\n", len(test.selected), len(test.selected))
			if output.String() != wantReport {
				t.Fatalf("report=%q, want %q", output.String(), wantReport)
			}
			data, err := os.ReadFile(levelPath)
			if test.withLevel {
				if err != nil || !bytes.Equal(data, levelBytes) {
					t.Fatalf("level.dat changed: %q, %v", data, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("unexpected level.dat: %v", err)
			}
			reader, err := java.OpenDirectory(directory, java.Options{ReadOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := reader.Close(); err != nil {
					t.Error(err)
				}
			})
			for _, dim := range []java.Dimension{java.Overworld, java.Nether, java.End} {
				chunk, err := reader.ReadChunk(dim, celler.ChunkPos{})
				if err != nil {
					t.Fatal(err)
				}
				section, err := chunk.Section(0)
				if err != nil {
					t.Fatal(err)
				}
				connected := section.Blocks.Get(layer_storage.At(5, 5, 5)).States().CheckKey("east", "true")
				if connected != test.selected[dim] {
					t.Fatalf("dimension %s: connected=%v", dim, connected)
				}
			}
		})
	}
}

func TestRunFixOnlyArguments(t *testing.T) {
	for _, conflict := range [][]string{
		{"-input", "bedrock"},
		{"-input", ""},
		{"-fix"},
		{"-fix=false"},
	} {
		args := append([]string{"-fix-only", "-output", filepath.Join(t.TempDir(), "missing")}, conflict...)
		var output bytes.Buffer
		if err := run(args, &output); err == nil || !strings.Contains(err.Error(), "不能与 -input 或 -fix 同时使用") {
			t.Fatalf("args=%v: %v", args, err)
		}
	}
	var output bytes.Buffer
	if err := run([]string{"-fix-only"}, &output); err == nil || !strings.Contains(err.Error(), "必须指定 -output") {
		t.Fatalf("missing output: %v", err)
	}
	directory := filepath.Join(t.TempDir(), "missing")
	if err := run([]string{"-fix-only", "-output", directory}, &output); !os.IsNotExist(err) {
		t.Fatalf("missing world: %v", err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("created missing world: %v", err)
	}
}

func TestHelpFixOnly(t *testing.T) {
	var output bytes.Buffer
	if err := run([]string{"-h"}, &output); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		"-fix-only -output <已有 Java 世界> [-dims 0,1,2]",
		"-fix-only 与 -input、-fix 互斥",
		"不读取 Bedrock 或改写 level.dat",
	} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("help missing %q: %s", text, output.String())
		}
	}
}
