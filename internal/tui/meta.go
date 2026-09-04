package tui

import (
	"fmt"
	"strings"

	"github.com/liang-junwei/sqlo/internal/driver"
)

// describeArgs 记录各驱动 DescribeTable 需要的参数个数。
//
// 既有驱动的 Metadata.DescribeTable 占位符风格与参数个数并不统一：
//   - postgres / kingbase 用 $1 $2
//   - mysql / clickhouse 用 ? ?
//   - sqlserver 用 @p1 @p2
//   - oracle 用 :1 :2
//     以上均需要 (schema, table) 两个参数
//   - tdengine / dameng / sqlite / access 只需要 table 一个参数
//
// 这里在 TUI 内部局部适配，不改动驱动层。
// 若后续在 driver.MetadataQueries 上补充「参数个数」声明，本表即可移除。
var describeArgs = map[string]int{
	"postgres":   2,
	"kingbase":   2,
	"mysql":      2,
	"clickhouse": 2,
	"sqlserver":  2,
	"oracle":     2,
	"tdengine":   1,
	"dameng":     1,
	"sqlite":     1,
	"access":     1,
}

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
func parseMeta(line, drvType, currentDB string) (metaAction, error) {
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
		return metaQueryAction(drvType, func(md driver.MetadataQueries) string { return md.ListTables })
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

// describeOwnerAware 标记“1 参数驱动里，\d <schema>.<表> 显式带 schema 时
// 需要把 schema 当作 owner/namespace 一并传入过滤”的驱动。
//
// 这类驱动的 DescribeTable 模板原本只用 table_name 过滤（如 dameng 的
// all_tab_columns WHERE table_name = ?）。若 \d 时丢弃 schema，只按表名查，
// 该表在多个 owner 下可见时元数据查询会返回重复行（每个可见 owner 一份），
// 表现为“每个字段重复 N 次”。这里在显式给出 schema 时注入 owner 过滤，
// 既消除重复，又保留“\d <表>（不带 schema）”的原有行为不变。
var describeOwnerAware = map[string]bool{
	"dameng": true,
}

// describeAction 构造表结构查询，按驱动决定参数个数
func describeAction(drvType, currentDB, target string) (metaAction, error) {
	drv, err := driver.Get(drvType)
	if err != nil {
		return metaAction{}, err
	}
	q := strings.TrimSpace(drv.Metadata.DescribeTable)
	if q == "" {
		return metaAction{}, fmt.Errorf("数据库类型 %s 未提供表结构查询", drvType)
	}

	// 不带表名时退化为列出表，避免用户面对未知的参数个数要求
	if target == "" {
		return metaQueryAction(drvType, func(md driver.MetadataQueries) string { return md.ListTables })
	}

	schema, table := splitQualified(target)

	if describeArgs[drvType] == 1 {
		// dameng 显式带 schema 时，注入 owner 过滤以避免多 owner 下重复行
		if describeOwnerAware[drvType] && schema != "" {
			q = strings.Replace(q, "table_name = ?", "owner = ? AND table_name = ?", 1)
			return metaAction{kind: metaQuery, query: q, args: []interface{}{schema, table}}, nil
		}
		return metaAction{kind: metaQuery, query: q, args: []interface{}{table}}, nil
	}

	// 需要 (schema, table) 两个参数
	if schema == "" {
		if currentDB == "" {
			return metaAction{}, fmt.Errorf("该库需要 schema，请用 \\d <schema>.<表>（当前连接未指定数据库）")
		}
		schema = currentDB
	}

	return metaAction{kind: metaQuery, query: q, args: []interface{}{schema, table}}, nil
}

// splitQualified 拆分 "schema.table"，兼容引号包裹
func splitQualified(name string) (schema, table string) {
	name = strings.TrimSpace(name)
	if i := strings.LastIndex(name, "."); i > 0 {
		return strings.Trim(name[:i], "\"'"), strings.Trim(name[i+1:], "\"'")
	}
	return "", name
}
