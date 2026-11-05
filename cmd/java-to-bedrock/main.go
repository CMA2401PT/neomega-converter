package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"

	cellerbedrock "celler/bedrock"
	cellerjava "celler/java"
	"neomega-converter-core"
	javaactors "neomega-converter-core/je2be/actors"
	bedrockexport "neomega-converter-core/je2be/export"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("java-to-bedrock", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.Usage = func() {
		fmt.Fprintln(output, "Java → Bedrock 1.21.50。输出目录必须不存在或为空。")
		fmt.Fprintln(output, "java-to-bedrock -input <Java 世界> -output <新建或空的 Bedrock 世界> [-dims 0,1,2] [-strict]")
		flags.PrintDefaults()
	}
	input := flags.String("input", "", "Java 世界目录")
	targetPath := flags.String("output", "", "Bedrock 世界目录：必须不存在或为空")
	dimensionText := flags.String("dims", "0", "维度编号，逗号分隔：0,1,2")
	strict := flags.Bool("strict", false, "遇到第一根失败的柱子立即返回")
	entityDefs := flags.String("entity-defs", "", "可选：mcconv 格式的 bedrock_entities.json")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return err
	}
	if *input == "" || *targetPath == "" {
		flags.Usage()
		return fmt.Errorf("必须指定 -input 和 -output")
	}
	dimensions, err := parseDimensions(*dimensionText)
	if err != nil {
		return err
	}
	if *entityDefs != "" {
		if err := javaactors.LoadEntityDefs(*entityDefs); err != nil {
			return err
		}
	}
	if err := prepareOutput(*targetPath); err != nil {
		return err
	}
	source, err := cellerjava.OpenDirectory(*input, cellerjava.Options{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, source.Close()) }()
	level, err := source.ReadLevel()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	convertedLevel, levelReport := bedrockexport.ConvertLevel(level)
	target, err := cellerbedrock.Open(*targetPath, cellerbedrock.Options{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, target.Close()) }()
	if err = target.WriteLevel(convertedLevel); err != nil {
		return err
	}
	report, err := bedrockexport.ConvertWorld(source, target, bedrockexport.ConvertOptions{
		Dimensions: dimensions,
		Strict:     *strict,
	})
	printReport(output, report, levelReport)
	return err
}

func parseDimensions(text string) ([]int32, error) {
	seen := make(map[int32]bool)
	var dimensions []int32
	for _, part := range strings.Split(text, ",") {
		value, err := strconv.ParseInt(strings.TrimSpace(part), 10, 32)
		if err != nil {
			return nil, err
		}
		dimension := int32(value)
		if dimension < 0 || dimension > 2 {
			return nil, fmt.Errorf("不支持的维度 %d", dimension)
		}
		if !seen[dimension] {
			seen[dimension] = true
			dimensions = append(dimensions, dimension)
		}
	}
	return dimensions, nil
}

func prepareOutput(directory string) error {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(directory, 0o755)
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("输出目录已存在且非空：%s", directory)
	}
	return nil
}

func printReport(output io.Writer, report bedrockexport.Report, level bedrockexport.LevelReport) {
	dimensions := make([]int, 0, len(report.Chunks))
	for dimension := range report.Chunks {
		dimensions = append(dimensions, int(dimension))
	}
	sort.Ints(dimensions)
	for _, dimension := range dimensions {
		fmt.Fprintf(output, "convert.dimension.%d.chunks=%d\n", dimension, report.Chunks[int32(dimension)])
	}
	fmt.Fprintf(output, "convert.entities=%d\n", report.Entities)
	fmt.Fprintf(output, "convert.failed_chunks=%d\n", len(report.FailedChunks))
	for _, failure := range report.FailedChunks {
		fmt.Fprintf(output, "failed dimension=%d x=%d z=%d: %v\n", failure.Dimension, failure.Pos.X, failure.Pos.Z, failure.Err)
	}
	for _, kind := range []converter.DiagnosticKind{
		converter.UnknownBlock,
		converter.NBTError,
		converter.RuleError,
		converter.UnsupportedBlockEntity,
		converter.UnknownEntity,
		converter.UnknownItem,
	} {
		fmt.Fprintf(output, "convert.diagnostic.%s=%d\n", kind, report.Diagnostics[kind])
	}
	fmt.Fprintf(output, "level.defaulted_fields=%d\n", len(level.Defaulted))
}
