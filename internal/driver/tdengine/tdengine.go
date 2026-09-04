package tdengine

import (
	_ "github.com/taosdata/driver-go/v3/taosWS"
	"github.com/liang-junwei/sqlo/internal/driver"
)

func init() {
	// 注册名必须为 taosWS（与 github.com/taosdata/driver-go/v3/taosWS 的 sql.Register 一致）
	// 选择 WebSocket 驱动而非原生 taosSql：原生依赖 CGO 且将于 2027-01-01 下线；
	// taosWS 走 WebSocket（taosAdapter，默认端口 6041），纯 Go、无 CGO。
	driver.Register("tdengine", &driver.Driver{
		Name:        "taosWS",
		DefaultPort: 6041,
		Metadata: driver.MetadataQueries{
			ListTables:    `SELECT table_name FROM information_schema.ins_tables WHERE db_name = ? ORDER BY table_name`,
			ListSchemas:   `SELECT name FROM information_schema.ins_databases ORDER BY name`,
			ListDatabases: `SELECT name FROM information_schema.ins_databases ORDER BY name`,
			DescribeTable: `SELECT column_name AS name, data_type AS type FROM information_schema.ins_columns WHERE table_name = ? ORDER BY column_id`,
		},
		Features: driver.Features{
			Transactions: false, // TDengine 不支持多语句事务
			Returning:    false, // TDengine 不支持 RETURNING
			LimitOffset:  true,
		},
	})
}
