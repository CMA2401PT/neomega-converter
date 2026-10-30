package fixer

import (
	"slices"

	"neomega-assets/solidtable"
)

type JavaBlockHash = uint32

type Neighbours [6][]JavaBlockHash

// 顺序为 west/east/down/up/north/south（-X/+X/-Y/+Y/-Z/+Z）；nil 表示该侧为空。
// 每片含 4096 格，索引为 (x<<8)|(z<<4)|y，以便直接接收 mesher 的输入布局。

type pass func(subchunk []JavaBlockHash, neighbours Neighbours) map[int]JavaBlockHash

// pass 只读取当前层和邻居并返回改动；统一应用后，下一轮才能看到完整结果。

type Fixer struct {
	solid  solidtable.Table
	passes []pass
}

func New() (*Fixer, error) {
	solid, err := solidtable.Bundled()
	if err != nil {
		return nil, err
	}
	f := &Fixer{solid: solid}
	f.passes = []pass{f.connections, f.fireConnections, f.stairShapes, f.doorHalves, f.snowyGround, f.redstoneConnections}
	return f, nil
}

func (f *Fixer) Fix(subchunk []JavaBlockHash, neighbours Neighbours) []JavaBlockHash {
	// 复制一次供各轮累积，调用者仍可保留原始输入。
	result := slices.Clone(subchunk)
	for _, run := range f.passes {
		for index, hash := range run(result, neighbours) {
			result[index] = hash
		}
	}
	return result
}

func neighbour(subchunk []JavaBlockHash, neighbours Neighbours, index, side int) (JavaBlockHash, bool) {
	position := [3]int{index >> 8, index & 15, (index >> 4) & 15}
	axis := side / 2
	position[axis] += 2*(side%2) - 1
	if position[axis] < 0 || position[axis] >= 16 {
		subchunk = neighbours[side]
		if subchunk == nil {
			return 0, false
		}
		position[axis] &= 15
	}
	return subchunk[(position[0]<<8)|(position[2]<<4)|position[1]], true
}
