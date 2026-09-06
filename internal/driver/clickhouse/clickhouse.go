package clickhouse

import (
	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/liang-junwei/sqlo/internal/driver"
)

func init() {
	driver.Register("clickhouse", &driver.Driver{
		Name:        "clickhouse",
		DefaultPort: 9000,
		Metadata: driver.MetadataQueries{
			ListTables:    `SELECT database, name FROM system.tables WHERE database NOT IN ('system', 'information_schema') ORDER BY database, name`,
			ListSchemas:   `SELECT name FROM system.databases ORDER BY name`,
			ListDatabases: `SELECT name FROM system.databases ORDER BY name`,
			DescribeTable: `SELECT name, type, default_expression, comment FROM system.columns WHERE database = ? AND table = ? ORDER BY position`,

			// DescribeTable 需要 (schema, table) 两个参数
			DescribeParamCount: 2,
		},
		Features: driver.Features{
			Transactions: false, // ClickHouse 多数表引擎不支持事务
			Returning:    false, // ClickHouse 不支持 RETURNING
			LimitOffset:  true,
		},
	})
}
