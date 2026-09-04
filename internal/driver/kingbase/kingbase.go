package kingbase

import (
	_ "kingbase.com/gokb"
	"github.com/liang-junwei/sqlo/internal/driver"
)

func init() {
	// 注册名必须为 kingbase（与 gokb 驱动 conn.go 中 sql.Register("kingbase", ...) 一致）
	// Kingbase ES 基于 PostgreSQL 协议，官方 Go 驱动 gokb 为纯 Go 实现（自研线协议，无 CGO），
	// 完全契合 sqlo 的零 gcc 取向。
	// 默认端口 54321（connector.go / doc.go 双重确认）。
	// 元数据查询兼容 PostgreSQL 体系：表/模式/列用 information_schema；
	// 查库用 Kingbase 特有的 sys_catalog.sys_database（pg_database 在 KES 上未必存在，待真机验证）。
	driver.Register("kingbase", &driver.Driver{
		Name:        "kingbase",
		DefaultPort: 54321,
		Metadata: driver.MetadataQueries{
			ListTables:    `SELECT table_schema, table_name FROM information_schema.tables WHERE table_schema NOT IN ('information_schema', 'sys_catalog', 'pg_catalog') ORDER BY table_schema, table_name`,
			ListSchemas:   `SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('information_schema', 'sys_catalog', 'pg_catalog') ORDER BY schema_name`,
			ListDatabases: `SELECT datname FROM sys_catalog.sys_database ORDER BY datname`,
			DescribeTable: `SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`,
		},
		Features: driver.Features{
			Transactions: true,
			Returning:    true,
			LimitOffset:  true,
		},
	})
}
