package fixer

import "testing"

func TestSnowyGround(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, ground := range []string{"grass_block", "mycelium", "podzol"} {
		for _, test := range []struct {
			above, input, want string
		}{
			{above: "snow", input: "false", want: "true"},
			{above: "snow_block", input: "false", want: "true"},
			{above: "powder_snow", input: "false", want: "true"},
			{above: "air", input: "true", want: "false"},
			{above: "stone", input: "true", want: "false"},
		} {
			t.Run(ground+"/"+test.above, func(t *testing.T) {
				chunk := airChunk(t)
				chunk[cell(8, 8, 8)] = stateHash(t, ground, map[string]string{"snowy": test.input})
				chunk[cell(8, 9, 8)] = stateHash(t, test.above, nil)
				got := f.Fix(chunk, Neighbours{})
				assertHash(t, got[cell(8, 8, 8)], stateHash(t, ground, map[string]string{"snowy": test.want}))
			})
		}
	}
}

func TestSnowyGroundAcrossSubchunkBoundary(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	chunk := airChunk(t)
	chunk[cell(4, 15, 4)] = stateHash(t, "grass_block", map[string]string{"snowy": "false"})
	up := airChunk(t)
	up[cell(4, 0, 4)] = stateHash(t, "snow", nil)
	got := f.Fix(chunk, Neighbours{3: up})
	assertHash(t, got[cell(4, 15, 4)], stateHash(t, "grass_block", map[string]string{"snowy": "true"}))
	// 上方子区块缺失时按空气处理，snowy 回到 false。
	got = f.Fix(chunk, Neighbours{})
	assertHash(t, got[cell(4, 15, 4)], stateHash(t, "grass_block", map[string]string{"snowy": "false"}))
}
