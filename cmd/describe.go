package cmd

import (
	"github.com/liang-junwei/sqlo/internal/driver"
	"github.com/spf13/cobra"
)

var describeCmd = &cobra.Command{
	Use:     "describe [schema.]表",
	Aliases: []string{"desc"},
	Short:   "查看表结构",
	Long: `查看指定表的列结构（与 TUI 中的 \d 使用同一套驱动层逻辑）。

不带表名时退化为列出表，等同 sqlo tables。

示例:
  sqlo describe users
  sqlo describe public.users
  sqlo describe users -o json
  sqlo describe users -S prod-pg`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, _ := cmd.Flags().GetString("database")

		target := ""
		if len(args) > 0 {
			target = args[0]
		}

		conn, ctx, err := connectForMeta(database)
		if err != nil {
			return err
		}
		defer conn.Close()

		// -d 覆盖优先，否则用 context 的默认库作为 schema 兜底
		currentDB := database
		if currentDB == "" {
			currentDB = ctx.Database
		}

		q, qargs, err := driver.BuildDescribe(ctx.Type, currentDB, target)
		if err != nil {
			return err
		}

		return runMetaQuery(conn, q, qargs)
	},
}

func init() {
	describeCmd.Flags().StringP("database", "d", "", "临时切换数据库（不修改配置文件）")
	rootCmd.AddCommand(describeCmd)
}
