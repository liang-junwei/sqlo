package sqlite

import (
	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动
	"github.com/liang-junwei/sqlo/internal/driver"
)

func init() {
	driver.Register("sqlite", &driver.Driver{
		Name:        "sqlite",
		DefaultPort: 0, // 文件型数据库，不需要端口
		Metadata: driver.MetadataQueries{
			ListTables:    `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`,
			ListSchemas:   `SELECT 'main'`, // SQLite 只有一个 main schema
			ListDatabases: `SELECT name FROM pragma_database_list ORDER BY seq`,
			DescribeTable: `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info(?) ORDER BY cid`,
		},
		Features: driver.Features{
			Transactions: true,
			Returning:    true, // SQLite 3.35.0+ 支持 RETURNING
			LimitOffset:  true,
		},
	})
}
