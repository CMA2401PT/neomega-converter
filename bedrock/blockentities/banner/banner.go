package banner

import (
	"fmt"
	"strings"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
)

const ID string = "Banner"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	// 旗帜底色使用 Bedrock 的染料编号，不能沿用基础映射的颜色。
	colors := [...]string{
		"black", "red", "green", "brown", "blue", "purple", "cyan", "light_gray",
		"gray", "pink", "lime", "yellow", "light_blue", "magenta", "orange", "white",
	}
	base, ok := input.Source["Base"].(int32)
	if !ok || base < 0 || int(base) >= len(colors) {
		return blockentities.Result{}, fmt.Errorf("banner Base must be an Int in 0..15")
	}
	suffix := "_banner"
	if strings.Contains(input.Base.NameWithNameSpace(), "wall") {
		suffix = "_wall_banner"
	}
	state := ir.IRBlock("minecraft:" + colors[base] + suffix + " " + string(input.Base.States())).Tidy()
	return blockentities.Result{State: state}, nil
}
