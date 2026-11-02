package export

import (
	cellerbedrock "celler/bedrock"
	nbt "neomega-nbt/presets/v1"
)

type LevelReport struct {
	// 默认字段单独报告，避免将源世界缺失的信息伪装成成功保留。
	DefaultedFields []string
}

func ConvertLevel(level cellerbedrock.Level, dataVersion int32) (nbt.Document, LevelReport, error) {
	// 1.21.4 将世界生成配置放在 Data.WorldGenSettings，不能沿用 26.x 的独立文件布局。
	source := level.NBT.Value
	report := LevelReport{}
	data := nbt.Compound{
		"DataVersion": dataVersion, "version": int32(19133),
		"Version":     nbt.Compound{"Id": dataVersion, "Name": "1.21.4", "Series": "main", "Snapshot": int8(0)},
		"initialized": int8(1),
		"DataPacks":   nbt.Compound{"Enabled": []string{"vanilla"}, "Disabled": []string{}},
	}
	data["LevelName"] = levelValue(source, "LevelName", "New World", &report)
	data["SpawnX"] = levelValue(source, "SpawnX", int32(0), &report)
	data["SpawnY"] = levelValue(source, "SpawnY", int32(64), &report)
	data["SpawnZ"] = levelValue(source, "SpawnZ", int32(0), &report)
	seed := levelValue(source, "RandomSeed", int64(0), &report)
	data["GameType"] = levelValue(source, "GameType", int32(0), &report)
	// Bedrock 的 Difficulty 是 Int，Java 用 Byte；Time 同时作为世界总时间和昼夜时间。
	data["Difficulty"] = int8(levelValue(source, "Difficulty", int32(2), &report))
	time := levelValue(source, "Time", int64(0), &report)
	data["Time"], data["DayTime"] = time, time
	data["allowCommands"] = levelValue(source, "commandsAllowed", int8(0), &report)
	data["WorldGenSettings"] = nbt.Compound{
		"seed": seed, "generate_features": int8(1), "bonus_chest": int8(0),
		"dimensions": nbt.Compound{
			"minecraft:overworld":  noiseDimension("overworld", "overworld", "overworld"),
			"minecraft:the_nether": noiseDimension("the_nether", "nether", "nether"),
			"minecraft:the_end":    noiseDimension("the_end", "end", ""),
		},
	}
	return nbt.Document{Value: nbt.Compound{"Data": data}}, report, nil
}

func levelValue[T any](source nbt.Compound, key string, fallback T, report *LevelReport) T {
	// 类型不符也采用目标默认值，所有替代均从这条路径进入报告。
	if value, ok := source[key].(T); ok {
		return value
	}
	report.DefaultedFields = append(report.DefaultedFields, key)
	return fallback
}

func noiseDimension(kind, settings, preset string) nbt.Compound {
	// 三套原版生成器只引用内置注册表，继续生成新区块时不需要附带数据包。
	biome := nbt.Compound{"type": "minecraft:multi_noise", "preset": "minecraft:" + preset}
	if preset == "" {
		biome = nbt.Compound{"type": "minecraft:the_end"}
	}
	return nbt.Compound{
		"type":      "minecraft:" + kind,
		"generator": nbt.Compound{"type": "minecraft:noise", "settings": "minecraft:" + settings, "biome_source": biome},
	}
}
