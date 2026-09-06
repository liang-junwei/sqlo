package cmd

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/liang-junwei/sqlo/internal/config"
	"github.com/liang-junwei/sqlo/internal/db"
	"github.com/liang-junwei/sqlo/internal/output"
)

// connectForMeta 建立连接并一并返回 context。
// 内省命令需要 context 里的 Type（选驱动元数据 SQL）与 Database（作为默认 schema）。
func connectForMeta(databaseOverride string) (*sql.DB, *config.Context, error) {
	name, err := GetServer()
	if err != nil {
		return nil, nil, err
	}

	ctx, err := config.GetContext(name)
	if err != nil {
		return nil, nil, err
	}

	override := &db.Override{}
	if databaseOverride != "" {
		override.Database = databaseOverride
	}

	conn, err := db.ConnectContext(ctx, override)
	if err != nil {
		return nil, nil, err
	}

	return conn, ctx, nil
}

// runMetaQuery 执行（可能带参数的）元查询并按当前输出格式输出。
// 探查命令（tables/describe）不做行数限制——结果集一般很小，且限制会漏掉表/列。
func runMetaQuery(conn *sql.DB, query string, args []interface{}) error {
	var rows *sql.Rows
	var err error
	if len(args) == 0 {
		rows, err = conn.Query(query)
	} else {
		rows, err = conn.Query(query, args...)
	}
	if err != nil {
		return fmt.Errorf("执行查询失败: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("获取列信息失败: %w", err)
	}

	resultRows := make([][]interface{}, 0)

	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return fmt.Errorf("扫描行数据失败: %w", err)
		}

		row := make([]interface{}, len(values))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历结果失败: %w", err)
	}

	formatter, err := output.GetFormatter(GetOutputFormat())
	if err != nil {
		return err
	}
	return formatter.Format(os.Stdout, &output.QueryResult{Columns: columns, Rows: resultRows})
}

// warnTruncated 输出截断提示到 stderr，保持 stdout 可被程序解析。
func warnTruncated(maxRows int, truncated bool) {
	if truncated {
		fmt.Fprintf(os.Stderr, "提示: 结果超过 %d 行已截断，完整输出请加 --row-limit 0\n", maxRows)
	}
}
