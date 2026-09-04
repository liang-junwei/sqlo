package tui

import (
	"testing"

	"github.com/liang-junwei/sqlo/internal/driver"
)

// TestPinSingleConn 锁定回归：TUI 的连接池必须固定为单连接。
//
// 背景：USE db / SET search_path 是会话级语句，只对执行它的那条物理连接生效。
// 连接池一旦不限，后续查询就可能落在没切过库的连接上，表现为「切了库但查到的
// 还是旧数据」，随机复现。TUI 的会话态（临时表/未提交事务/会话变量）同样绑定
// 单条物理连接，本来就要求所有语句走同一条。
func TestPinSingleConn(t *testing.T) {
	conn, err := driver.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatalf("打开 sqlite 内存库失败: %v", err)
	}
	defer conn.Close()

	// 前置条件：driver.Open 当前不设连接池（Go 默认 0 = 不限）。
	// 若将来驱动层自行加了连接池设置，这里会失败，提示重新审视本用例的必要性。
	if got := conn.Stats().MaxOpenConnections; got != 0 {
		t.Errorf("driver.Open 后的 MaxOpenConnections = %d，期望 0（不限）；驱动层若已改动请同步调整本用例", got)
	}

	pinSingleConn(conn)
	if got := conn.Stats().MaxOpenConnections; got != 1 {
		t.Errorf("pinSingleConn 后 MaxOpenConnections = %d，期望 1", got)
	}

	// nil 安全：不应 panic
	pinSingleConn(nil)
}
