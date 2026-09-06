package oracle

import (
	_ "github.com/sijms/go-ora/v2"
	"github.com/liang-junwei/sqlo/internal/driver"
)

func init() {
	driver.Register("oracle", &driver.Driver{
		Name:        "oracle",
		DefaultPort: 1521,
		Metadata: driver.MetadataQueries{
			ListTables:    `SELECT owner, table_name FROM all_tables WHERE owner NOT IN ('SYS', 'SYSTEM', 'XDB', 'CTXSYS', 'MDSYS', 'ORDSYS', 'OUTLN', 'DBSNMP') ORDER BY owner, table_name`,
			ListSchemas:   `SELECT username FROM all_users ORDER BY username`,
			ListDatabases: `SELECT name FROM v$database`,
			DescribeTable: `SELECT column_name, data_type, nullable, data_default FROM all_tab_columns WHERE owner = :1 AND table_name = :2 ORDER BY column_id`,

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
