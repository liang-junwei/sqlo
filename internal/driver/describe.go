package driver

import (
	"fmt"
	"strings"
)

// describeOwnerAware 标记「1 参数驱动里，显式带 schema 时需要把 schema 当作
// owner/namespace 一并传入过滤」的驱动。
//
// 这类驱动的 DescribeTable 模板原本只用 table_name 过滤（如 dameng 的
// all_tab_columns WHERE table_name = ?）。若丢弃 schema 只按表名查，
// 该表在多个 owner 下可见时元数据查询会返回重复行（每个可见 owner 一份），
// 表现为「每个字段重复 N 次」。这里在显式给出 schema 时注入 owner 过滤，
// 既消除重复，又保留不带 schema 时的原有行为不变。
var describeOwnerAware = map[string]bool{
	"dameng": true,
}

// BuildDescribe 按驱动构造表结构查询，返回可直接执行的 SQL 与参数。
//
// target 为空时退化为列出表（与 TUI 中 \d 不带参数的行为一致），
// 避免调用方面对未知的参数个数要求。
//
// currentDB 用于 2 参数驱动在未显式给出 schema 时作为默认 schema。
func BuildDescribe(drvType, currentDB, target string) (string, []interface{}, error) {
	drv, err := Get(drvType)
	if err != nil {
		return "", nil, err
	}

	target = strings.TrimSpace(target)
	if target == "" {
		q := strings.TrimSpace(drv.Metadata.ListTables)
		if q == "" {
			return "", nil, fmt.Errorf("数据库类型 %s 未提供该元数据查询", drvType)
		}
		return q, nil, nil
	}

	q := strings.TrimSpace(drv.Metadata.DescribeTable)
	if q == "" {
		return "", nil, fmt.Errorf("数据库类型 %s 未提供表结构查询", drvType)
	}

	schema, table := SplitQualified(target)

	if drv.Metadata.DescribeParamCount == 1 {
		// dameng 显式带 schema 时，注入 owner 过滤以避免多 owner 下重复行
		if describeOwnerAware[drvType] && schema != "" {
			q = strings.Replace(q, "table_name = ?", "owner = ? AND table_name = ?", 1)
			return q, []interface{}{schema, table}, nil
		}
		return q, []interface{}{table}, nil
	}

	// 需要 (schema, table) 两个参数
	if schema == "" {
		if currentDB == "" {
			return "", nil, fmt.Errorf("数据库类型 %s 需要 schema，请用 <schema>.<表> 形式（当前连接未指定数据库）", drvType)
		}
		schema = currentDB
	}

	return q, []interface{}{schema, table}, nil
}

// SplitQualified 拆分 "schema.table"，兼容引号包裹。
// 不含 "." 时 schema 返回空字符串。
func SplitQualified(name string) (schema, table string) {
	name = strings.TrimSpace(name)
	if i := strings.LastIndex(name, "."); i > 0 {
		return strings.Trim(name[:i], "\"'"), strings.Trim(name[i+1:], "\"'")
	}
	return "", name
}
