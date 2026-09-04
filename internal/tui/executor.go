package tui

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/liang-junwei/sqlo/internal/output"
)

// maxRows 单次查询最多保留的行数。
// 交互会话可能反复执行大查询，若不加保护会把结果集全部堆进内存。
const maxRows = 1000

// outcome 一次执行的完整结果
type outcome struct {
	result    *output.QueryResult
	total     int // 实际扫描到的总行数（截断时 > len(result.Rows)）
	truncated bool
	elapsed   time.Duration
	err       error
}

// isQuery 判断语句是否返回结果集。
// 与 cmd/exec.go 的判断口径保持一致，但这里需要 context 与行数截断，
// 因此单独实现而不复用 exec 的一次性输出逻辑。
func isQuery(sqlText string) bool {
	s := strings.ToLower(strings.TrimSpace(sqlText))
	return strings.HasPrefix(s, "select") ||
		strings.HasPrefix(s, "show") ||
		strings.HasPrefix(s, "describe") ||
		strings.HasPrefix(s, "desc ") ||
		strings.HasPrefix(s, "explain") ||
		strings.HasPrefix(s, "with")
}

// runSQL 执行一条 SQL，返回统一结果
func runSQL(ctx context.Context, conn *sql.DB, text string) outcome {
	start := time.Now()

	if isQuery(text) {
		res, total, truncated, err := queryRows(ctx, conn, text)
		return outcome{
			result:    res,
			total:     total,
			truncated: truncated,
			elapsed:   time.Since(start),
			err:       err,
		}
	}

	res, err := execStmt(ctx, conn, text)
	return outcome{result: res, elapsed: time.Since(start), err: err}
}

// queryRows 执行查询并读取结果集，超过 maxRows 后停止保留但仍统计总数
func queryRows(ctx context.Context, conn *sql.DB, query string) (*output.QueryResult, int, bool, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, 0, false, err
	}

	result := &output.QueryResult{Columns: columns}
	total := 0
	truncated := false

	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, 0, false, err
		}

		total++
		if len(result.Rows) >= maxRows {
			// 已经取够，继续遍历只为统计真实总行数
			truncated = true
			continue
		}

		row := make([]interface{}, len(values))
		for i, v := range values {
			switch val := v.(type) {
			case []byte:
				row[i] = string(val)
			default:
				row[i] = val
			}
		}
		result.Rows = append(result.Rows, row)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}

	return result, total, truncated, nil
}

// execStmt 执行非查询语句（INSERT/UPDATE/DELETE/DDL 等）
func execStmt(ctx context.Context, conn *sql.DB, query string) (*output.QueryResult, error) {
	rs, err := conn.ExecContext(ctx, query)
	if err != nil {
		return nil, err
	}

	// 部分驱动不支持 RowsAffected，此时只报告成功
	msg := "执行成功"
	if affected, err := rs.RowsAffected(); err == nil {
		msg = fmt.Sprintf("执行成功，影响 %d 行", affected)
	}

	return &output.QueryResult{Message: msg}, nil
}

// runMetaQuery 执行元命令查询（\dt/\d 等），支持带参数
func runMetaQuery(ctx context.Context, conn *sql.DB, query string, args []interface{}) outcome {
	if len(args) == 0 {
		return runSQL(ctx, conn, query)
	}

	// 元数据查询一定是 SELECT，直接走带参数的查询路径
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return outcome{err: err}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return outcome{err: err}
	}

	res := &output.QueryResult{Columns: cols}
	total := 0
	truncated := false

	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return outcome{err: err}
		}
		total++
		if len(res.Rows) >= maxRows {
			truncated = true
			continue
		}
		row := make([]interface{}, len(vals))
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		res.Rows = append(res.Rows, row)
	}

	if err := rows.Err(); err != nil {
		return outcome{err: err}
	}

	return outcome{result: res, total: total, truncated: truncated}
}
