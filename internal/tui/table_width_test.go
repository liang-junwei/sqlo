package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/liang-junwei/sqlo/internal/output"
)

// 本文件锁定「宽表列被压扁」这次修复（2026-09-04）。
//
// 原 bug：computeWidths 在总宽超出终端时把所有列等比压缩到刚好塞进终端宽，
// 于是 17 列每列只剩 1~4 字符；更隐蔽的是，因为所有列都「放得下」，
// renderWindow 的横向窗口永不裁剪，←/→ 横滚彻底失效，屏幕上只剩被
// runewidth.Truncate 截成碎渣的内容。
//
// 修复后：列宽 = 内容自然宽（clamp 到 minColW..maxColW），与终端宽无关；
// 放不下的列由 renderWindow 按列裁剪，←/→ 平移窗口查看。

// 宽表夹具：17 列，对应真实业务表（UUID 主键 + 若干状态列 + 两个时间戳）。
var (
	wideHeaders = []string{
		"id", "org_id", "dept_id", "parent_id", "name", "code", "type",
		"leader_id", "phone", "email", "status", "sort", "remark",
		"created_at", "created_by", "updated_at", "updated_by",
	}
	wideRows = [][]interface{}{
		{
			"9b61300be1b64d2fa0c8b21c2f6e5a11", "ORG001", "DEPT001", "ROOT", "研发中心",
			"RD", "1", "U1001", "13800138000", "a@b.com", "1", "10", "备注文本",
			"2026-09-04 10:11:12", "admin", "2026-09-04 10:11:12", "admin",
		},
		{
			"7a12300be1b64d2fa0c8b21c2f6e5a22", "ORG001", "DEPT002", "DEPT001", "测试部",
			"QA", "2", "U1002", "13900139000", "c@d.com", "1", "20", "",
			"2026-09-04 10:12:12", "admin", "2026-09-04 10:12:12", "admin",
		},
	}

	// 期望的自然列宽：id 是 32 字符 UUID（顶到 maxColW），created_at/updated_at
	// 是 19 字符时间戳，其余按内容宽度。中文按显示宽度计（"研发中心" = 8）。
	wantWideWidths = []int{
		32, 6, 7, 9, 8, 4, 4, 9, 11, 7, 6, 4, 8, 19, 10, 19, 10,
	}
)

// newWidthModel 构造一个只装载列/行数据的 Model，用于验证窗口裁剪。
func newWidthModel(width int, cols []table.Column, rows []table.Row) *Model {
	return &Model{
		width:   width,
		height:  24,
		resCols: cols,
		resRows: rows,
	}
}

// wideTable 按 renderTableData 的同款逻辑把夹具转成表格列/行。
func wideTable() ([]table.Column, []table.Row) {
	widths := computeWidths(wideHeaders, wideRows)

	cols := make([]table.Column, len(wideHeaders))
	for i, h := range wideHeaders {
		cols[i] = table.Column{Title: h, Width: widths[i]}
	}

	rows := make([]table.Row, len(wideRows))
	for i, r := range wideRows {
		row := make(table.Row, len(r))
		for j, v := range r {
			row[j] = formatValue(v)
		}
		rows[i] = row
	}
	return cols, rows
}

// TestWideTableKeepsNaturalWidths 锁定回归：列宽由内容决定，不做「塞进终端」
// 的等比压缩。压缩一旦回来，这里的每一列都会变小（时间戳列尤其明显）。
func TestWideTableKeepsNaturalWidths(t *testing.T) {
	got := computeWidths(wideHeaders, wideRows)

	if len(got) != len(wantWideWidths) {
		t.Fatalf("列宽数量 = %d，期望 %d", len(got), len(wantWideWidths))
	}
	for i, want := range wantWideWidths {
		if got[i] != want {
			t.Errorf("列 %-11s 宽度 = %d，期望 %d", wideHeaders[i], got[i], want)
		}
	}

	// 总宽必须明显超过典型终端宽：这正是「没被压进终端」的直接证据，
	// 放不下应由 renderWindow 裁列，而不是把所有列一起压扁。
	total := 0
	for _, w := range got {
		total += w
	}
	if total <= 120 {
		t.Errorf("自然宽总和 = %d，应大于典型终端宽 120（说明发生了等比压缩）", total)
	}
}

// TestNarrowTableHugsContent 窄表按内容贴合，不被拉伸填充（曾试过「剩余宽度
// 平均分配」，结果 status 只有 1 个字符却占 32 格，已回退）。
func TestNarrowTableHugsContent(t *testing.T) {
	headers := []string{"id", "name", "status"}
	rows := [][]interface{}{{"1", "alice", "1"}, {"2", "bob", "0"}}

	got := computeWidths(headers, rows)
	want := []int{minColW, 5, 6} // name 最长值 alice=5，status 表头=6
	for i, w := range want {
		if got[i] != w {
			t.Errorf("列 %s 宽度 = %d，期望 %d", headers[i], got[i], w)
		}
	}
}

// TestRenderWindowClipsWideTable 锁定回归：宽表必须被横向窗口裁剪（否则所有列
// 挤在一屏，←/→ 横滚失效），且裁剪后的渲染行宽严格小于终端宽。
func TestRenderWindowClipsWideTable(t *testing.T) {
	const termWidth = 122
	cols, rows := wideTable()

	m := newWidthModel(termWidth, cols, rows)
	m.renderWindow()

	visible := len(m.table.Columns())
	if visible == 0 {
		t.Fatal("窗口未显示任何列")
	}
	if visible >= len(wideHeaders) {
		t.Errorf("宽表应被窗口裁剪：可见 %d 列 / 共 %d 列（未裁剪说明又把所有列压扁塞进去了）",
			visible, len(wideHeaders))
	}
	if m.table.Columns()[0].Title != "id" {
		t.Errorf("窗口首列 = %q，期望 id（平移前窗口应从第一列开始）", m.table.Columns()[0].Title)
	}

	// 满宽行拿不到行尾清理会折行，是幽灵行的成因之一。
	if w := lipgloss.Width(m.table.View()); w >= termWidth {
		t.Errorf("渲染行宽 = %d，必须 < 终端宽 %d", w, termWidth)
	}
}

// TestRenderWindowWidthHoldsAtEveryOffset 平移窗口到任意列，渲染行宽都必须
// 小于终端宽 —— 覆盖「尾部是超宽列」这类容易被忽略的位置。
func TestRenderWindowWidthHoldsAtEveryOffset(t *testing.T) {
	const termWidth = 122
	cols, rows := wideTable()

	for off := 0; off < len(cols); off++ {
		m := newWidthModel(termWidth, cols, rows)
		m.hOffset = off
		m.renderWindow()

		if w := lipgloss.Width(m.table.View()); w >= termWidth {
			t.Errorf("hOffset=%d 渲染行宽 = %d，必须 < 终端宽 %d", off, w, termWidth)
		}
	}
}

// TestRenderWindowClampsOversizedColumn 极窄终端下「至少显示一列」的兜底分支
// 会放行宽度超过可用宽度的单列，必须夹住。夹取值要扣掉 styleCell 的
// Padding(0,1)（列渲染宽 = Width + 2），否则正好占满终端宽导致折行。
func TestRenderWindowClampsOversizedColumn(t *testing.T) {
	const termWidth = 22
	m := newWidthModel(termWidth,
		[]table.Column{{Title: "id", Width: 32}},
		[]table.Row{{"9b61300be1b64d2fa0c8b21c2f6e5a11"}},
	)
	m.renderWindow()

	avail := termWidth - 2
	got := m.table.Columns()[0].Width
	if want := avail - 2; got != want {
		t.Errorf("超宽列被夹到 %d，期望 %d（可用宽 %d 扣除 Padding 2）", got, want, avail)
	}
	if w := lipgloss.Width(m.table.View()); w >= termWidth {
		t.Errorf("渲染行宽 = %d，必须 < 终端宽 %d", w, termWidth)
	}
	// 夹取不得污染共享列宽，否则横滚回来列宽就变了
	if m.resCols[0].Width != 32 {
		t.Errorf("夹取改写了 m.resCols 的列宽: %d，期望保持 32", m.resCols[0].Width)
	}
}

// TestSummarizeReportsTotalColumns 状态栏给出结果集的总列数（不是窗口可见列数），
// 用户据此判断右边还压着多少列、值不值得按 ←/→。
func TestSummarizeReportsTotalColumns(t *testing.T) {
	o := outcome{
		result:  &output.QueryResult{Columns: wideHeaders, Rows: wideRows},
		total:   len(wideRows),
		elapsed: 20 * time.Millisecond,
	}
	got := summarize(o)
	if !strings.Contains(got, "17 列") {
		t.Errorf("状态栏应报告总列数 17 列，实际: %s", got)
	}
	if !strings.Contains(got, "2 行") {
		t.Errorf("状态栏应报告行数 2 行，实际: %s", got)
	}

	// 非查询（无列）不应出现列数
	o2 := outcome{
		result:  &output.QueryResult{Message: "✓ 执行成功，影响 3 行"},
		elapsed: 8 * time.Millisecond,
	}
	if strings.Contains(summarize(o2), "列") {
		t.Errorf("非查询结果不应报告列数，实际: %s", summarize(o2))
	}
}
