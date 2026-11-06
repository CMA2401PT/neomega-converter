package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"

	cellerbedrock "celler/bedrock"
	cellerjava "celler/java"
	"neomega-converter-core"
	"neomega-converter-core/export"
	postprocess "neomega-converter/export"
	"neomega-viewer-core/fixer/java-postprocess"
)

func main() {
	// 错误集中交给进程入口处理，转换流程可以被测试直接调用。
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) (err error) {
	// LevelDB 即使只读打开也可能写辅助文件，说明文字要求用户对副本操作。
	flags := flag.NewFlagSet("bedrock-to-java", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.Usage = func() {
		fmt.Fprintln(output, "Bedrock → Java 1.21.4。LevelDB 打开时可能写文件，请对 Bedrock 存档副本运行。")
		fmt.Fprintln(output, "转换：bedrock-to-java -input <Bedrock 世界> -output <新建或空的 Java 世界> [-dims 0,1,2] [-all-chunks] [-fix] [-strict]")
		fmt.Fprintln(output, "仅修复：bedrock-to-java -fix-only -output <已有 Java 世界> [-dims 0,1,2]")
		fmt.Fprintln(output, "-fix-only 与 -input、-fix 互斥；只修复指定维度，不读取 Bedrock 或改写 level.dat。")
		flags.PrintDefaults()
	}
	input := flags.String("input", "", "Bedrock 世界目录（请使用副本）")
	targetPath := flags.String("output", "", "Java 世界目录：转换时须新建或为空；-fix-only 时须已存在")
	dimensionText := flags.String("dims", "0", "Bedrock 维度，逗号分隔：0,1,2")
	fix := flags.Bool("fix", false, "写出后独立读取 Java 世界并修复邻接状态")
	fixOnly := flags.Bool("fix-only", false, "只修复已有 Java 世界的邻接状态")
	strict := flags.Bool("strict", false, "遇到第一根转换失败的柱子立即返回")
	memory := flags.Int("memory-mib", 512, "转换工作内存预算（MiB，至少 64；不是进程内存硬上限）")
	workers := flags.Int("workers", 0, "转换并发任务数上限，0 按 CPU 和内存预算自动选择")
	allChunks := flags.Bool("all-chunks", false, "也转换未生成完成的柱子；默认跳过")
	if err = flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return err
	}
	if *memory < 64 || *workers < 0 {
		return fmt.Errorf("转换内存预算至少 64 MiB，workers 不能为负数")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if *fixOnly {
		// 按参数是否出现判断互斥，显式传入 -fix=false 或空 -input 也不能混用模式。
		flags.Visit(func(flag *flag.Flag) {
			if flag.Name == "input" || flag.Name == "fix" {
				err = fmt.Errorf("-fix-only 不能与 -input 或 -fix 同时使用")
			}
		})
		if err != nil {
			return err
		}
		if *targetPath == "" {
			flags.Usage()
			return fmt.Errorf("-fix-only 必须指定 -output")
		}
	} else if *input == "" || *targetPath == "" {
		flags.Usage()
		return fmt.Errorf("必须指定 -input 和 -output")
	}
	dimensions, err := parseDimensions(*dimensionText)
	if err != nil {
		return err
	}
	if *fixOnly {
		return runFixOnly(ctx, *targetPath, dimensions, output)
	}
	if err = prepareOutput(*targetPath); err != nil {
		return err
	}
	source, err := cellerbedrock.Open(*input, cellerbedrock.Options{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, source.Close()) }()
	target, err := cellerjava.OpenDirectory(*targetPath, cellerjava.Options{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, target.Close()) }()
	level, err := source.ReadLevel()
	// 只有 db 的源存档仍可导出，缺少元数据时统一走字段默认值并报告。
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	document, levelReport, err := export.ConvertLevel(level, export.DataVersion)
	if err != nil {
		return err
	}
	if err = target.WriteLevel(document); err != nil {
		return err
	}
	report, err := export.ConvertWorld(
		source,
		target,
		export.ConvertOptions{
			Dimensions:        dimensions,
			Strict:            *strict,
			IncludeUnfinished: *allChunks,
			Context:           ctx,
			MemoryMiB:         *memory,
			Workers:           *workers,
		},
	)
	printReport(output, report, levelReport)
	if err != nil {
		return err
	}
	if *fix {
		if err = fixWorld(ctx, target, dimensions, output); err != nil {
			return err
		}
	}
	return target.Commit()
}

func runFixOnly(ctx context.Context, directory string, dimensions []int32, output io.Writer) (err error) {
	// 写模式打开会自动创建目录，先确认目标存在，避免路径输错后生成一个空世界。
	if _, err = os.Stat(directory); err != nil {
		return err
	}
	target, err := cellerjava.OpenDirectory(directory, cellerjava.Options{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, target.Close()) }()
	if err = fixWorld(ctx, target, dimensions, output); err != nil {
		return err
	}
	return target.Commit()
}

func fixWorld(ctx context.Context, target *cellerjava.World, dimensions []int32, output io.Writer) error {
	// 两种修复入口共用编号映射与报告，修复始终只依赖 Java 世界中的数据。
	f, err := fixer.New()
	if err != nil {
		return err
	}
	javaDims := make([]cellerjava.Dimension, 0, len(dimensions))
	for _, dim := range dimensions {
		mapped, _ := export.JavaDimension(dim)
		javaDims = append(javaDims, mapped)
	}
	fixed, err := postprocess.FixWorld(
		target,
		f,
		postprocess.FixOptions{Dimensions: javaDims, Context: ctx},
	)
	fmt.Fprintf(output, "fix.chunks=%d\nfix.changed_sections=%d\n", fixed.Chunks, fixed.ChangedSections)
	return err
}

func parseDimensions(text string) ([]int32, error) {
	// 命令行只接受本次导出支持的原版维度，避免输错编号后静默生成空世界。
	var dimensions []int32
	seen := make(map[int32]bool)
	for _, part := range strings.Split(text, ",") {
		value, err := strconv.ParseInt(strings.TrimSpace(part), 10, 32)
		if err != nil {
			return nil, err
		}
		dim := int32(value)
		if _, ok := export.JavaDimension(dim); !ok {
			return nil, fmt.Errorf("不支持的维度 %d", dim)
		}
		if !seen[dim] {
			dimensions = append(dimensions, dim)
			seen[dim] = true
		}
	}
	return dimensions, nil
}

func prepareOutput(directory string) error {
	// 非空目录拒绝写入，防止把已有存档与新导出的 region 混在一起。
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(directory, 0755)
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("输出目录已存在且非空：%s", directory)
	}
	return nil
}

func printReport(output io.Writer, report export.Report, level export.LevelReport) {
	// 稳定排序方便比较开关修复前后的统计，所有计数各占一行。
	dims := make([]int, 0, len(report.Chunks))
	for dim := range report.Chunks {
		dims = append(dims, int(dim))
	}
	sort.Ints(dims)
	for _, dim := range dims {
		fmt.Fprintf(output, "convert.dimension.%d.chunks=%d\n", dim, report.Chunks[int32(dim)])
		fmt.Fprintf(output, "convert.dimension.%d.dropped_sections=%d\n", dim, report.DroppedSections[int32(dim)])
	}
	fmt.Fprintf(output, "convert.failed_chunks=%d\n", len(report.FailedChunks))
	fmt.Fprintf(output, "convert.skipped_chunks=%d\n", report.Skipped)
	fmt.Fprintf(output, "convert.block_entities=%d\n", report.BlockEntities)
	fmt.Fprintf(output, "convert.entities=%d\n", report.Entities)
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
	for _, failure := range report.FailedChunks {
		fmt.Fprintf(output, "failed dimension=%d x=%d z=%d: %v\n", failure.Dimension, failure.Pos.X, failure.Pos.Z, failure.Err)
	}
	for _, state := range report.UnknownStates {
		fmt.Fprintf(output, "unknown %s=%d\n", state.State, state.Count)
	}
	for _, item := range report.UnknownItems {
		fmt.Fprintf(output, "unknown_item %s=%d\n", item.Item, item.Count)
	}
	ids := make([]string, 0, len(report.UnsupportedBlockEntities))
	for id := range report.UnsupportedBlockEntities {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := report.UnsupportedBlockEntities[ids[i]], report.UnsupportedBlockEntities[ids[j]]
		if a != b {
			return a > b
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids[:min(10, len(ids))] {
		fmt.Fprintf(output, "unsupported_block_entity %s=%d\n", id, report.UnsupportedBlockEntities[id])
	}
	fmt.Fprintf(output, "level.defaulted_fields=%d\n", len(level.DefaultedFields))
	for _, field := range level.DefaultedFields {
		fmt.Fprintf(output, "level.defaulted.%s=1\n", field)
	}
}
