// Package tui 提供 sqlo 的交互式终端界面（REPL）。
//
// 渲染层是 _tuidemo/v3（用户实测滚动完全干净）的逐行移植，不是"参考"：
//   - 同样的事件流：WindowSizeMsg 只记录尺寸；表格尺寸只在查询结果到达
//     重建表格时设置（table.New + WithHeight/WithWidth）；
//   - 同样的 View：纯字符串拼接 5 段——顶栏 / 结果区 / 状态行 / 编辑器 /
//     帮助行，每段按「终端宽-1」做 rune 截断，无 JoinVertical、无边框、
//     无整行背景色、无左右对齐空格填充；
//   - 同样的样式：仅三个（表头青色加粗、单元格无样式、当前行黄色加粗）；
//   - 同样的按键流：表格按键直接应用，无任何延迟帧 / 重绘补丁。
//
// 与 v3 的差别只有两类：
//  1. simulateQuery 换成真实 SQL 执行（executor.go / meta.go，纯业务逻辑，
//     不参与渲染）；
//  2. 两个有意的功能新增：输入区 ↑/↓ 历史翻阅（单行输入时生效，多行编辑
//     仍移动光标）；当前行 Selected 加亮黄色前景（实测仅加粗不够醒目）。
package tui

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"github.com/liang-junwei/sqlo/internal/config"
	sqldb "github.com/liang-junwei/sqlo/internal/db"
	"github.com/liang-junwei/sqlo/internal/output"
)

// ConnInfo 顶栏展示的连接信息
type ConnInfo struct {
	Name     string
	Type     string
	Host     string
	Port     int
	Username string
	Database string
}

type runState int

const (
	stateIdle runState = iota
	stateRunning
)

// queryDoneMsg 一次执行完成后回传给 UI 的消息（对应 v3 的 queryDoneMsg，
// 携带 executor 的执行结果）
type queryDoneMsg struct {
	outcome
	label string
}

// Model TUI 根模型——字段与 v3 一一对应
type Model struct {
	conn *sql.DB
	info ConnInfo

	state  runState
	spin   spinner.Model
	editor textarea.Model

	table   table.Model
	focused bool // 结果表格是否聚焦（Tab 切换）
	cursor  int  // 重建表格时恢复的光标位置

	resCols []table.Column // 最近一次结果集的完整列（水平滚动的数据源）
	resRows []table.Row
	hOffset int // 水平滚动：最左可见列的下标

	width, height int
	statusText    string
	errText       string   // 执行错误（结果区第一行显示）
	textLines     []string // 纯文本结果（\? 帮助 / 非查询消息），不滚动

	history   []string // 已执行语句历史（输入区 ↑/↓ 翻阅，会话内）
	histIdx   int      // 当前翻阅位置；== len(history) 表示不在翻阅状态
	histDraft string   // 进入翻阅前编辑器里的未提交内容

	cancel context.CancelFunc
}

// Run 启动 TUI 会话。conn 由调用方持有生命周期，退出后由调用方关闭。
func Run(conn *sql.DB, info ConnInfo) error {
	if err := checkTerminal(); err != nil {
		return err
	}
	// 会话级语句（USE db / SET search_path）与会话态都必须落在同一条物理连接上
	pinSingleConn(conn)
	_, err := tea.NewProgram(newModel(conn, info), tea.WithAltScreen()).Run()
	return err
}

// checkTerminal 确认当前运行在交互式终端上。
//
// bubbletea 在 stdin 不是 TTY 时会自行另开一个输入 TTY 等待按键，
// 于是在 CI、管道、重定向这类非交互场景下程序不会收到 EOF 而永久挂住，
// 既没有输出也杀不掉。这里提前拦下，把场景导向 exec 子命令。
func checkTerminal() error {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return errors.New("tui 需要交互式终端（当前 stdin 是管道或重定向）\n批量执行请用: sqlo exec -e \"...\"")
	}
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return errors.New("tui 需要交互式终端（当前 stdout 被重定向）\n批量执行请用: sqlo exec -e \"...\"")
	}
	return nil
}

func newModel(conn *sql.DB, info ConnInfo) Model {
	ta := textarea.New()
	ta.Placeholder = "输入 SQL，Ctrl+X 执行"
	ta.Prompt = "› "
	ta.ShowLineNumbers = false
	ta.SetHeight(4)
	ta.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		conn:       conn,
		info:       info,
		editor:     ta,
		table:      table.New(table.WithFocused(false)),
		cursor:     0,
		spin:       sp,
		statusText: "就绪",
	}
}

func (m Model) Init() tea.Cmd { return m.spin.Tick }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// v3 行为：只记录尺寸，不调整任何组件。
		// 表格尺寸在下一次查询结果重建表格时才会按新尺寸设置。
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case spinner.TickMsg:
		if m.state != stateRunning {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case queryDoneMsg:
		m.state = stateIdle
		m.cancel = nil
		if msg.err != nil {
			m.errText = msg.err.Error()
			if msg.label != "" {
				m.errText = fmt.Sprintf("%s（语句: %s）", m.errText, truncate(msg.label, 60))
			}
			m.statusText = "执行出错"
			return m, nil
		}
		m.errText = ""
		m.applyOutcome(msg.outcome)
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			// 执行中先取消当前查询，空闲时才退出
			if m.state == stateRunning && m.cancel != nil {
				m.cancel()
				m.cancel = nil
				m.state = stateIdle
				m.statusText = "已取消"
				return m, nil
			}
			return m, tea.Quit
		case tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyCtrlX:
			return m.submit()
		case tea.KeyTab:
			m.focused = !m.focused
			if m.focused {
				m.editor.Blur()
				m.table.Focus()
			} else {
				m.editor.Focus()
				m.table.Blur()
			}
			return m, nil
		case tea.KeyLeft, tea.KeyRight:
			if m.focused {
				return m.hScroll(msg.Type == tea.KeyRight)
			}
			// 输入区聚焦：落到默认分支交给编辑器移动光标
		case tea.KeyUp:
			if m.editor.Focused() {
				// 单行输入：↑ 翻上一条历史；多行编辑中：交给编辑器移动光标
				if strings.Contains(m.editor.Value(), "\n") {
					break
				}
				return m.historyPrev()
			}
			if m.focused {
				var cmd tea.Cmd
				m.table, cmd = m.table.Update(msg)
				m.cursor = m.table.Cursor()
				return m, cmd
			}
			return m, nil
		case tea.KeyDown:
			if m.editor.Focused() {
				if strings.Contains(m.editor.Value(), "\n") {
					break
				}
				return m.historyNext()
			}
			if m.focused {
				var cmd tea.Cmd
				m.table, cmd = m.table.Update(msg)
				m.cursor = m.table.Cursor()
				return m, cmd
			}
			return m, nil
		}
	}

	// 其余按键交给编辑器
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}

// historyPrev 输入区 ↑：翻到上一条历史。首次翻阅时把当前未提交内容
// 存入草稿，↓ 翻回末尾时恢复。
func (m Model) historyPrev() (tea.Model, tea.Cmd) {
	if m.histIdx == len(m.history) {
		if len(m.history) == 0 {
			return m, nil
		}
		m.histDraft = m.editor.Value()
		m.histIdx = len(m.history) - 1
	} else if m.histIdx > 0 {
		m.histIdx--
	} else {
		return m, nil // 已在最旧一条
	}
	m.editor.SetValue(m.history[m.histIdx])
	m.editor.CursorEnd()
	return m, nil
}

// historyNext 输入区 ↓：翻到下一条；翻过末尾恢复进入翻阅前的草稿。
func (m Model) historyNext() (tea.Model, tea.Cmd) {
	if m.histIdx >= len(m.history) {
		return m, nil // 不在翻阅状态
	}
	m.histIdx++
	if m.histIdx == len(m.history) {
		m.editor.SetValue(m.histDraft)
		m.editor.CursorEnd()
		return m, nil
	}
	m.editor.SetValue(m.history[m.histIdx])
	m.editor.CursorEnd()
	return m, nil
}

// remember 把一条已提交的语句记入历史（连续重复只记一次），并复位翻阅状态。
func (m *Model) remember(text string) {
	if n := len(m.history); n > 0 && m.history[n-1] == text {
		m.histIdx = n
		m.histDraft = ""
		return
	}
	m.history = append(m.history, text)
	m.histIdx = len(m.history)
	m.histDraft = ""
}

// hScroll 结果区聚焦时 ←/→ 横向滚动：按列调整可见窗口并重建表格
//（bubbles/table 无原生水平滚动，实现方式见 renderWindow）。
func (m Model) hScroll(right bool) (tea.Model, tea.Cmd) {
	n := len(m.resCols)
	if n == 0 {
		return m, nil
	}
	if right {
		if m.hOffset < n-1 {
			m.hOffset++
		}
	} else if m.hOffset > 0 {
		m.hOffset--
	} else {
		return m, nil
	}
	m.renderWindow()
	return m, nil
}

// submit 提交编辑器内容（对应 v3 的 Ctrl+X 分支，加元命令与多语句支持）
func (m Model) submit() (tea.Model, tea.Cmd) {
	if m.state != stateIdle {
		return m, nil
	}

	text := strings.TrimSpace(m.editor.Value())
	if text == "" {
		return m, nil
	}
	m.editor.SetValue("")
	m.remember(text)

	// 元命令
	if strings.HasPrefix(text, "\\") {
		return m.runMeta(text)
	}

	stmts := splitStatements(text)
	if len(stmts) == 0 {
		return m, nil
	}

	m.state = stateRunning
	m.statusText = "执行中"
	m.errText = ""
	m.textLines = nil

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	conn := m.conn
	return m, func() tea.Msg {
		return runStatements(ctx, conn, stmts)
	}
}

// runStatements 顺序执行多条语句：保留最后一条有结果集的查询用于展示
func runStatements(ctx context.Context, conn *sql.DB, stmts []string) tea.Msg {
	start := time.Now()

	var (
		last      *output.QueryResult
		lastTotal int
		lastTrunc bool
		msgs      []string
	)

	for _, s := range stmts {
		if err := ctx.Err(); err != nil {
			return queryDoneMsg{outcome: outcome{err: err}, label: s}
		}

		o := runSQL(ctx, conn, s)
		if o.err != nil {
			return queryDoneMsg{outcome: o, label: s}
		}

		if o.result.Message != "" {
			msgs = append(msgs, o.result.Message)
		}
		if len(o.result.Columns) > 0 {
			last, lastTotal, lastTrunc = o.result, o.total, o.truncated
		}
	}

	// 没有任何结果集时，汇总非查询消息
	final := outcome{result: last, total: lastTotal, truncated: lastTrunc, elapsed: time.Since(start)}
	if last == nil {
		final.result = &output.QueryResult{Message: strings.Join(msgs, "；")}
	}

	return queryDoneMsg{outcome: final}
}

// runMeta 处理元命令（纯业务：\? 帮助、\clear、\c 切换、\dt/\d 元查询）
func (m Model) runMeta(line string) (tea.Model, tea.Cmd) {
	act, err := parseMeta(line, m.info.Type, m.info.Database, m.info.Username)
	if err != nil {
		m.errText = err.Error()
		return m, nil
	}

	switch act.kind {
	case metaQuit:
		return m, tea.Quit

	case metaHelp:
		m.errText = ""
		m.textLines = strings.Split(act.text, "\n")
		m.statusText = "\\? 帮助"
		return m, nil

	case metaClear:
		m.table = table.New(table.WithFocused(false))
		m.cursor = 0
		m.resCols, m.resRows = nil, nil
		m.hOffset = 0
		m.textLines = nil
		m.errText = ""
		m.statusText = "已清空"
		return m, nil

	case metaExpand:
		m.statusText = "\\x 已移除（极简版）"
		return m, nil

	case metaFormat:
		m.statusText = "\\o 已移除（极简版）"
		return m, nil

	case metaSwitch:
		return m.switchContext(act.text)

	case metaQuery:
		m.errText = ""
		m.textLines = nil
		m.state = stateRunning
		m.statusText = "执行中"

		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel

		conn := m.conn
		q, args := act.query, act.args
		return m, func() tea.Msg {
			start := time.Now()
			o := runMetaQuery(ctx, conn, q, args)
			o.elapsed = time.Since(start)
			return queryDoneMsg{outcome: o, label: line}
		}
	}

	return m, nil
}

// switchContext 切换到另一个已保存的连接
func (m Model) switchContext(name string) (tea.Model, tea.Cmd) {
	if name == "" {
		m.errText = "用法: \\c <连接名称>；已保存: " + strings.Join(config.ListContexts(), ", ")
		return m, nil
	}

	// 先校验 context 存在，避免把现有连接关掉后才发现连不上
	info, err := infoFromContext(name)
	if err != nil {
		m.errText = err.Error()
		return m, nil
	}

	old := m.conn
	conn, err := sqldb.Connect(name, nil)
	if err != nil {
		m.errText = err.Error()
		return m, nil
	}
	// 新连接同样要固定单连接，否则切过去之后又掉进多连接的老问题
	pinSingleConn(conn)

	if old != nil {
		_ = old.Close()
	}

	m.conn = conn
	m.info = info
	m.errText = ""
	m.textLines = nil
	m.table = table.New(table.WithFocused(false))
	m.cursor = 0
	m.resCols, m.resRows = nil, nil
	m.hOffset = 0
	m.statusText = "已切换到 " + name

	return m, nil
}

func infoFromContext(name string) (ConnInfo, error) {
	ctx, err := config.GetContext(name)
	if err != nil {
		return ConnInfo{}, err
	}
	return ConnInfo{
		Name:     name,
		Type:     ctx.Type,
		Host:     ctx.Host,
		Port:     ctx.Port,
		Username: ctx.Username,
		Database: ctx.Database,
	}, nil
}

// ---- 布局与视图（v3 形态）----

func (m Model) headerText() string {
	name := m.info.Name
	if name == "" {
		name = "(临时连接)"
	}

	loc := m.info.Host
	if m.info.Port > 0 {
		loc = fmt.Sprintf("%s:%d", m.info.Host, m.info.Port)
	}

	user := ""
	if m.info.Username != "" {
		user = " · " + m.info.Username
	}

	db := ""
	if m.info.Database != "" {
		db = " · " + m.info.Database
	}

	s := fmt.Sprintf("sqlo › %s · %s%s%s", name, m.info.Type, user, db)
	if loc != "" && loc != ":" {
		s += " · " + loc
	}
	return s
}

func (m Model) bodyView() string {
	w := viewW(m.width)

	if m.errText != "" {
		return truncate("错误: "+m.errText, w)
	}

	if len(m.textLines) > 0 {
		h := m.height - 9
		if h < 1 {
			h = 1
		}
		lines := m.textLines
		if len(lines) > h {
			lines = lines[:h]
		}
		var b strings.Builder
		for i, l := range lines {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(truncate(l, w))
		}
		return b.String()
	}

	if len(m.table.Rows()) > 0 {
		return m.table.View()
	}

	// 无结果时保持结果区为空（提示信息多余，去掉）
	return ""
}

func focusName(focused bool) string {
	if focused {
		return "结果区"
	}
	return "输入区"
}

// View 组装最终画面——与 v3 完全相同：纯字符串拼接 5 段，
// 每段按「终端宽-1」rune 截断。
func (m Model) View() string {
	w := viewW(m.width)
	var b strings.Builder

	// 顶栏（纯文本）
	top := m.headerText()
	if m.state == stateRunning {
		top += "   " + m.spin.View() + " 执行中"
	}
	b.WriteString(truncate(top, w))
	b.WriteString("\n")

	// 结果区
	b.WriteString(m.bodyView())
	b.WriteString("\n")

	// 状态行（纯文本单行截断，不填充、不折行）
	status := m.statusText
	if m.state == stateRunning {
		status = m.spin.View() + " " + status
	}
	b.WriteString(truncate("状态: "+status+"   焦点: "+focusName(m.focused), w))
	b.WriteString("\n")

	// 编辑器
	b.WriteString(m.editor.View())
	b.WriteString("\n")

	// 帮助行
	b.WriteString(truncate("Ctrl+X 执行   Tab 切换焦点   ↑/↓ 历史/滚动   ←/→ 横滚   Ctrl+C 取消/退出   \\? 帮助", w))
	return b.String()
}

// viewW 返回视图可用宽度：终端宽减 1，保证渲染出的每一行都严格窄于终端。
func viewW(termW int) int {
	w := termW - 1
	if w < 10 {
		w = 80
	}
	return w
}
