// Package report 打印两个方向整图转换共有的统计行。
// 两个命令行工具输出同一种格式，比较往返转换前后的结果时可以直接逐行对照。
package report

import (
	"fmt"
	"io"

	"neomega-converter-core"
)

// Failures 每个失败柱子一行，带维度和坐标。
func Failures(output io.Writer, failures []converter.FailedChunk) {
	fmt.Fprintf(output, "convert.failed_chunks=%d\n", len(failures))
	for _, failure := range failures {
		fmt.Fprintf(output, "failed dimension=%d x=%d z=%d: %v\n", failure.Dimension, failure.Pos.X, failure.Pos.Z, failure.Err)
	}
}

// Diagnostics 先按种类打印全部计数（含零），再各列影响最大的十项未知状态、物品和方块实体。
func Diagnostics(output io.Writer, summary converter.DiagnosticSummary) {
	for _, kind := range []converter.DiagnosticKind{
		converter.UnknownBlock,
		converter.NBTError,
		converter.RuleError,
		converter.UnsupportedBlockEntity,
		converter.UnknownEntity,
		converter.UnknownItem,
	} {
		fmt.Fprintf(output, "convert.diagnostic.%s=%d\n", kind, summary.ByKind[kind])
	}
	for _, state := range converter.TopCounts(summary.UnknownStates, 10) {
		fmt.Fprintf(output, "unknown %s=%d\n", state.Key, state.Count)
	}
	for _, item := range converter.TopCounts(summary.UnknownItems, 10) {
		fmt.Fprintf(output, "unknown_item %s=%d\n", item.Key, item.Count)
	}
	for _, entity := range converter.TopCounts(summary.UnsupportedBlockEntities, 10) {
		fmt.Fprintf(output, "unsupported_block_entity %s=%d\n", entity.Key, entity.Count)
	}
}
