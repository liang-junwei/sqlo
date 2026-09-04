package cmd

import (
	"github.com/liang-junwei/sqlo/internal/config"
	"github.com/liang-junwei/sqlo/internal/db"
	"github.com/liang-junwei/sqlo/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "启动交互式 TUI 会话",
	Long: `在一个持续保持的数据库连接会话中交互式执行 SQL。

与 exec 不同，tui 只建立一次连接并在整个会话中保持，
因此临时表、未提交事务、会话变量等会话状态会跨语句保留。

示例:
  sqlo tui                             使用当前默认连接
  sqlo tui -S dm-dev                   使用已保存的连接（全局参数 -S）
  sqlo tui -t mysql -H 127.0.0.1 -u root -p pass   行内参数直连（无需先 connect add）
  sqlo tui -d other_db                 临时切换数据库
  sqlo tui --opt sslmode=disable

常用按键:
  Ctrl+X              执行当前 SQL
  Tab                 在编辑器与结果区之间切换焦点
  Ctrl+C              取消正在执行的查询；空闲时退出
  Esc                 退出

常用元命令:
  \dt                 列出表        \d <表>   查看表结构
  \l                  列出数据库    \c <名称> 切换连接
  \x                  扩展显示      \o         切换输出格式(table/json/csv/yaml)
  \?                  帮助`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, _ := cmd.Flags().GetString("database")
		optStrs, _ := cmd.Flags().GetStringSlice("opt")

		override := &db.Override{
			Database: database,
			Options:  parseOptions(optStrs),
		}

		// 行内参数直连：提供 -t 即视为直连（无需先 connect add 保存 context）
		if t, _ := cmd.Flags().GetString("type"); t != "" {
			host, _ := cmd.Flags().GetString("host")
			port, _ := cmd.Flags().GetInt("port")
			if port == 0 {
				port = getDefaultPort(t)
			}
			user, _ := cmd.Flags().GetString("username")
			pass, _ := cmd.Flags().GetString("password")

			ctx := &config.Context{
				Type:     t,
				Host:     host,
				Port:     port,
				Username: user,
				Password: pass,
				Database: database,
			}
			conn, err := db.ConnectContext(ctx, override)
			if err != nil {
				return err
			}
			defer conn.Close()

			info := tui.ConnInfo{
				Name:     "(行内直连)",
				Type:     ctx.Type,
				Host:     ctx.Host,
				Port:     ctx.Port,
				Username: ctx.Username,
				Database: ctx.Database,
			}
			return tui.Run(conn, info)
		}

		// 否则使用已保存的 context：全局 -S 指定名称，否则取配置中的默认连接
		serverName, err := GetServer()
		if err != nil {
			return err
		}

		conn, err := db.Connect(serverName, override)
		if err != nil {
			return err
		}
		defer conn.Close()

		return tui.Run(conn, connInfo(serverName, database))
	},
}

func init() {
	tuiCmd.Flags().StringP("database", "d", "", "临时切换数据库（不修改配置文件）")
	tuiCmd.Flags().StringSlice("opt", nil, "运行时临时覆盖连接参数 (key=value),可多次指定或逗号分隔")

	// 行内直连参数：提供 -t 即视为直连，无需先 connect add
	// 已保存的 context 用全局 -S 指定；与 -t 同时给出时以 -t 直连为准
	tuiCmd.Flags().StringP("type", "t", "", "行内直连：数据库类型（mysql/postgres/dameng/kingbase...）")
	tuiCmd.Flags().StringP("host", "H", "", "行内直连：主机地址")
	tuiCmd.Flags().IntP("port", "P", 0, "行内直连：端口（缺省取该类型默认端口）")
	tuiCmd.Flags().StringP("username", "u", "", "行内直连：用户名")
	tuiCmd.Flags().StringP("password", "p", "", "行内直连：密码")

	rootCmd.AddCommand(tuiCmd)
}

// connInfo 组装顶栏展示的连接信息。
// database 参数为空时回落到配置中的数据库名。
func connInfo(name, database string) tui.ConnInfo {
	info := tui.ConnInfo{Name: name}

	ctx, err := config.GetContext(name)
	if err != nil {
		return info
	}

	info.Type = ctx.Type
	info.Host = ctx.Host
	info.Port = ctx.Port
	info.Username = ctx.Username
	info.Database = ctx.Database
	if database != "" {
		info.Database = database
	}

	return info
}
