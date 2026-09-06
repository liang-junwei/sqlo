package access

import (
	_ "github.com/mattn/go-adodb" // ADODB 驱动 (Windows only, 无需 CGO)
	"github.com/liang-junwei/sqlo/internal/driver"
)

func init() {
	driver.Register("access", &driver.Driver{
		Name:        "access",
		DefaultPort: 0, // 文件型数据库，不需要端口
		Metadata: driver.MetadataQueries{
			ListTables:    `SELECT MSysObjects.Name FROM MSysObjects WHERE MSysObjects.Type=1 AND MSysObjects.Name NOT LIKE 'MSys%' ORDER BY MSysObjects.Name`,
			ListSchemas:   `SELECT 'default'`,
			ListDatabases: `SELECT 'default'`,
			DescribeTable: `SELECT ColumnName, TypeName, Nullable, ColumnDefault FROM MSysColumns WHERE TableName = ? ORDER BY OrdinalPosition`,

			// DescribeTable 只需 table 一个参数
			DescribeParamCount: 1,
		},
		Features: driver.Features{
			Transactions: true,
			Returning:    false, // MS Access 不支持 RETURNING
			LimitOffset:  false, // MS Access 使用 TOP N 而不是 LIMIT
		},
	})
}

