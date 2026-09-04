package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"

	"github.com/liang-junwei/sqlo/internal/output"
)

// applyOutcome 把执行结果落到界面（v3 形态：结果到达即重建表格）
func (m *Model) applyOutcome(o outcome) {
	m.textLines = nil

	if o.result == nil {
		m.statusText = summarize(o)
		return
	}

	// 非查询消息（INSERT/UPDATE 等）
	if o.result.Message != "" && len(o.result.Columns) == 0 {
		m.textLines = strings.Split(o.result.Message, "\n")
		m.statusText = summarize(o)
		return
	}

	if len(o.result.Columns) > 0 {
		m.renderTableData(o.result)
	}

	m.statusText = summarize(o)
}

// renderTableData 把 QueryResult 转成表格数据。完整列/行数据保留在
// resCols/resRows 作为水平滚动的数据源；新结果集水平复位到最左，
// 再按终端宽截取可见列窗口重建表格。
func (m *Model) renderTableData(res *output.QueryResult) {
	widths := computeWidths(res.Columns, res.Rows)

	cols := make([]table.Column, len(res.Columns))
	for i, hd := range res.Columns {
		cols[i] = table.Column{Title: hd, Width: widths[i]}
	}

	rows := make([]table.Row, 0, len(res.Rows))
	for _, r := range res.Rows {
		row := make(table.Row, len(res.Columns))
		for i := range res.Columns {
			v := ""
			if i < len(r) {
				v = formatValue(r[i])
			}
			row[i] = v
		}
		rows = append(rows, row)
	}

	m.resCols = cols
	m.resRows = rows
	m.hOffset = 0
	m.renderWindow()
}

// renderWindow 按 [hOffset, 可见列数) 从完整数据里截取窗口重建表格。
// bubbles/table 没有原生水平滚动，这里以列为单位滑动窗口：终端放得下几列
// 就显示几列，←/→ 平移窗口后整体重建（与查询结果到达时的重建同构，
// 渲染层面无新差异）。列宽开销按每列 3 计：styleCell 的 Padding(0,1) 实际是
// 2，多留 1 格余量，避免渲染行正好占满终端宽（满宽行拿不到行尾清理会折行）。
func (m *Model) renderWindow() {
	n := len(m.resCols)
	if n == 0 {
		return
	}
	if m.hOffset >= n {
		m.hOffset = n - 1
	}
	if m.hOffset < 0 {
		m.hOffset = 0
	}

	avail := m.width - 2
	if avail < 10 {
		avail = 10
	}

	end := m.hOffset + 1 // 至少显示一列（首列独占超宽终端时允许被截断）
	sum := m.resCols[m.hOffset].Width + 3
	for end < n && sum+m.resCols[end].Width+3 <= avail {
		sum += m.resCols[end].Width + 3
		end++
	}

	cols := m.resCols[m.hOffset:end]

	// 上面的累加保证窗口总宽不超 avail，唯一的例外是「至少显示一列」的兜底：
	// 极窄终端下单列本身就可能超宽。这里把它夹住 —— bodyView 直接吐出
	// table.View() 不做逐行截断，越界行会破坏「每行宽 < 终端宽」的渲染纪律。
	// 夹取要扣掉 styleCell/styleHeader 的 Padding(0,1)：列渲染宽 = Width + 2。
	if len(cols) == 1 && cols[0].Width+2 > avail {
		c := cols[0] // 复制，避免改动 m.resCols 里的共享列宽
		c.Width = max(avail-2, minColW)
		cols = []table.Column{c}
	}

	rows := make([]table.Row, len(m.resRows))
	for i, r := range m.resRows {
		if end <= len(r) {
			rows[i] = r[m.hOffset:end]
		} else {
			row := make(table.Row, end-m.hOffset)
			copy(row, r[m.hOffset:])
			rows[i] = row
		}
	}

	m.renderTable(cols, rows)
}

// renderTable 与 _tuidemo/v3 的 renderTable 逐行一致：
// 尺寸在此刻按当前终端大小计算（WindowSizeMsg 不改表格尺寸）、
// table.New 整体重建、SetCursor 恢复导航位置。
func (m *Model) renderTable(cols []table.Column, rows []table.Row) {
	h := m.height - 9 // 扣除顶栏/状态/编辑器/帮助等占用
	if h < 3 {
		h = 3
	}
	w := m.width - 2
	if w < 10 {
		w = 10
	}

	m.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithHeight(h),
		table.WithWidth(w),
		table.WithFocused(m.focused),
		table.WithStyles(table.Styles{
			Header:   styleHeader,
			Cell:     styleCell,
			Selected: styleSelected,
		}),
	)
	m.table.SetCursor(m.cursor) // 恢复导航位置
}

func summarize(o outcome) string {
	parts := []string{fmt.Sprintf("%.2fs", o.elapsed.Seconds())}

	if o.result != nil && len(o.result.Columns) > 0 {
		rows := fmt.Sprintf("%d 行", len(o.result.Rows))
		if o.truncated {
			rows = fmt.Sprintf("显示 %d / 共 %d 行（已截断）", len(o.result.Rows), o.total)
		}
		// 列数取结果集总列数，不是横向窗口的可见列数：窗口只显示放得下的前几列，
		// 需要 ←/→ 平移，状态栏给总量才知道右边还压着多少列。
		parts = append([]string{rows, fmt.Sprintf("%d 列", len(o.result.Columns))}, parts...)
	}
	if o.result != nil && o.result.Message != "" {
		parts = append(parts, o.result.Message)
	}

	return strings.Join(parts, " · ")
}
