// structure-to-world 将结构文件写成与输入数据最接近的 Java 或基岩版存档。
package main

import (
	"context"
	"flag"
	"fmt"
	"neomega-converter-core/structure"
	"os"
	"os/signal"
)

func main() {
	input := flag.String("input", "", "结构文件路径")
	output := flag.String("output", "", "输出目录；默认在结构文件旁建立同名文件夹")
	flag.Parse()
	if *input == "" {
		fmt.Fprintln(os.Stderr, "需要 -input 指定结构文件")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	result, err := structure.ConvertFile(ctx, *input, *output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if result.Edition == "java" {
		fmt.Printf("%s · %s · %d chunks\n", result.Edition, result.Directory, result.Chunks)
	} else {
		fmt.Printf("%s · %s\n", result.Edition, result.Directory)
	}
}
