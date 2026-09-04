package tui

import "github.com/charmbracelet/lipgloss"

// 样式与 _tuidemo/v3 一致——仅前景色/加粗，禁止 Background、Border
//（幽灵行排查结论）。与 v3 的唯一偏差：Cell/Header 加 Padding(0,1)——
// v3 的短测试数据靠列宽富余天然分隔，真实数据宽度=内容宽，无 Padding
// 时各列会连成一坨。Padding 参与 bubbles/table 的列对齐，是数据可读性
// 所需，不属于渲染管线差异。
var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("6")).
			Padding(0, 1)

	styleCell = lipgloss.NewStyle().
			Padding(0, 1)

	// 当前行：亮品红加粗。禁止 Background（幽灵行排查结论）；bubbles v1.0.0
	// 的 Selected 套在整行拼接结果上（Selected.Render(row)），只加 SGR
	// 不改行宽/对齐。品红与表头青色、正文默认色的色相差最大，暗底终端上
	// 是前景色里最醒目的选择。
	styleSelected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
)
