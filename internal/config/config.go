package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	AppName    = "sqlo"
	ConfigFile = ".sqlo.json"
)

// Context 表示一个数据库连接配置
type Context struct {
	Name     string            `json:"name"`     // server_name
	Type     string            `json:"type"`     // 驱动类型：postgres/mysql/sqlserver/oracle
	Host     string            `json:"host"`     // 主机地址
	Port     int               `json:"port"`     // 端口
	Username string            `json:"username"` // 用户名
	Password string            `json:"password"` // 密码（明文，后续考虑加密）
	Database string            `json:"database"` // 默认数据库
	Options  map[string]string `json:"options"`  // 额外连接参数（key-value）
}

// Config 存储所有配置
type Config struct {
	CurrentContext string              `json:"current_context"` // 当前默认 context 名称
	Contexts       map[string]*Context `json:"contexts"`        // context 映射
}

var (
	globalConfig *Config
	configPath   string
	mu           sync.RWMutex
)

// GetConfigPath 返回配置文件完整路径
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: get home dir failed: %w", err)
	}
	return filepath.Join(home, ConfigFile), nil
}

// Load 从文件加载配置（线程安全）
func Load() error {
	mu.Lock()
	defer mu.Unlock()

	path, err := GetConfigPath()
	if err != nil {
		return err
	}
	configPath = path

	// 文件不存在则返回空配置
	if _, err := os.Stat(path); os.IsNotExist(err) {
		globalConfig = &Config{
			Contexts: make(map[string]*Context),
		}
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: read %s failed: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("config: parse %s failed: %w", path, err)
	}

	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]*Context)
	}

	globalConfig = &cfg
	return nil
}

// saveLocked 保存配置到文件（调用者必须已持有 mu 锁）
func saveLocked() error {
	data, err := json.MarshalIndent(globalConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal failed: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("config: write %s failed: %w", configPath, err)
	}

	return nil
}

// Get 获取当前配置（只读，线程安全）
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig
}
