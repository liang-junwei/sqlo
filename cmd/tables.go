package cmd

import (
	"github.com/liang-junwei/sqlo/internal/driver"
	"github.com/spf13/cobra"
)

var tablesCmd = &cobra.Command{
	Use:   "tables",
	Short: "列出当前库的表",
	Long: `列出当前连接数据库中的表，使用驱动自带的元数据查询。

示例:
  sqlo tables
  sqlo tables -o json
  sqlo tables -S prod-pg
  sqlo tables -d other_db`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, _ := cmd.Flags().GetString("database")

		conn, ctx, err := connectForMeta(database)
		if err != nil {
			return err
		}
		defer conn.Close()

		// 当前库：优先 -d 临时覆盖，否则取连接默认库
		currentDB := ctx.Database
		if database != "" {
			currentDB = database
		}

		q, qargs, err := driver.BuildListTables(ctx.Type, currentDB, ctx.Username)
		if err != nil {
			return err
		}

		return runMetaQuery(conn, q, qargs)
	},
}

func init() {
	tablesCmd.Flags().StringP("database", "d", "", "临时切换数据库（不修改配置文件）")
	rootCmd.AddCommand(tablesCmd)
}
