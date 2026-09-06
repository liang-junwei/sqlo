package driver_test

import (
	"testing"

	"github.com/liang-junwei/sqlo/internal/driver"

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

// TestBuildListTablesDameng owner=? 需要 1 个参数，取当前连接用户名。
func TestBuildListTablesDameng(t *testing.T) {
	q, args, err := driver.BuildListTables("dameng", "", "SYSDBA")
	if err != nil {
		t.Fatalf("dameng: %v", err)
	}
	if len(args) != 1 || args[0] != "SYSDBA" {
		t.Errorf("dameng args = %v, want [SYSDBA]", args)
	}
	if q == "" || !containsStr(q, "owner = ?") {
		t.Errorf("dameng query 应含 owner = ?，实际: %s", q)
	}
}

// TestBuildListTablesTdengine db_name=? 需要 1 个参数，取当前数据库。
func TestBuildListTablesTdengine(t *testing.T) {
	q, args, err := driver.BuildListTables("tdengine", "testdb", "")
	if err != nil {
		t.Fatalf("tdengine: %v", err)
	}
	if len(args) != 1 || args[0] != "testdb" {
		t.Errorf("tdengine args = %v, want [testdb]", args)
	}
	if q == "" || !containsStr(q, "db_name = ?") {
		t.Errorf("tdengine query 应含 db_name = ?，实际: %s", q)
	}
}

// TestBuildListTablesDamengNeedsUser dameng 缺用户名应报错。
func TestBuildListTablesDamengNeedsUser(t *testing.T) {
	if _, _, err := driver.BuildListTables("dameng", "", ""); err == nil {
		t.Fatal("dameng 无用户名时应当报错")
	}
}

// TestBuildListTablesTdengineNeedsDB tdengine 缺数据库名应报错。
func TestBuildListTablesTdengineNeedsDB(t *testing.T) {
	if _, _, err := driver.BuildListTables("tdengine", "", ""); err == nil {
		t.Fatal("tdengine 无数据库名时应当报错")
	}
}

// TestBuildListTablesNoParamDrivers 无占位符驱动不应注入参数（行为不变）。
func TestBuildListTablesNoParamDrivers(t *testing.T) {
	for _, dt := range []string{"postgres", "mysql", "sqlite", "clickhouse", "sqlserver", "oracle", "kingbase", "access"} {
		q, args, err := driver.BuildListTables(dt, "db", "u")
		if err != nil {
			t.Fatalf("%s: %v", dt, err)
		}
		if len(args) != 0 {
			t.Errorf("%s 不应有参数，实际 %v", dt, args)
		}
		if q == "" {
			t.Errorf("%s query 为空", dt)
		}
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
