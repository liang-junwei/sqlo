package tui

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"testing"

	// 触发各驱动的 init() 自注册，使 driver.Get 可用
	_ "github.com/liang-junwei/sqlo/internal/driver/access"
	_ "github.com/liang-junwei/sqlo/internal/driver/clickhouse"
	_ "github.com/liang-junwei/sqlo/internal/driver/dm"
	_ "github.com/liang-junwei/sqlo/internal/driver/kingbase"
	_ "github.com/liang-junwei/sqlo/internal/driver/mysql"
	_ "github.com/liang-junwei/sqlo/internal/driver/oracle"
	_ "github.com/liang-junwei/sqlo/internal/driver/postgres"
	_ "github.com/liang-junwei/sqlo/internal/driver/sqlserver"
	_ "github.com/liang-junwei/sqlo/internal/driver/sqlite"
	_ "github.com/liang-junwei/sqlo/internal/driver/tdengine"
)

// TestDamengDescribeOwnerAware 锁定 Bug3 修复：dameng 的 \d <schema>.<表>
// 必须把 schema 作为 owner 一并传入过滤，否则只按表名查会在多 owner 下返回
// 重复行（表现为每个字段重复 N 次）。
func TestDamengDescribeOwnerAware(t *testing.T) {
	act, err := parseMeta(`\d SYSDBA.T1`, "dameng", "")
	if err != nil {
		t.Fatalf("parseMeta 失败: %v", err)
	}
	if act.kind != metaQuery {
		t.Fatalf("kind=%v 期望 metaQuery", act.kind)
	}
	// 期望 args = [owner, table] = [SYSDBA, T1]
	if len(act.args) != 2 || act.args[0] != "SYSDBA" || act.args[1] != "T1" {
		t.Errorf("dameng \\d SYSDBA.T1 应传 [SYSDBA T1]，实际 %v", act.args)
	}
	// 期望 SQL 注入了 owner 过滤
	if !contains(act.query, "owner = ? AND table_name = ?") {
		t.Errorf("dameng \\d 的 SQL 应注入 owner 过滤，实际: %s", act.query)
	}
}

// TestDamengDescribeNoSchemaUnchanged 确保不带 schema 时行为不变（仍按表名查）。
func TestDamengDescribeNoSchemaUnchanged(t *testing.T) {
	act, err := parseMeta(`\d T1`, "dameng", "")
	if err != nil {
		t.Fatalf("parseMeta 失败: %v", err)
	}
	if len(act.args) != 1 || act.args[0] != "T1" {
		t.Errorf("dameng \\d T1 应传 [T1]，实际 %v", act.args)
	}
	if contains(act.query, "owner = ?") {
		t.Errorf("dameng \\d T1（无 schema）不应注入 owner 过滤，实际: %s", act.query)
	}
}

// TestOtherOneArgDriversUnchanged sqlite/tdengine/access 保持单参数，不被 owner 逻辑影响。
func TestOtherOneArgDriversUnchanged(t *testing.T) {
	for _, dt := range []string{"sqlite", "tdengine", "access"} {
		act, err := parseMeta(`\d main.users`, dt, "")
		if err != nil {
			t.Fatalf("%s parseMeta 失败: %v", dt, err)
		}
		if len(act.args) != 1 {
			t.Errorf("%s \\d 应只传 1 个参数，实际 %v", dt, act.args)
		}
	}
}

// TestTwoArgDriversPassSchemaTable 2 参数驱动（postgres 等）必须 (schema, table)。
func TestTwoArgDriversPassSchemaTable(t *testing.T) {
	for _, dt := range []string{"postgres", "kingbase", "mysql", "clickhouse", "sqlserver", "oracle"} {
		act, err := parseMeta(`\d public.users`, dt, "")
		if err != nil {
			t.Fatalf("%s parseMeta 失败: %v", dt, err)
		}
		if len(act.args) != 2 || act.args[0] != "public" || act.args[1] != "users" {
			t.Errorf("%s \\d public.users 应传 [public users]，实际 %v", dt, act.args)
		}
	}
}

// ---- 渲染层不翻倍：用 mock 驱动返回固定 3 行，验证 applyOutcome 后表格行数 == 3 ----

func TestRenderDoesNotTriplicateRows(t *testing.T) {
	sql.Register("describereg", &describeMockDriver{})
	db, err := sql.Open("describereg", "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	o := runMetaQuery(ctx, db, "SELECT ... FROM t WHERE owner=? AND name=?", []interface{}{"s", "t"})
	if o.err != nil {
		t.Fatalf("runMetaQuery 失败: %v", o.err)
	}
	if len(o.result.Rows) != 3 {
		t.Fatalf("mock 驱动应返回 3 行，实际 %d", len(o.result.Rows))
	}

	m := newModel(db, ConnInfo{Type: "postgres"})
	m.applyOutcome(o)

	if got := len(m.table.Rows()); got != 3 {
		t.Errorf("BUG3 回归：驱动返回 3 行，但表格出现 %d 行（被放大 %.0f 倍）", got, float64(got)/3)
	}
	// 水平滚动引入列窗口后，表格 Columns() 是「可见窗口」而非完整列；
	// 完整列数应断言在数据源 resCols 上。
	if len(m.resCols) != 4 {
		t.Errorf("结果集列数异常 = %d，期望 4", len(m.resCols))
	}
	if len(m.table.Columns()) < 1 {
		t.Errorf("可见列窗口为空（width=%d）", m.width)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type describeMockDriver struct{}

func (describeMockDriver) Open(name string) (driver.Conn, error) { return describeMockConn{}, nil }

type describeMockConn struct{}

func (describeMockConn) Prepare(query string) (driver.Stmt, error) { return &describeMockStmt{}, nil }
func (describeMockConn) Close() error                             { return nil }
func (describeMockConn) Begin() (driver.Tx, error)                { return nil, driver.ErrSkip }
func (describeMockConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return &describeMockRows{cur: -1}, nil
}

type describeMockStmt struct{}

func (describeMockStmt) Close() error                       { return nil }
func (describeMockStmt) NumInput() int                     { return -1 }
func (describeMockStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, driver.ErrSkip
}
func (describeMockStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &describeMockRows{cur: -1}, nil
}

type describeMockRows struct{ cur int }

func (r *describeMockRows) Columns() []string {
	return []string{"column_name", "data_type", "is_nullable", "column_default"}
}
func (r *describeMockRows) Close() error { return nil }
func (r *describeMockRows) Next(dest []driver.Value) error {
	r.cur++
	if r.cur >= 3 {
		return io.EOF
	}
	for i := range dest {
		dest[i] = driver.Value("")
	}
	dest[0] = driver.Value("col" + string(rune('A'+r.cur)))
	dest[1] = driver.Value("int")
	dest[2] = driver.Value("NO")
	dest[3] = driver.Value(nil)
	return nil
}

// TestShowTextDoesNotBlockEditorInput 已随极简重写移除：
// v3 形态没有 viewport/showText 组件，帮助文本只是普通字符串，
// 编辑器聚焦时按键天然直达编辑器（v3 Update 的兜底分支）。
