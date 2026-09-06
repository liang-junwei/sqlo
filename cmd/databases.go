package cmd

import (
	"fmt"
	"strings"

	"github.com/liang-junwei/sqlo/internal/driver"
	"github.com/spf13/cobra"
)

var databasesCmd = &cobra.Command{
	Use:   "databases",
	Short: "列出服务器上的数据库",
	Long: `列出当前连接服务器上的数据库，使用驱动自带的元数据查询。

注意与 sqlo drivers 区分：
  sqlo drivers   —— sqlo 支持哪些数据库驱动类型（离线，不需要连接）
  sqlo databases —— 当前连的这台服务器上有哪些库（需要连接）

示例:
  sqlo databases
  sqlo databases -o json
  sqlo databases -S prod-pg`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, ctx, err := connectForMeta("")
		if err != nil {
			return err
		}
		defer conn.Close()

		drv, err := driver.Get(ctx.Type)
		if err != nil {
			return err
		}

		q := strings.TrimSpace(drv.Metadata.ListDatabases)
		if q == "" {
			return fmt.Errorf("数据库类型 %s 未提供列库查询", ctx.Type)
		}

		return runMetaQuery(conn, q, nil)
	},
}

func init() {
	rootCmd.AddCommand(databasesCmd)
}
