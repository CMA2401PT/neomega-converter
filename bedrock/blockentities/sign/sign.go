package sign

import (
	"fmt"
	"strings"

	"neomega-converter/bedrock/blockentities"
	"neomega-converter/bedrock/internal/textcomponent"
	nbt "neomega-nbt/presets/v1"
)

const (
	ID        = "Sign"
	HangingID = "HangingSign"
)

func Convert(input blockentities.Input) (blockentities.Result, error) {
	front := input.Source
	if raw, exists := input.Source["FrontText"]; exists {
		var ok bool
		front, ok = raw.(nbt.Compound)
		if !ok {
			return blockentities.Result{}, fmt.Errorf("sign FrontText must be a compound")
		}
	}
	back, _ := input.Source["BackText"].(nbt.Compound)
	id := "minecraft:sign"
	if strings.HasSuffix(input.Base.NameWithNameSpace(), "_hanging_sign") {
		id = "minecraft:hanging_sign"
	}
	waxed, _ := input.Source["IsWaxed"].(int8)
	entity := nbt.Compound{
		"id":         id,
		"front_text": side(front),
		"back_text":  side(back),
		"is_waxed":   flag(waxed),
	}
	return blockentities.Result{State: input.Base, BlockEntity: entity}, nil
}

func side(source nbt.Compound) nbt.Compound {
	text, _ := source["Text"].(string)
	lines := strings.Split(text, "\n")
	messages := make([]string, 4)
	for i := range messages {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		messages[i] = textcomponent.Section(line)
	}
	color := "black"
	if argb, ok := source["SignTextColor"].(int32); ok {
		color = nearestDye(argb & 0xFFFFFF)
	}
	glow, _ := source["IgnoreLighting"].(int8)
	return nbt.Compound{
		"messages":         messages,
		"color":            color,
		"has_glowing_text": flag(glow),
	}
}

var dyes = []struct {
	name string
	rgb  int32
}{
	{"white", 0xF9FFFE}, {"orange", 0xF9801D}, {"magenta", 0xC74EBD}, {"light_blue", 0x3AB3DA},
	{"yellow", 0xFED83D}, {"lime", 0x80C71F}, {"pink", 0xF38BAA}, {"gray", 0x474F52},
	{"light_gray", 0x9D9D97}, {"cyan", 0x169C9C}, {"purple", 0x8932B8}, {"blue", 0x3C44AA},
	{"brown", 0x835432}, {"green", 0x5E7C16}, {"red", 0xB02E26}, {"black", 0x1D1D21},
}

func nearestDye(rgb int32) string {
	name := "black"
	best := int32(3*255*255 + 1)
	for _, dye := range dyes {
		r := (rgb>>16)&255 - (dye.rgb>>16)&255
		g := (rgb>>8)&255 - (dye.rgb>>8)&255
		b := rgb&255 - dye.rgb&255
		// 平方距离保留同样的大小关系，无需开方。
		distance := r*r + g*g + b*b
		if distance < best {
			best, name = distance, dye.name
		}
	}
	return name
}

func flag(value int8) int8 {
	if value != 0 {
		return 1
	}
	return 0
}
