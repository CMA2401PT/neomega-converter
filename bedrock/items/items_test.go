package items

import (
	"testing"

	nbt "neomega-nbt/presets/v1"
)

func TestConvertItem(t *testing.T) {
	for _, test := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "log",
			src:  `{Name:"minecraft:oak_log",Count:12b,Slot:3b}`,
			want: `{id:"minecraft:oak_log",count:12,Slot:3b}`,
		},
		{
			name: "damage and enchantments",
			src:  `{Name:"minecraft:oak_log",Count:1b,Damage:9s,tag:{Damage:4,ench:[{id:17s,lvl:3s},{id:40s,lvl:2s}]}}`,
			want: `{id:"minecraft:oak_log",count:1,components:{"minecraft:damage":4,"minecraft:enchantments":{levels:{"minecraft:unbreaking":3,"minecraft:breach":2}}}}`,
		},
		{
			name: "top level short damage",
			src:  `{Name:"minecraft:oak_log",Count:1b,Damage:7s}`,
			want: `{id:"minecraft:oak_log",count:1,components:{"minecraft:damage":7}}`,
		},
		{
			name: "tag short damage",
			src:  `{Name:"minecraft:oak_log",Count:1b,Damage:7,tag:{Damage:2s}}`,
			want: `{id:"minecraft:oak_log",count:1,components:{"minecraft:damage":2}}`,
		},
		{
			name: "zero tag damage overrides aux",
			src:  `{Name:"minecraft:oak_log",Count:1b,Damage:7s,tag:{Damage:0}}`,
			want: `{id:"minecraft:oak_log",count:1}`,
		},
		{
			name: "water potion",
			src:  `{Name:"minecraft:potion",Count:1b,Damage:0s}`,
			want: `{id:"minecraft:potion",count:1,components:{"minecraft:potion_contents":{potion:"minecraft:water"}}}`,
		},
		{
			name: "splash potion aux is not damage",
			src:  `{Name:"minecraft:splash_potion",Count:2b,Damage:21s,tag:{Damage:9}}`,
			want: `{id:"minecraft:splash_potion",count:2,components:{"minecraft:potion_contents":{potion:"minecraft:healing"}}}`,
		},
		{
			name: "lingering final potion",
			src:  `{Name:"minecraft:lingering_potion",Count:1b,Damage:46}`,
			want: `{id:"minecraft:lingering_potion",count:1,components:{"minecraft:potion_contents":{potion:"minecraft:infested"}}}`,
		},
		{
			name: "tipped arrow offset",
			src:  `{Name:"minecraft:arrow",Count:8b,Slot:0b,Damage:22s}`,
			want: `{id:"minecraft:tipped_arrow",count:8,Slot:0b,components:{"minecraft:potion_contents":{potion:"minecraft:healing"}}}`,
		},
		{
			name: "written book",
			src:  `{Name:"minecraft:written_book",Count:1b,tag:{title:"标题",author:"作者",generation:2s,pages:[{text:"§a第一页"},{text:""}]}}`,
			want: `{id:"minecraft:written_book",count:1,components:{"minecraft:written_book_content":{title:{raw:"标题"},author:"作者",generation:2,pages:[{raw:'{"text":"§a第一页"}'},{raw:'{"text":""}'}],resolved:1b}}}`,
		},
		{
			name: "writable book",
			src:  `{Name:"minecraft:writable_book",Count:1b,tag:{pages:[{text:"§a草稿"}]}}`,
			want: `{id:"minecraft:writable_book",count:1,components:{"minecraft:writable_book_content":{pages:[{raw:"§a草稿"}]}}}`,
		},
		{
			name: "empty writable book",
			src:  `{Name:"minecraft:writable_book",Count:1b}`,
			want: `{id:"minecraft:writable_book",count:1,components:{"minecraft:writable_book_content":{pages:[]}}}`,
		},
		{name: "tool", src: `{Name:"minecraft:diamond_sword",Count:1b}`},
		{name: "zero count", src: `{Name:"minecraft:oak_log",Count:0b}`},
		{name: "negative count", src: `{Name:"minecraft:oak_log",Count:-1b}`},
		{name: "air", src: `{Name:"minecraft:air",Count:1b}`},
		{name: "empty name", src: `{Name:"",Count:1b}`},
		{name: "decay", src: `{Name:"minecraft:potion",Count:1b,Damage:36s}`},
		{name: "negative potion", src: `{Name:"minecraft:potion",Count:1b,Damage:-1s}`},
		{name: "out of range potion", src: `{Name:"minecraft:potion",Count:1b,Damage:47s}`},
		{name: "decay arrow", src: `{Name:"minecraft:arrow",Count:1b,Damage:37s}`},
		{name: "out of range arrow", src: `{Name:"minecraft:arrow",Count:1b,Damage:48s}`},
		{name: "ordinary arrow", src: `{Name:"minecraft:arrow",Count:1b,Damage:0s}`},
		{name: "nested container", src: `{Name:"minecraft:shulker_box",Count:1b,tag:{Items:[{Name:"minecraft:oak_log",Count:1b}]}}`},
		{name: "unknown enchantment", src: `{Name:"minecraft:oak_log",Count:1b,tag:{ench:[{id:17s,lvl:3s},{id:41s,lvl:1s}]}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := decode(t, test.src)
			got := ConvertItem(src)
			if test.want == "" {
				if got != nil {
					t.Fatalf("expected skipped item, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("supported item was skipped")
			}
			encoded, err := nbt.EncodeDocument(nbt.Document{Value: got}, nbt.EncodingBigEndian)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := nbt.DecodeDocument(encoded, nbt.EncodingBigEndian)
			if err != nil {
				t.Fatal(err)
			}
			want := decode(t, test.want)
			// SNBT 比较忽略空列表的元素类型，同时保留数值宽度。
			if snbt(t, decoded.Value) != snbt(t, want) {
				t.Fatalf("got %#v, want %#v", decoded.Value, want)
			}
		})
	}
}

func TestBookMultiline(t *testing.T) {
	text := "§a首行\n第二行\n第三行\n第四行\n第五行\n"
	for _, name := range []string{"written_book", "writable_book"} {
		t.Run(name, func(t *testing.T) {
			got := ConvertItem(nbt.Compound{
				"Name": "minecraft:" + name, "Count": int8(1),
				"tag": nbt.Compound{"pages": []nbt.Compound{{"text": text}}},
			})
			comps := got["components"].(nbt.Compound)
			content := comps["minecraft:"+name+"_content"].(nbt.Compound)
			pages, _ := nbt.Compounds(content["pages"])
			want := text
			if name == "written_book" {
				want = `{"text":"","extra":[{"text":"§a首行"},{"text":"\n"},{"text":"第二行"},{"text":"\n"},{"text":"第三行"},{"text":"\n"},{"text":"第四行"},{"text":"\n"},{"text":"第五行"},{"text":"\n"},{"text":""}]}`
			}
			if len(pages) != 1 || pages[0]["raw"] != want {
				t.Fatalf("unexpected pages: %v", pages)
			}
		})
	}
}

func decode(t *testing.T, text string) nbt.Compound {
	t.Helper()
	// 实际输入由二进制 NBT 解码，compound 列表具有确定的元素类型。
	encoded, err := nbt.SNBTToNBT(text, nbt.OptSNBTToNBT{Encoding: nbt.EncodingLittleEndian})
	if err != nil {
		t.Fatal(err)
	}
	document, err := nbt.DecodeDocument(encoded, nbt.EncodingLittleEndian)
	if err != nil {
		t.Fatal(err)
	}
	return document.Value
}

func snbt(t *testing.T, value nbt.Compound) string {
	t.Helper()
	text, err := nbt.EncodeToSNBT(value, nbt.OptEncodeToSNBT{SortMapKey: true})
	if err != nil {
		t.Fatal(err)
	}
	return text
}
