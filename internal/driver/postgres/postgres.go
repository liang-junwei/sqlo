package postgres

import (
	_ "github.com/lib/pq"
	"github.com/liang-junwei/sqlo/internal/driver"
)

func init() {
	driver.Register("postgres", &driver.Driver{
		Name:        "postgres",
		DefaultPort: 5432,
		Metadata: driver.MetadataQueries{
			ListTables:    `SELECT table_schema, table_name FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog', 'information_schema') ORDER BY table_schema, table_name`,
			ListSchemas:   `SELECT schema_name FROM information_schema.schemata ORDER BY schema_name`,
			ListDatabases: `SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname`,
			DescribeTable: `SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`,
		},
		Features: driver.Features{
			Transactions: true,
			Returning:    true,
			LimitOffset:  true,
		},
	})
}
