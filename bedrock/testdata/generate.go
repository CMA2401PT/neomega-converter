//go:build ignore

package main

import (
	"bytes"
	"celler"
	"celler/layer_storage"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/df-mc/goleveldb/leveldb"
	"github.com/df-mc/goleveldb/leveldb/opt"
	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
)

type cell struct {
	Position [3]int   `json:"position"`
	Layers   []string `json:"layers"`
	SNBT     string   `json:"snbt,omitempty"`
}

func main() {
	// 从数据库副本提取两层，保留源方块文本，便于直接核对植物和含水信息。
	dbPath := flag.String("db", "/Users/dai/.claude/jobs/5ce0302a/tmp/brdb", "Bedrock 数据库副本")
	output := flag.String("out", "bedrock/testdata/subchunk.json", "fixture 输出路径")
	flag.Parse()
	if err := generate(*dbPath, *output); err != nil {
		panic(err)
	}
}

func generate(dbPath, output string) error {
	db, err := leveldb.OpenFile(dbPath, &opt.Options{Compression: opt.FlateCompression})
	if err != nil {
		return err
	}
	defer db.Close()
	data, err := db.Get([]byte{0, 0, 0, 0, 0, 0, 0, 0, '/', 252}, nil)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(data)
	var header [3]byte
	if _, err := reader.Read(header[:]); err != nil {
		return err
	}
	layers := make([]*layer_storage.Palette[string], header[1])
	for i := range layers {
		layers[i], err = readLayer(reader)
		if err != nil {
			return err
		}
	}
	data, err = db.Get([]byte{0, 0, 0, 0, 0, 0, 0, 0, '1'}, nil)
	if err != nil {
		return err
	}
	entities := make(map[[3]int]string)
	reader = bytes.NewReader(data)
	for reader.Len() > 0 {
		var entity map[string]any
		if err := nbt.DecodeFromNBT(reader, &entity, nbt.OptDecodeFromNBT{Encoding: nbt.EncodingLittleEndian}); err != nil {
			return err
		}
		position := [3]int{int(entity["x"].(int32)), int(entity["y"].(int32)), int(entity["z"].(int32))}
		if position[1] < -64 || position[1] > -49 {
			continue
		}
		text, err := nbt.EncodeToSNBT(entity, nbt.OptEncodeToSNBT{SortMapKey: true})
		if err != nil {
			return err
		}
		entities[position] = text
	}
	cells := make([]cell, 0)
	for index := range 4096 {
		local := celler.SubChunkBlockPos{X: uint8(index >> 8), Y: uint8(index & 15), Z: uint8((index >> 4) & 15)}
		position := [3]int{int(local.X), int(local.Y) - 64, int(local.Z)}
		entry := cell{Position: position, SNBT: entities[position]}
		nonAir := false
		for _, layer := range layers {
			block := layer.Get(local)
			entry.Layers = append(entry.Layers, block)
			nonAir = nonAir || ir.IRBlock(block).ShortName() != "air"
		}
		if nonAir || entry.SNBT != "" {
			cells = append(cells, entry)
		}
	}
	var encoded bytes.Buffer
	encoded.WriteString("[\n")
	for index, entry := range cells {
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if index > 0 {
			encoded.WriteString(",\n")
		}
		encoded.WriteString("  ")
		encoded.Write(data)
	}
	encoded.WriteString("\n]\n")
	fmt.Printf("version=%d, layers=%d, subchunk=(0,-4,0), cells=%d\n", header[0], header[1], len(cells))
	return os.WriteFile(output, encoded.Bytes(), 0644)
}

func readLayer(reader *bytes.Reader) (*layer_storage.Palette[string], error) {
	// 沿用 celler 的调色板布局，逐层读取以保留第二层。
	header, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	layer := &layer_storage.Palette[string]{}
	layer.Fill(int(header>>1), "")
	if err := binary.Read(reader, binary.LittleEndian, layer.Data); err != nil {
		return nil, err
	}
	count := uint32(1)
	if layer.Level != 0 {
		if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
			return nil, err
		}
	}
	palette := make([]string, count)
	for i := range palette {
		var block struct {
			Name   string         `nbt:"name"`
			States map[string]any `nbt:"states"`
		}
		if err := nbt.DecodeFromNBT(reader, &block, nbt.OptDecodeFromNBT{Encoding: nbt.EncodingLittleEndian}); err != nil {
			return nil, err
		}
		value, err := ir.NewIRBlockFromGo(block.Name, block.States)
		if err != nil {
			return nil, err
		}
		palette[i] = string(value.Tidy())
	}
	layer.SetPalette(palette)
	return layer, nil
}
