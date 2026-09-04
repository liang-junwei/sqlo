package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/liang-junwei/sqlo/internal/driver"
	"github.com/liang-junwei/sqlo/internal/output"
	"github.com/spf13/cobra"
)

var databasesCmd = &cobra.Command{
	Use:     "databases",
	Aliases: []string{"dbs"},
	Short:   "列出当前支持的数据库类型",
	Long: `列出 sqlo 当前已编译支持的所有数据库类型及其默认端口。

该列表由驱动注册表在运行时动态生成，始终与可执行文件实际支持的数据库保持一致，
不再依赖命令帮助文本中的硬编码枚举。可用 -o 指定输出格式 (table/json/yaml/csv)。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		names := driver.List()
		sort.Strings(names)

		rows := make([][]interface{}, 0, len(names))
		for _, name := range names {
			drv, err := driver.Get(name)
			if err != nil {
				// 注册表与 Get 应保持一致；异常时跳过该条目而非中断
				continue
			}
			port := "-"
			if drv.DefaultPort > 0 {
				port = fmt.Sprintf("%d", drv.DefaultPort)
			}
			rows = append(rows, []interface{}{name, port})
		}

		format := GetOutputFormat()
		formatter, err := output.GetFormatter(format)
		if err != nil {
			return err
		}

		result := &output.QueryResult{
			Columns: []string{"TYPE", "DEFAULT_PORT"},
			Rows:    rows,
		}
		return formatter.Format(os.Stdout, result)
	},
}

func init() {
	rootCmd.AddCommand(databasesCmd)
}
