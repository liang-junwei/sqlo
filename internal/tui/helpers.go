package tui

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// pinSingleConn 把连接池固定为单连接。
//
// database/sql 默认 MaxOpenConns 不限，而 USE db / SET search_path 这类切换
// 语句是【会话级】的，只对执行它的那条物理连接生效。池里一旦还有别的连接，
// 后续查询就可能落在没切过库的连接上，表现为「明明切了库，查到的还是旧数据」，
// 随机复现且极难排查。
//
// TUI 的核心价值就是会话态（临时表、未提交事务、会话变量），这些同样绑定在
// 单条物理连接上，本来就必须让所有语句走同一条连接。TUI 一次只执行一条语句，
// 固定单连接没有任何并发损失。
func pinSingleConn(conn *sql.DB) {
	if conn == nil {
		return
	}
	// MaxIdleConns 大于 MaxOpenConns 时会被自动降到同等值，无需另行设置。
	conn.SetMaxOpenConns(1)
}

// formatValue 把数据库值格式化为显示文本；压掉换行/制表，防止单元格破行
func formatValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	var s string
	switch val := v.(type) {
	case []byte:
		s = string(val)
	case time.Time:
		s = val.Format("2006-01-02 15:04:05")
	default:
		s = fmt.Sprintf("%v", v)
	}
	if strings.ContainsAny(s, "\n\r\t") {
		s = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(s)
	}
	return s
}

// truncate 与 _tuidemo/v3 相同的 rune 截断；额外把换行/制表压成空格
//（v3 的测试数据是单行文本，真实 SQL 结果里可能有换行值）。
func truncate(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return string(r[:w])
}

const (
	minColW = 4  // 列宽下限：内容再窄也留出可辨认的宽度，避免表头被压成一个字符
	maxColW = 32 // 列宽上限：单个超长值（长文本/JSON）最多占这么多，全值用 \x 扩展显示查看
)

// computeWidths 依据表头与内容计算每列的「自然宽度」，即
// clamp(max(表头宽, 内容最大宽), minColW, maxColW)。
//
// 这里刻意不做「等比压缩到能塞进终端宽」：宽表一旦被压缩，每列只剩一两个
// 字符，全部列都能塞下时 renderWindow 的横向窗口也永远不会裁剪，←/→ 横滚
// 随之失效，屏幕上只剩被 Truncate 截断的碎渣。所以放不下就交给 renderWindow
// 按列裁剪（←/→ 平移窗口查看），而不是把所有列一起压扁。
//
// 列宽因此与终端宽度完全无关：能否放得下由 renderWindow 判断。
func computeWidths(headers []string, rows [][]interface{}) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, r := range rows {
		for i, v := range r {
			if i >= len(widths) {
				break
			}
			if w := lipgloss.Width(formatValue(v)); w > widths[i] {
				widths[i] = w
			}
		}
	}

	for i := range widths {
		if widths[i] < minColW {
			widths[i] = minColW
		}
		if widths[i] > maxColW {
			widths[i] = maxColW
		}
	}

	return widths
}

// splitStatements 按分号拆分多条 SQL，跳过字符串与注释中的分号。
// 只做轻量扫描，不是完整的 SQL 解析器。
func splitStatements(text string) []string {
	var (
		stmts          []string
		buf            strings.Builder
		inSingle       bool
		inDouble       bool
		inLineComment  bool
		inBlockComment bool
	)

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}

		switch {
		case inLineComment:
			buf.WriteRune(c)
			if c == '\n' {
				inLineComment = false
			}
			continue

		case inBlockComment:
			buf.WriteRune(c)
			if c == '*' && next == '/' {
				buf.WriteRune(next)
				i++
				inBlockComment = false
			}
			continue

		case inSingle:
			buf.WriteRune(c)
			if c == '\\' && next != 0 {
				buf.WriteRune(next)
				i++
			} else if c == '\'' {
				inSingle = false
			}
			continue

		case inDouble:
			buf.WriteRune(c)
			if c == '"' {
				inDouble = false
			}
			continue
		}

		switch {
		case c == '-' && next == '-':
			inLineComment = true
			buf.WriteRune(c)
		case c == '/' && next == '*':
			inBlockComment = true
			buf.WriteRune(c)
		case c == '\'':
			inSingle = true
			buf.WriteRune(c)
		case c == '"':
			inDouble = true
			buf.WriteRune(c)
		case c == ';':
			if s := strings.TrimSpace(buf.String()); s != "" {
				stmts = append(stmts, s)
			}
			buf.Reset()
		default:
			buf.WriteRune(c)
		}
	}

	if s := strings.TrimSpace(buf.String()); s != "" {
		stmts = append(stmts, s)
	}

	return stmts
}
