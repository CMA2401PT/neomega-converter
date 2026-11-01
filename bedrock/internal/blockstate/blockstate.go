package blockstate

import (
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
)

func ToJava(source ir.IRBlock) (ir.IRBlock, bool) {
	// Java 转换表接收目标 NEMC 状态，旧名称、旧属性和缺省值要先对齐。
	bedrock, ok := converts.ConvertFromBedrock(source)
	if !ok {
		return bedrock, false
	}
	converted, ok := converts.ConvertToJava(bedrock)
	return converted.Tidy(), ok
}
