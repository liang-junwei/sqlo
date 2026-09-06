package dm

import (
	_ "dm"
	"github.com/liang-junwei/sqlo/internal/driver"
)

func init() {
	// 用户面向的注册名(即 --type 与 databases 列表中的名称)为 dameng；
	// 底层 sql 驱动名 Name 必须保持为 dm，与达梦官方驱动 p.go 中 sql.Register("dm", ...) 一致，
	// 这样 driver.Open 调用 sql.Open("dm", dsn) 才能命中官方已注册的驱动。
	// 达梦官方 Go 驱动为纯 Go 实现（自研 DM 线协议，无 CGO、无原生库依赖），
	// 完全契合 sqlo 的零 gcc 取向。
	// DSN 格式: dm://user:password@host:port/dbname（必须以 dm:// 开头，dbname 用 /dbname 形式）
	// 默认端口 5236。
	driver.Register("dameng", &driver.Driver{
		Name:        "dm",
		DefaultPort: 5236,
		Metadata: driver.MetadataQueries{
			ListTables:    `SELECT table_name FROM all_tables WHERE owner = ? ORDER BY table_name`,
			ListSchemas:   `SELECT username FROM all_users ORDER BY username`,
			ListDatabases: `SELECT name FROM v$database`,
			DescribeTable: `SELECT column_name AS name, data_type AS type FROM all_tab_columns WHERE table_name = ? ORDER BY column_id`,

			// DescribeTable 只需 table 一个参数
			DescribeParamCount: 1,
			// ListTables 的 owner=? 需要 1 个参数（当前连接用户名）
			ListTablesParamCount: 1,
		},
		Features: driver.Features{
			Transactions: true,  // 达梦支持事务
			Returning:    false, // 达梦不支持 RETURNING（用序列/触发器替代）
			LimitOffset:  true,  // 达梦支持 LIMIT/OFFSET
		},
	})
}
