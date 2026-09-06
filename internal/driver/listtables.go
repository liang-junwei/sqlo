package driver

import (
	"fmt"
	"strings"
)

// listTablesOwnerAware 标记「ListTables 需要 1 个参数、且该参数为 owner
// （当前连接用户 / schema）」的驱动。
//
// dameng 的 all_tables 用 `owner = ?` 过滤，owner 即登录用户对应的 schema；
// 其余 1 参数驱动（如 tdengine 的 `db_name = ?`）按当前数据库过滤。
// 这里显式列出，保持与 describeOwnerAware 一致的「按驱动名映射」风格。
var listTablesOwnerAware = map[string]bool{
	"dameng": true,
}

// BuildListTables 按驱动构造"列出表"查询，返回可直接执行的 SQL 与参数。
//
// 多数驱动的 ListTables 无需参数；dameng（owner=?）与 tdengine（db_name=?）
// 需要 1 个参数，分别由 currentUser / currentDB 提供：
//   - dameng 的 owner 取当前连接用户名（schema）；
//   - tdengine 的 db_name 取当前数据库。
//
// currentDB 一般来自连接的 Database（或 -d 临时覆盖）；
// currentUser 一般来自连接的 Username。
func BuildListTables(drvType, currentDB, currentUser string) (string, []interface{}, error) {
	drv, err := Get(drvType)
	if err != nil {
		return "", nil, err
	}
	q := strings.TrimSpace(drv.Metadata.ListTables)
	if q == "" {
		return "", nil, fmt.Errorf("数据库类型 %s 未提供列表查询", drvType)
	}

	if drv.Metadata.ListTablesParamCount == 0 {
		return q, nil, nil
	}

	if listTablesOwnerAware[drvType] {
		if currentUser == "" {
			return "", nil, fmt.Errorf("数据库类型 %s 需要用户名以确定 owner，但当前连接未提供用户名", drvType)
		}
		return q, []interface{}{currentUser}, nil
	}

	// 其余 1 参数驱动按当前数据库过滤（如 tdengine 的 db_name=?）
	if currentDB == "" {
		return "", nil, fmt.Errorf("数据库类型 %s 需要数据库名以确定过滤条件，但当前连接未指定数据库", drvType)
	}
	return q, []interface{}{currentDB}, nil
}
