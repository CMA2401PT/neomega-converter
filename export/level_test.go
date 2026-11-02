package export

import (
	"reflect"
	"testing"

	cellerbedrock "celler/bedrock"
	nbt "neomega-nbt/presets/v1"
)

func TestConvertLevel(t *testing.T) {
	level := cellerbedrock.Level{NBT: nbt.Document{Value: nbt.Compound{
		"LevelName": "测试", "SpawnX": int32(-12), "SpawnY": int32(100), "SpawnZ": int32(33),
		"RandomSeed": int64(-9876543210), "GameType": int32(1), "Difficulty": int32(3), "Time": int64(12345), "commandsAllowed": int8(1),
	}}}
	document, report, err := ConvertLevel(level, DataVersion)
	if err != nil || len(report.DefaultedFields) != 0 {
		t.Fatalf("report=%v err=%v", report, err)
	}
	world := openJava(t)
	if err = world.WriteLevel(document); err != nil {
		t.Fatal(err)
	}
	got, err := world.ReadLevel()
	if err != nil {
		t.Fatal(err)
	}
	data := got.Value["Data"].(nbt.Compound)
	for key, want := range (nbt.Compound{
		"LevelName": "测试", "SpawnX": int32(-12), "SpawnY": int32(100), "SpawnZ": int32(33),
		"GameType": int32(1), "Difficulty": int8(3), "Time": int64(12345), "DayTime": int64(12345), "allowCommands": int8(1),
		"DataVersion": DataVersion, "version": int32(19133), "initialized": int8(1),
		"Version":   nbt.Compound{"Id": DataVersion, "Name": "1.21.4", "Series": "main", "Snapshot": int8(0)},
		"DataPacks": nbt.Compound{"Enabled": []string{"vanilla"}, "Disabled": []string{}},
	}) {
		if !reflect.DeepEqual(data[key], want) {
			t.Errorf("%s=%#v want=%#v", key, data[key], want)
		}
	}
	settings := data["WorldGenSettings"].(nbt.Compound)
	if settings["seed"] != int64(-9876543210) || settings["generate_features"] != int8(1) || settings["bonus_chest"] != int8(0) {
		t.Fatal(settings)
	}
	dimensions := settings["dimensions"].(nbt.Compound)
	for _, test := range []struct{ key, kind, noise, preset string }{
		{"minecraft:overworld", "minecraft:overworld", "minecraft:overworld", "minecraft:overworld"},
		{"minecraft:the_nether", "minecraft:the_nether", "minecraft:nether", "minecraft:nether"},
		{"minecraft:the_end", "minecraft:the_end", "minecraft:end", ""},
	} {
		dim := dimensions[test.key].(nbt.Compound)
		generator := dim["generator"].(nbt.Compound)
		biome := generator["biome_source"].(nbt.Compound)
		if dim["type"] != test.kind || generator["settings"] != test.noise || generator["type"] != "minecraft:noise" {
			t.Fatal(dim)
		}
		if test.preset == "" {
			if biome["type"] != "minecraft:the_end" {
				t.Fatal(biome)
			}
		} else if biome["type"] != "minecraft:multi_noise" || biome["preset"] != test.preset {
			t.Fatal(biome)
		}
	}
}

func TestConvertLevelDefaults(t *testing.T) {
	document, report, err := ConvertLevel(cellerbedrock.Level{NBT: nbt.Document{Value: nbt.Compound{"SpawnX": "bad"}}}, DataVersion)
	if err != nil {
		t.Fatal(err)
	}
	wantFields := []string{"LevelName", "SpawnX", "SpawnY", "SpawnZ", "RandomSeed", "GameType", "Difficulty", "Time", "commandsAllowed"}
	if !reflect.DeepEqual(report.DefaultedFields, wantFields) {
		t.Fatal(report)
	}
	data := document.Value["Data"].(nbt.Compound)
	for key, want := range (nbt.Compound{"LevelName": "New World", "SpawnX": int32(0), "SpawnY": int32(64), "SpawnZ": int32(0), "GameType": int32(0), "Difficulty": int8(2), "Time": int64(0), "allowCommands": int8(0)}) {
		if data[key] != want {
			t.Errorf("%s=%v want=%v", key, data[key], want)
		}
	}
	if data["WorldGenSettings"].(nbt.Compound)["seed"] != int64(0) {
		t.Fatal("seed default")
	}
}
