package adapt

import (
	"reflect"

	"neomega-converter"
)

func chunkBytes(chunk *converter.ConvertedChunk) int64 {
	// 预算覆盖调色板、NBT 和诊断，并给 map/LRU 预留开销；共享对象在每层独立计费。
	size := int64(256)
	if chunk == nil {
		return size
	}
	for _, section := range chunk.Sections {
		size += 128
		if blocks := section.Blocks; blocks != nil {
			size += int64(cap(blocks.Data))*4 + int64(cap(blocks.Palette))*16
			for _, state := range blocks.Palette {
				size += int64(len(state))
			}
		}
		if section.Biomes != nil {
			size += 64 * 16
			for _, name := range section.Biomes {
				size += int64(len(name))
			}
		}
	}
	size += int64(cap(chunk.BlockEntities)) * 32
	for _, entity := range chunk.BlockEntities {
		size += valueBytes(reflect.ValueOf(entity.NBT))
	}
	size += int64(cap(chunk.Entities)) * 8
	for _, entity := range chunk.Entities {
		size += valueBytes(reflect.ValueOf(entity))
	}
	size += int64(cap(chunk.Diagnostics)) * 128
	for _, diagnostic := range chunk.Diagnostics {
		size += int64(len(diagnostic.State) + len(diagnostic.Kind) + len(diagnostic.BlockEntityID))
		if diagnostic.Err != nil {
			size += int64(len(diagnostic.Err.Error()))
		}
	}
	return size
}

func valueBytes(value reflect.Value) int64 {
	// NBT 列表与定长数组都保留 Go 类型，递归估算可涵盖物品组件中的嵌套 compound。
	if !value.IsValid() {
		return 0
	}
	size := int64(value.Type().Size())
	switch value.Kind() {
	case reflect.Interface:
		size += valueBytes(value.Elem())
	case reflect.String:
		size += int64(value.Len())
	case reflect.Map:
		size += 48
		entries := value.MapRange()
		for entries.Next() {
			size += 16 + valueBytes(entries.Key()) + valueBytes(entries.Value())
		}
	case reflect.Slice, reflect.Array:
		elementBytes := int64(value.Type().Elem().Size())
		if value.Kind() == reflect.Slice {
			size += int64(value.Cap()) * elementBytes
		}
		for i := 0; i < value.Len(); i++ {
			size += valueBytes(value.Index(i)) - elementBytes
		}
	}
	return size
}
