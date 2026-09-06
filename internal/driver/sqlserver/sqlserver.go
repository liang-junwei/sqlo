package sqlserver

import (
	_ "github.com/microsoft/go-mssqldb"
	"github.com/liang-junwei/sqlo/internal/driver"
)

func init() {
	driver.Register("sqlserver", &driver.Driver{
		Name:        "sqlserver",
		DefaultPort: 1433,
		Metadata: driver.MetadataQueries{
			ListTables:    `SELECT TABLE_SCHEMA, TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE = 'BASE TABLE' AND TABLE_SCHEMA NOT IN ('sys') ORDER BY TABLE_SCHEMA, TABLE_NAME`,
			ListSchemas:   `SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA ORDER BY SCHEMA_NAME`,
			ListDatabases: `SELECT name FROM sys.databases WHERE name NOT IN ('master', 'tempdb', 'model', 'msdb') ORDER BY name`,
			DescribeTable: `SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = @p1 AND TABLE_NAME = @p2 ORDER BY ORDINAL_POSITION`,

			// DescribeTable 需要 (schema, table) 两个参数
			DescribeParamCount: 2,
		},
		Features: driver.Features{
			Transactions: true,
			Returning:    true,
			LimitOffset:  true,
		},
	})
}
