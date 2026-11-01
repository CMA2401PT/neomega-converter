package blockstate

import (
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
)

func ToJava(source ir.IRBlock) (ir.IRBlock, bool) {
	// 单状态调用与区块缓存共用转换路径，避免两处的版本对齐行为分歧。
	converted, _, ok := ToJavaWithSource(source)
	return converted, ok
}

func ToJavaWithSource(source ir.IRBlock) (converted ir.IRBlock, bedrock ir.IRBlock, ok bool) {
	// Java 转换表接收目标 NEMC 状态，旧名称、旧属性和缺省值要先对齐。
	bedrock, ok = converts.ConvertFromBedrock(source)
	if !ok {
		return bedrock, bedrock, false
	}
	converted, ok = converts.ConvertToJava(bedrock)
	return converted.Tidy(), bedrock, ok
}
