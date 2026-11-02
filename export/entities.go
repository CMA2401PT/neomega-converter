package export

import (
	"neomega-converter"
	nbt "neomega-nbt/presets/v1"
)

func ToJavaEntities(chunk *converter.ConvertedChunk, dataVersion int32) *nbt.Document {
	// 实体独立于地形存储；空指针让调用方直接跳过没有实体的 region 记录。
	if len(chunk.Entities) == 0 {
		return nil
	}
	return &nbt.Document{Value: nbt.Compound{
		"DataVersion": dataVersion,
		"Position":    nbt.Array([]int32{chunk.Pos.X, chunk.Pos.Z}),
		"Entities":    chunk.Entities,
	}}
}
