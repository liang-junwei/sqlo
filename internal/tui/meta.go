package tui

import (
	"fmt"
	"strings"

	"github.com/liang-junwei/sqlo/internal/driver"
)

type metaKind int

const (
	metaNone metaKind = iota
	metaQuery
	metaQuit
	metaSwitch
	metaHelp
	metaClear
	metaExpand
	metaFormat
)

// metaAction 元命令解析结果
type metaAction struct {
	kind  metaKind
	query string
	args  []interface{}
	text  string // 附加文本（帮助内容、要切换的 context 名等）
}

const metaHelpText = `元命令
  \dt                列出当前库的表
  \dn                列出 schema
  \l                 列出数据库
  \d [schema.]表     查看表结构（不带参数时等同 \dt）
  \c <name>          切换到另一个已保存的连接
  \clear             清空结果区
  \q                 退出
  \?                 显示本帮助

按键
  Ctrl+X             执行当前 SQL
  Tab                在编辑器与结果区之间切换焦点
  ↑ / ↓              输入区（单行）翻阅历史；结果区聚焦时上下滚动
  ← / →              结果区聚焦时横向滚动（列超出终端宽时）
  Ctrl+C             取消正在执行的查询；空闲时退出
  Esc                退出

说明
  SQL 以分号分隔可一次执行多条；单次查询最多保留 1000 行，
  超出会截断并显示真实总行数。结果仅以表格形式展示。`

// parseMeta 解析以反斜杠开头的元命令
func parseMeta(line, drvType, currentDB, currentUser string) (metaAction, error) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "\\") {
		return metaAction{kind: metaNone}, nil
	}

	body := strings.TrimSpace(strings.TrimPrefix(s, "\\"))
	if body == "" {
		return metaAction{kind: metaHelp, text: metaHelpText}, nil
	}

	fields := strings.Fields(body)
	cmd := fields[0]
	arg := ""
	if len(fields) > 1 {
		arg = strings.Join(fields[1:], " ")
	}

	switch cmd {
	case "?", "help", "h":
		return metaAction{kind: metaHelp, text: metaHelpText}, nil
	case "q", "quit", "exit":
		return metaAction{kind: metaQuit}, nil
	case "x":
		return metaAction{kind: metaExpand}, nil
	case "o", "output":
		outFmt := "table"
		if len(fields) > 1 {
			outFmt = strings.ToLower(fields[1])
		}
		switch outFmt {
		case "table", "json", "csv", "yaml":
			return metaAction{kind: metaFormat, text: outFmt}, nil
		default:
			return metaAction{}, fmt.Errorf("未知输出格式: %s（支持 table/json/csv/yaml）", fields[1])
		}
	case "clear", "cls":
		return metaAction{kind: metaClear}, nil
	case "c", "connect":
		return metaAction{kind: metaSwitch, text: arg}, nil
	case "dt":
		return listTablesAction(drvType, currentDB, currentUser)
	case "dn":
		return metaQueryAction(drvType, func(md driver.MetadataQueries) string { return md.ListSchemas })
	case "l":
		return metaQueryAction(drvType, func(md driver.MetadataQueries) string { return md.ListDatabases })
	case "d":
		return describeAction(drvType, currentDB, arg)
	}

	return metaAction{}, fmt.Errorf("未知元命令: \\%s（用 \\? 查看帮助）", cmd)
}

// metaQueryAction 取出驱动的元数据 SQL（无参数）
func metaQueryAction(drvType string, pick func(driver.MetadataQueries) string) (metaAction, error) {
	drv, err := driver.Get(drvType)
	if err != nil {
		return metaAction{}, err
	}
	q := strings.TrimSpace(pick(drv.Metadata))
	if q == "" {
		return metaAction{}, fmt.Errorf("数据库类型 %s 未提供该元数据查询", drvType)
	}
	return metaAction{kind: metaQuery, query: q}, nil
}

// describeAction 构造表结构查询。
// 参数个数、占位符风格、owner 过滤等驱动差异已下沉到 driver.BuildDescribe，
// 这里只做 TUI 侧的封装（\d 与 sqlo describe 共用同一套逻辑）。
func describeAction(drvType, currentDB, target string) (metaAction, error) {
	q, args, err := driver.BuildDescribe(drvType, currentDB, target)
	if err != nil {
		return metaAction{}, err
	}
	return metaAction{kind: metaQuery, query: q, args: args}, nil
}

// listTablesAction 构造"列出表"查询。
// 参数个数、占位符风格等驱动差异已下沉到 driver.BuildListTables，
// 这里只做 TUI 侧的封装（\dt 与 sqlo tables 共用同一套逻辑）。
func listTablesAction(drvType, currentDB, currentUser string) (metaAction, error) {
	q, args, err := driver.BuildListTables(drvType, currentDB, currentUser)
	if err != nil {
		return metaAction{}, err
	}
	return metaAction{kind: metaQuery, query: q, args: args}, nil
}
