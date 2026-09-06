package driver

import (
	"database/sql"
	"fmt"
	"sync"
)

// Driver 定义数据库驱动的能力边界
type Driver struct {
	// 驱动名称，用于 --type 参数
	Name string

	// 默认端口
	DefaultPort int

	// 元数据查询模板（可选，用于 \dt 等命令）
	Metadata MetadataQueries

	// 特性标志
	Features Features
}

// MetadataQueries 存储常用元数据 SQL
type MetadataQueries struct {
	ListTables   string
	ListSchemas  string
	ListDatabases string
	DescribeTable string

	// DescribeParamCount 声明 DescribeTable 需要的参数个数：
	// 1 = 只需 table；2 = 需要 (schema, table)。
	// 各驱动的占位符风格与参数个数并不统一（$1 $2 / ? ? / @p1 @p2 / :1 :2），
	// 这里只声明个数，具体占位符写在 SQL 模板里。
	DescribeParamCount int

	// ListTablesParamCount 声明 ListTables 需要的参数个数：
	// 0 = 无需参数（大多数驱动）；1 = 需要 1 个参数。
	// 仅 dameng（owner=?）与 tdengine（db_name=?）为 1，参数含义见 BuildListTables。
	ListTablesParamCount int
}

// Features 声明驱动支持的特性
type Features struct {
	// 是否支持多语句事务
	Transactions bool

	// 是否支持 RETURNING 子句
	Returning bool

	// 是否支持 LIMIT/OFFSET
	LimitOffset bool
}

var (
	registry = make(map[string]*Driver)
	mu       sync.RWMutex
)

// Register 注册驱动，由 init() 调用
func Register(name string, drv *Driver) {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("driver: %s already registered", name))
	}
	registry[name] = drv
}

// Get 获取已注册驱动
func Get(name string) (*Driver, error) {
	mu.RLock()
	defer mu.RUnlock()

	drv, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("driver: %s not registered", name)
	}
	return drv, nil
}

// List 返回所有已注册驱动名称
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// Open 使用驱动名称和 DSN 打开数据库连接
func Open(driverName, dsn string) (*sql.DB, error) {
	drv, err := Get(driverName)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(drv.Name, dsn)
	if err != nil {
		return nil, fmt.Errorf("driver: open %s failed: %w", driverName, err)
	}

	// 验证连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("driver: ping %s failed: %w", driverName, err)
	}

	return db, nil
}
