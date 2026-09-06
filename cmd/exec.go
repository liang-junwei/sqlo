package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/liang-junwei/sqlo/internal/db"
	"github.com/liang-junwei/sqlo/internal/output"
	"github.com/spf13/cobra"
)

// execRowLimit 结果集最大返回行数（仅 exec 命令支持，0 表示无限制）
var execRowLimit int

var execCmd = &cobra.Command{
	Use:   "exec [sql]",
	Short: "执行 SQL 语句",
	Long: `执行 SQL 语句并输出结果。

示例:
  sqlo exec -e "SELECT * FROM users LIMIT 10"
  sqlo exec -e "SELECT * FROM users" -o json
  sqlo exec -f query.sql
  sqlo exec -S dev-mysql -e "SHOW DATABASES"
  sqlo exec -e "INSERT INTO users (name) VALUES ('alice')"
  sqlo exec -d other_db -e "SELECT * FROM users"
  sqlo exec --opt sslmode=disable -e "SELECT 1"
  sqlo exec --opt "timeout=30,sslmode=require" -e "SELECT 1"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		execute, _ := cmd.Flags().GetString("execute")
		file, _ := cmd.Flags().GetString("file")
		database, _ := cmd.Flags().GetString("database")
		optStrs, _ := cmd.Flags().GetStringSlice("opt")

		var sqlText string

		// 优先使用 -e 参数
		if execute != "" {
			sqlText = execute
		} else if file != "" {
			// 从文件读取
			content, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("读取文件失败: %w", err)
			}
			sqlText = string(content)
		} else if len(args) > 0 {
			// 位置参数
			sqlText = strings.Join(args, " ")
		} else {
			return fmt.Errorf("请提供 SQL 语句 (使用 -e 或 -f 或位置参数)")
		}

		// 构建运行时覆盖参数
		override := &db.Override{
			Database: database,
			Options:  parseOptions(optStrs),
		}

		// 获取目标 server
		serverName, err := GetServer()
		if err != nil {
			return err
		}

		// 连接数据库
		conn, err := db.Connect(serverName, override)
		if err != nil {
			return err
		}
		defer conn.Close()

		// 执行 SQL
		return executeSQL(conn, sqlText)
	},
}

func init() {
	execCmd.Flags().StringP("execute", "e", "", "直接执行 SQL 语句")
	execCmd.Flags().StringP("file", "f", "", "从文件读取 SQL 执行")
	execCmd.Flags().StringP("database", "d", "", "临时切换数据库（不修改配置文件）")
	execCmd.Flags().StringSlice("opt", nil, "运行时临时覆盖连接参数 (key=value),可多次指定或逗号分隔")
	execCmd.Flags().IntVar(&execRowLimit, "row-limit", 500, "结果集最大返回行数，0 表示无限制")

	rootCmd.AddCommand(execCmd)
}

// parseOptions 将 []string{"key1=val1", "key2=val2"} 解析为 map
func parseOptions(pairs []string) map[string]string {
	if len(pairs) == 0 {
		return nil
	}
	opts := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			opts[parts[0]] = parts[1]
		}
	}
	return opts
}

// executeSQL 执行 SQL 并根据类型格式化输出
func executeSQL(conn *sql.DB, sqlText string) error {
	// 判断是否为查询语句(SELECT/SHOW/DESCRIBE/EXPLAIN)
	sqlLower := strings.ToLower(strings.TrimSpace(sqlText))
	isQuery := strings.HasPrefix(sqlLower, "select") ||
		strings.HasPrefix(sqlLower, "show") ||
		strings.HasPrefix(sqlLower, "describe") ||
		strings.HasPrefix(sqlLower, "explain") ||
		strings.HasPrefix(sqlLower, "with")

	if isQuery {
		return executeQueryWithResult(conn, sqlText)
	}

	// 非查询语句(INSERT/UPDATE/DELETE/CREATE 等)
	return executeExec(conn, sqlText)
}

// executeQueryWithResult 执行查询并返回结果集
func executeQueryWithResult(conn *sql.DB, sqlText string) error {
	rows, err := conn.Query(sqlText)
	if err != nil {
		return fmt.Errorf("执行查询失败: %w", err)
	}
	defer rows.Close()

	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("获取列信息失败: %w", err)
	}

	// 读取所有行，超过 --row-limit 时提前中止，避免大结果集全量拉取
	maxRows := execRowLimit
	var resultRows [][]interface{}
	truncated := false
	for rows.Next() {
		// 已取满则停止扫描（此时不再 Scan 该行）
		if maxRows > 0 && len(resultRows) >= maxRows {
			truncated = true
			break
		}

		// 创建值切片
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("扫描行数据失败: %w", err)
		}

		// 转换为 interface{} 切片
		row := make([]interface{}, len(columns))
		for i, v := range values {
			switch val := v.(type) {
			case []byte:
				row[i] = string(val)
			default:
				row[i] = val
			}
		}

		resultRows = append(resultRows, row)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历结果失败: %w", err)
	}

	// 构建结果
	result := &output.QueryResult{
		Columns: columns,
		Rows:    resultRows,
	}

	// 获取格式化器
	format := GetOutputFormat()
	formatter, err := output.GetFormatter(format)
	if err != nil {
		return err
	}

	// 输出结果
	if err := formatter.Format(os.Stdout, result); err != nil {
		return err
	}

	warnTruncated(maxRows, truncated)
	return nil
}

// executeExec 执行非查询语句
func executeExec(conn *sql.DB, sqlText string) error {
	result, err := conn.Exec(sqlText)
	if err != nil {
		return fmt.Errorf("执行失败: %w", err)
	}

	// 获取影响行数
	affected, err := result.RowsAffected()
	if err != nil {
		// 某些驱动不支持 RowsAffected,忽略错误
		fmt.Println("✓ 执行成功")
		return nil
	}

	fmt.Printf("✓ 执行成功,影响 %d 行\n", affected)
	return nil
}
