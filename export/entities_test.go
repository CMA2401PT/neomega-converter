package export

import (
	"reflect"
	"testing"

	"celler"
	"celler/java"
	"celler/java/region"
	"neomega-converter/bedrock"
	"neomega-converter/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestToJavaEntitiesRoundTrip(t *testing.T) {
	source := converted(celler.ChunkPos{X: 2, Z: -3}, 0)
	if ToJavaEntities(source, DataVersion) != nil {
		t.Fatal("empty entities produced a record")
	}
	frame, ok, err := bedrock.ConvertBlockEntity("ItemFrame", blockentities.Input{
		Base:        "minecraft:air []",
		SourceState: `minecraft:frame ["facing_direction"=3]`,
		WorldPos:    celler.BlockPos{X: 32, Y: 1, Z: -48},
		Source:      nbt.Compound{},
	})
	if err != nil || !ok {
		t.Fatalf("frame: %v %v", ok, err)
	}
	source.Entities = []nbt.Compound{frame.Entity}
	document := ToJavaEntities(source, DataVersion)
	data, err := nbt.EncodeDocument(*document, nbt.EncodingBigEndian)
	if err != nil {
		t.Fatal(err)
	}
	record, err := region.Compress(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	world := openJava(t)
	if err = world.WriteRaw(java.Overworld, java.Entities, source.Pos, record); err != nil {
		t.Fatal(err)
	}
	if err = world.Flush(); err != nil {
		t.Fatal(err)
	}
	raw, err := world.ReadRaw(java.Overworld, java.Entities, source.Pos)
	if err != nil {
		t.Fatal(err)
	}
	data, err = raw.Decode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := nbt.DecodeDocument(data, nbt.EncodingBigEndian)
	if err != nil || !reflect.DeepEqual(got, *document) {
		t.Fatalf("entities=%v err=%v", got, err)
	}
}
