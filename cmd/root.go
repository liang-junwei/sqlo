package cmd

import (
	"fmt"
	"os"

	"github.com/liang-junwei/sqlo/internal/config"
	"github.com/spf13/cobra"
)

var (
	// 全局参数
	flagServer  string
	flagOutput  string
	flagQuiet   bool
	flagVerbose bool
)

var rootCmd = &cobra.Command{
	Use:   "sqlo",
	Short: "SQL CLI - 多数据库命令行工具",
	Long: `sqlo 是一个支持多种数据库的命令行 SQL 工具。

通过 connect 命令管理数据库连接，使用 query 命令执行 SQL。`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// version 与 databases 不需要加载用户配置
		if cmd.Name() == "version" || cmd.Name() == "databases" {
			return nil
		}

		if err := config.Load(); err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}

		return nil
	},
}

func init() {
	// 全局持久化参数
	rootCmd.PersistentFlags().StringVarP(&flagServer, "server", "S", "", "指定目标 server context 名称")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "table", "输出格式: table, json, yaml, csv")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "静默模式，仅输出结果")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "详细输出")
}

// Execute 执行根命令
func Execute() error {
	return rootCmd.Execute()
}

// GetServer 获取目标 server 名称（优先全局参数，其次配置默认值）
func GetServer() (string, error) {
	if flagServer != "" {
		return flagServer, nil
	}

	current := config.GetCurrentContextName()
	if current == "" {
		return "", fmt.Errorf("未指定 server 且未设置默认 context\n使用 'sqlo connect use <name>' 设置默认，或 '-S <name>' 指定目标")
	}

	return current, nil
}

// GetOutputFormat 获取输出格式
func GetOutputFormat() string {
	return flagOutput
}

// IsQuiet 是否静默模式
func IsQuiet() bool {
	return flagQuiet
}

// IsVerbose 是否详细模式
func IsVerbose() bool {
	return flagVerbose
}

// ExitWithError 输出错误并退出
func ExitWithError(msg string) {
	fmt.Fprintln(os.Stderr, "错误:", msg)
	os.Exit(1)
}
