package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/liang-junwei/sqlo/internal/config"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "管理数据库连接配置",
	Long:  `connect 命令用于管理数据库连接配置（context），包括添加、删除、切换和查看连接信息。`,
}

var connectAddCmd = &cobra.Command{
	Use:   "add",
	Short: "添加新的数据库连接",
	Long: `添加新的数据库连接配置并保存到 ~/.sqlo.json。

支持的全部数据库类型及默认端口，请用 'sqlo databases' 查看。

示例:
  # 网络型数据库（典型）
  sqlo connect add -n prod-pg -t postgres -h 10.0.1.5 -P 5432 -u admin -p secret -d mydb

  # 文件型数据库（典型，仅需 -d 指定文件路径）
  sqlo connect add -n myapp -t sqlite -d ./data/myapp.db`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		dbType, _ := cmd.Flags().GetString("type")
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		database, _ := cmd.Flags().GetString("database")
		optionsStr, _ := cmd.Flags().GetString("options")
		setDefault, _ := cmd.Flags().GetBool("default")

		// 解析 options: "key1=value1,key2=value2"
		options := make(map[string]string)
		if optionsStr != "" {
			for _, pair := range strings.Split(optionsStr, ",") {
				parts := strings.SplitN(pair, "=", 2)
				if len(parts) == 2 {
					options[parts[0]] = parts[1]
				}
			}
		}

		if name == "" {
			return fmt.Errorf("连接名称不能为空 (使用 -n 指定)")
		}

		if dbType == "" {
			return fmt.Errorf("数据库类型不能为空 (使用 -t 指定，可用 'sqlo databases' 查看支持的类型)")
		}

		// 根据数据库类型区分网络型和文件型
		isFileBased := dbType == "sqlite" || dbType == "access"

		if !isFileBased {
			// 网络型数据库需要 host 和 username
			if host == "" {
				return fmt.Errorf("主机地址不能为空 (使用 -H 指定)")
			}

			if username == "" {
				return fmt.Errorf("用户名不能为空 (使用 -u 指定)")
			}

			// 如果未指定端口，使用驱动默认
			if port == 0 {
				port = getDefaultPort(dbType)
			}
		} else {
			// 文件型数据库需要 database（文件路径）
			if database == "" {
				return fmt.Errorf("数据库文件路径不能为空 (使用 -d 指定)")
			}
			port = 0 // 文件型数据库不需要端口
		}

		ctx := &config.Context{
			Name:     name,
			Type:     dbType,
			Host:     host,
			Port:     port,
			Username: username,
			Password: password,
			Database: database,
			Options:  options,
		}

		if err := config.AddContext(ctx, setDefault); err != nil {
			return err
		}

		fmt.Printf("✓ 连接 %q 已添加", name)
		if setDefault || config.GetCurrentContextName() == name {
			fmt.Printf(" (当前默认)")
		}
		fmt.Println()

		return nil
	},
}

var connectListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有数据库连接",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		contexts := config.ListContexts()
		if len(contexts) == 0 {
			fmt.Println("暂无连接配置，使用 'sqlo connect add' 添加")
			return nil
		}

		current := config.GetCurrentContextName()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "名称\t类型\t主机\t端口\t用户\t数据库\t默认")
		fmt.Fprintln(w, "----\t----\t----\t----\t----\t------\t----")

		for _, name := range contexts {
			ctx, err := config.GetContext(name)
			if err != nil {
				continue
			}

			marker := ""
			if name == current {
				marker = "*"
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
				name, ctx.Type, ctx.Host, ctx.Port, ctx.Username, ctx.Database, marker)
		}

		w.Flush()
		return nil
	},
}

var connectUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "切换默认数据库连接",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := config.UseContext(name); err != nil {
			return err
		}

		fmt.Printf("✓ 已切换到连接 %q\n", name)
		return nil
	},
}

var connectDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "删除数据库连接",
	Aliases: []string{"rm"},
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := config.DeleteContext(name); err != nil {
			return err
		}

		fmt.Printf("✓ 连接 %q 已删除\n", name)
		return nil
	},
}

var connectShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "查看连接详情",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		if len(args) > 0 {
			name = args[0]
		}

		ctx, err := config.GetContext(name)
		if err != nil {
			return err
		}

		current := config.GetCurrentContextName()
		marker := ""
		if ctx.Name == current {
			marker = " (当前默认)"
		}

		fmt.Printf("名称:   %s%s\n", ctx.Name, marker)
		fmt.Printf("类型:   %s\n", ctx.Type)
		fmt.Printf("主机:   %s\n", ctx.Host)
		fmt.Printf("端口:   %d\n", ctx.Port)
		fmt.Printf("用户:   %s\n", ctx.Username)
		fmt.Printf("密码:   %s\n", maskPassword(ctx.Password))
		fmt.Printf("数据库: %s\n", ctx.Database)
		if len(ctx.Options) > 0 {
			fmt.Printf("选项:\n")
			keys := make([]string, 0, len(ctx.Options))
			for k := range ctx.Options {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("  %s = %s\n", k, ctx.Options[k])
			}
		}

		return nil
	},
}

func init() {
	// connect add 参数
	connectAddCmd.Flags().StringP("name", "n", "", "连接名称 (必填)")
	connectAddCmd.Flags().StringP("type", "t", "", "数据库类型 (必填，可用 'sqlo databases' 查看支持的类型)")
	connectAddCmd.Flags().StringP("host", "H", "", "主机地址 (必填)")
	connectAddCmd.Flags().IntP("port", "P", 0, "端口 (默认根据类型自动推断)")
	connectAddCmd.Flags().StringP("username", "u", "", "用户名 (必填)")
	connectAddCmd.Flags().StringP("password", "p", "", "密码")
	connectAddCmd.Flags().StringP("database", "d", "", "默认数据库")
	connectAddCmd.Flags().String("options", "", "额外连接参数，格式: key1=value1,key2=value2")
	connectAddCmd.Flags().Bool("default", false, "设置为默认连接")

	// 注册子命令
	connectCmd.AddCommand(connectAddCmd)
	connectCmd.AddCommand(connectListCmd)
	connectCmd.AddCommand(connectUseCmd)
	connectCmd.AddCommand(connectDeleteCmd)
	connectCmd.AddCommand(connectShowCmd)

	// 注册到根命令
	rootCmd.AddCommand(connectCmd)
}

// getDefaultPort 根据数据库类型返回默认端口
func getDefaultPort(dbType string) int {
	ports := map[string]int{
		"postgres":  5432,
		"mysql":     3306,
		"sqlserver": 1433,
		"oracle":    1521,
		"sqlite":     0, // 文件型数据库
		"access":     0, // 文件型数据库
		"clickhouse": 9000, // 网络型数据库
		"tdengine":   6041, // 网络型数据库（WebSocket via taosAdapter）
		"dameng":     5236, // 网络型数据库（达梦 DM，纯 Go 官方驱动）
		"kingbase":   54321, // 网络型数据库（人大金仓 Kingbase ES，纯 Go 官方驱动 gokb）
	}

	if port, ok := ports[dbType]; ok {
		return port
	}
	return 0
}

// maskPassword 密码脱敏显示
func maskPassword(password string) string {
	if password == "" {
		return "(空)"
	}
	if len(password) <= 2 {
		return "***"
	}
	return password[:1] + "***" + password[len(password)-1:]
}
