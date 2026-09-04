package main

import (
	"fmt"
	"os"

	"github.com/liang-junwei/sqlo/cmd"

	// 驱动空白导入 —— 按此顺序注册，新增驱动在此追加
	_ "github.com/liang-junwei/sqlo/internal/driver/access"
	_ "github.com/liang-junwei/sqlo/internal/driver/clickhouse"
	_ "github.com/liang-junwei/sqlo/internal/driver/mysql"
	_ "github.com/liang-junwei/sqlo/internal/driver/oracle"
	_ "github.com/liang-junwei/sqlo/internal/driver/postgres"
	_ "github.com/liang-junwei/sqlo/internal/driver/sqlite"
	_ "github.com/liang-junwei/sqlo/internal/driver/sqlserver"
	_ "github.com/liang-junwei/sqlo/internal/driver/tdengine"
	_ "github.com/liang-junwei/sqlo/internal/driver/dm"
	_ "github.com/liang-junwei/sqlo/internal/driver/kingbase"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
