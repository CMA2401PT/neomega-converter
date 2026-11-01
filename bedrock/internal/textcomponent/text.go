package textcomponent

import (
	"encoding/json"
	"strings"
)

type component struct {
	Text  string      `json:"text"`
	Extra []component `json:"extra,omitempty"`
}

// Section 保留原文中的格式码，各行共用简单文本组件表示。
func Section(text string) string {
	lines := strings.Split(text, "\n")
	value := component{Text: lines[0]}
	if len(lines) > 1 {
		// 书页和名称没有四行限制，换行也需要保留在同一个组件里。
		value = component{}
		for i, line := range lines {
			if i > 0 {
				value.Extra = append(value.Extra, component{Text: "\n"})
			}
			value.Extra = append(value.Extra, component{Text: line})
		}
	}
	// 数据只含字符串和自行构造的树，JSON 编码不会失败。
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
