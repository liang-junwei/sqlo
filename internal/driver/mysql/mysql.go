package mysql

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/liang-junwei/sqlo/internal/driver"
)

func init() {
	driver.Register("mysql", &driver.Driver{
		Name:        "mysql",
		DefaultPort: 3306,
		Metadata: driver.MetadataQueries{
			ListTables:    `SELECT table_schema, table_name FROM information_schema.tables WHERE table_schema NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys') ORDER BY table_schema, table_name`,
			ListSchemas:   `SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys') ORDER BY schema_name`,
			ListDatabases: `SELECT schema_name FROM information_schema.schemata ORDER BY schema_name`,
			DescribeTable: `SELECT column_name, data_type, is_nullable, column_default, column_key, extra FROM information_schema.columns WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position`,
		},
		Features: driver.Features{
			Transactions: true,
			Returning:    false,
			LimitOffset:  true,
		},
	})
}
