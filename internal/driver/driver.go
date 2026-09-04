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
