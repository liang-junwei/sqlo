package config

import (
	"fmt"
)

// AddContext 添加新的 context
func AddContext(ctx *Context, setDefault bool) error {
	mu.Lock()
	defer mu.Unlock()

	if globalConfig == nil {
		return fmt.Errorf("config: not loaded")
	}

	if ctx.Name == "" {
		return fmt.Errorf("config: context name is required")
	}

	if _, exists := globalConfig.Contexts[ctx.Name]; exists {
		return fmt.Errorf("config: context %q already exists", ctx.Name)
	}

	globalConfig.Contexts[ctx.Name] = ctx

	// 如果是第一个 context 或明确设置为默认
	if len(globalConfig.Contexts) == 1 || setDefault {
		globalConfig.CurrentContext = ctx.Name
	}

	return saveLocked()
}

// DeleteContext 删除 context
func DeleteContext(name string) error {
	mu.Lock()
	defer mu.Unlock()

	if globalConfig == nil {
		return fmt.Errorf("config: not loaded")
	}

	if _, exists := globalConfig.Contexts[name]; !exists {
		return fmt.Errorf("config: context %q not found", name)
	}

	delete(globalConfig.Contexts, name)

	// 如果删除的是当前默认 context，清空
	if globalConfig.CurrentContext == name {
		globalConfig.CurrentContext = ""
		// 如果还有其他 context，选第一个作为默认
		for n := range globalConfig.Contexts {
			globalConfig.CurrentContext = n
			break
		}
	}

	return saveLocked()
}

// UseContext 切换默认 context
func UseContext(name string) error {
	mu.Lock()
	defer mu.Unlock()

	if globalConfig == nil {
		return fmt.Errorf("config: not loaded")
	}

	if _, exists := globalConfig.Contexts[name]; !exists {
		return fmt.Errorf("config: context %q not found", name)
	}

	globalConfig.CurrentContext = name
	return saveLocked()
}

// GetContext 获取指定 context，如果 name 为空则返回当前默认
func GetContext(name string) (*Context, error) {
	mu.RLock()
	defer mu.RUnlock()

	if globalConfig == nil {
		return nil, fmt.Errorf("config: not loaded")
	}

	// 如果未指定名称，使用当前默认
	if name == "" {
		name = globalConfig.CurrentContext
	}

	if name == "" {
		return nil, fmt.Errorf("config: no context selected (use 'connect use' to set default)")
	}

	ctx, exists := globalConfig.Contexts[name]
	if !exists {
		return nil, fmt.Errorf("config: context %q not found", name)
	}

	return ctx, nil
}

// ListContexts 返回所有 context 名称列表
func ListContexts() []string {
	mu.RLock()
	defer mu.RUnlock()

	if globalConfig == nil {
		return nil
	}

	names := make([]string, 0, len(globalConfig.Contexts))
	for name := range globalConfig.Contexts {
		names = append(names, name)
	}
	return names
}

// GetCurrentContextName 返回当前默认 context 名称
func GetCurrentContextName() string {
	mu.RLock()
	defer mu.RUnlock()

	if globalConfig == nil {
		return ""
	}
	return globalConfig.CurrentContext
}

// UpdateContext 更新已有 context 的字段
func UpdateContext(name string, updates map[string]interface{}) error {
	mu.Lock()
	defer mu.Unlock()

	if globalConfig == nil {
		return fmt.Errorf("config: not loaded")
	}

	ctx, exists := globalConfig.Contexts[name]
	if !exists {
		return fmt.Errorf("config: context %q not found", name)
	}

	// 应用更新
	if v, ok := updates["type"]; ok {
		ctx.Type = v.(string)
	}
	if v, ok := updates["host"]; ok {
		ctx.Host = v.(string)
	}
	if v, ok := updates["port"]; ok {
		ctx.Port = v.(int)
	}
	if v, ok := updates["username"]; ok {
		ctx.Username = v.(string)
	}
	if v, ok := updates["password"]; ok {
		ctx.Password = v.(string)
	}
	if v, ok := updates["database"]; ok {
		ctx.Database = v.(string)
	}
	if v, ok := updates["options"]; ok {
		if opts, isMap := v.(map[string]string); isMap {
			ctx.Options = opts
		}
	}

	return saveLocked()
}
