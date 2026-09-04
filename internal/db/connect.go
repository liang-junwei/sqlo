package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/liang-junwei/sqlo/internal/config"
	"github.com/liang-junwei/sqlo/internal/driver"
)

// Override 运行时覆盖参数（不修改持久化配置）
type Override struct {
	Database string            // 覆盖默认数据库（空字符串表示不覆盖）
	Options  map[string]string // 覆盖/追加连接选项
}

// Connect 根据 context 名称创建数据库连接
func Connect(contextName string, override *Override) (*sql.DB, error) {
	ctx, err := config.GetContext(contextName)
	if err != nil {
		return nil, err
	}

	// 验证驱动是否已注册
	_, err = driver.Get(ctx.Type)
	if err != nil {
		return nil, fmt.Errorf("驱动 %s 未注册，请检查是否已导入", ctx.Type)
	}

	// 确定最终使用的 database
	database := ctx.Database
	if override != nil && override.Database != "" {
		database = override.Database
	}

	// 合并 options：配置 options + 运行时覆盖
	var runtimeOpts map[string]string
	if override != nil {
		runtimeOpts = override.Options
	}
	mergedOpts := mergeOptions(ctx.Options, runtimeOpts)

	// 用合并后的参数构建 DSN（不修改原始 ctx）
	dsn, err := buildDSNWithOpts(ctx.Type, ctx.Username, ctx.Password,
		ctx.Host, ctx.Port, database, mergedOpts)
	if err != nil {
		return nil, fmt.Errorf("构建连接字符串失败: %w", err)
	}

	// 打开连接
	conn, err := driver.Open(ctx.Type, dsn)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	return conn, nil
}

// ConnectContext 使用给定的连接配置创建数据库连接。
// 与 Connect 的区别：不依赖已持久化的 context 名称，直接吃一个 *config.Context，
// 用于 TUI 行内参数直连（-t/-H/-P/-u/-p）等无需先 connect add 的场景。
func ConnectContext(ctx *config.Context, override *Override) (*sql.DB, error) {
	// 验证驱动是否已注册
	if _, err := driver.Get(ctx.Type); err != nil {
		return nil, fmt.Errorf("驱动 %s 未注册，请检查是否已导入", ctx.Type)
	}

	// 确定最终使用的 database
	database := ctx.Database
	if override != nil && override.Database != "" {
		database = override.Database
	}

	// 合并 options：配置 options + 运行时覆盖
	mergedOpts := mergeOptions(ctx.Options, overrideOptions(override))

	// 用合并后的参数构建 DSN（不修改原始 ctx）
	dsn, err := buildDSNWithOpts(ctx.Type, ctx.Username, ctx.Password,
		ctx.Host, ctx.Port, database, mergedOpts)
	if err != nil {
		return nil, fmt.Errorf("构建连接字符串失败: %w", err)
	}

	// 打开连接
	conn, err := driver.Open(ctx.Type, dsn)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	return conn, nil
}

// overrideOptions 从 Override 中取出运行时覆盖参数（nil 安全）
func overrideOptions(override *Override) map[string]string {
	if override == nil {
		return nil
	}
	return override.Options
}

// mergeOptions 合并配置 options 和运行时覆盖
// 优先级：runtime > base
func mergeOptions(base, runtime map[string]string) map[string]string {
	if len(base) == 0 && len(runtime) == 0 {
		return nil
	}

	merged := make(map[string]string, len(base)+len(runtime))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range runtime {
		merged[k] = v
	}
	return merged
}

// buildDSNWithOpts 根据参数构建 DSN 连接字符串
func buildDSNWithOpts(dbType, username, password, host string, port int, database string, opts map[string]string) (string, error) {
	switch dbType {
	case "postgres":
		return buildPostgresDSN(username, password, host, port, database, opts), nil
	case "mysql":
		return buildMySQLDSN(username, password, host, port, database, opts), nil
	case "sqlserver":
		return buildSQLServerDSN(username, password, host, port, database, opts), nil
	case "oracle":
		return buildOracleDSN(username, password, host, port, database, opts), nil
	case "clickhouse":
		return buildClickHouseDSN(username, password, host, port, database, opts), nil
	case "tdengine":
		return buildTDengineDSN(username, password, host, port, database, opts), nil
	case "dameng":
		return buildDMDSN(username, password, host, port, database, opts), nil
	case "kingbase":
		return buildKingbaseDSN(username, password, host, port, database, opts), nil
	case "sqlite":
		return buildSQLiteDSN(database, opts), nil
	case "access":
		return buildAccessDSN(database, opts), nil
	default:
		return "", fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
}

// buildOptionsQuery 将 map 类型的 options 编码为 URL query string
// 返回形如 "key1=value1&key2=value2" 的字符串，空 map 返回 ""
func buildOptionsQuery(opts map[string]string) string {
	if len(opts) == 0 {
		return ""
	}

	// 排序保证输出确定性（便于测试和调试）
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s",
			url.QueryEscape(k), url.QueryEscape(opts[k])))
	}
	return strings.Join(pairs, "&")
}

// buildPostgresDSN 构建 PostgreSQL DSN
// 格式: postgres://user:password@host:port/database?key=value&...
func buildPostgresDSN(username, password, host string, port int, database string, opts map[string]string) string {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		username, password, host, port, database)

	query := buildOptionsQuery(opts)
	if query != "" {
		dsn += "?" + query
	}
	return dsn
}

// buildMySQLDSN 构建 MySQL DSN
// 格式: user:password@tcp(host:port)/database?params
func buildMySQLDSN(username, password, host string, port int, database string, opts map[string]string) string {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		username, password, host, port, database)

	query := buildOptionsQuery(opts)
	if query != "" {
		dsn += "?" + query
	}
	return dsn
}

// buildSQLServerDSN 构建 SQL Server DSN
// 格式: sqlserver://user:password@host:port?database=xxx&key=value&...
func buildSQLServerDSN(username, password, host string, port int, database string, opts map[string]string) string {
	// database 参数必须放第一个，后续 options 追加
	dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
		username, password, host, port, database)

	query := buildOptionsQuery(opts)
	if query != "" {
		dsn += "&" + query
	}
	return dsn
}

// buildOracleDSN 构建 Oracle DSN
// 格式: oracle://user:password@host:port/service_name?key=value&...
func buildOracleDSN(username, password, host string, port int, database string, opts map[string]string) string {
	dsn := fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
		username, password, host, port, database)

	query := buildOptionsQuery(opts)
	if query != "" {
		dsn += "?" + query
	}
	return dsn
}

// buildClickHouseDSN 构建 ClickHouse DSN
// 格式: clickhouse://user:password@host:9000/default?key=value&...
func buildClickHouseDSN(username, password, host string, port int, database string, opts map[string]string) string {
	dsn := fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s",
		username, password, host, port, database)

	query := buildOptionsQuery(opts)
	if query != "" {
		dsn += "?" + query
	}
	return dsn
}

// buildTDengineDSN 构建 TDengine WebSocket DSN
// 格式: user:password@ws(host:6041)/dbName?key=value&...
// taosWS 驱动解析规则: [user[:password]@][net[(addr)]]/dbname[?param]
func buildTDengineDSN(username, password, host string, port int, database string, opts map[string]string) string {
	dsn := fmt.Sprintf("%s:%s@ws(%s:%d)/%s",
		username, password, host, port, database)

	query := buildOptionsQuery(opts)
	if query != "" {
		dsn += "?" + query
	}
	return dsn
}

// buildDMDSN 构建达梦(DM) DSN
// 格式: dm://user:password@host:port/dbname?key=value&...
// 达梦官方 Go 驱动解析规则(n.go parseDSN): 必须以 dm:// 开头，
// user:password@host:port，dbname 用 /dbname 形式，query 参数追加 ?key=value
// 注意: 用户名/密码必须【原样】写入，不能 url.QueryEscape。
// 原因: 官方 parseDSN 提取 password 后并不做 QueryUnescape（全驱动无 QueryUnescape 调用），
// 若此处转义，含 @ / : 等特殊字符的密码会被静默改坏（如 Dameng@123 → Dameng%40123），
// 触发服务端 -2501 用户名或密码错误。原样写入时，parseDSN 用 LastIndex("@") 定位 host 分隔符、
// 用 SplitN(":",2) 取 user:pass，可正确还原含 @ 的密码。
func buildDMDSN(username, password, host string, port int, database string, opts map[string]string) string {
	var buf strings.Builder
	buf.WriteString("dm://")
	if username != "" {
		buf.WriteString(username)
		if password != "" {
			buf.WriteByte(':')
			buf.WriteString(password)
		}
		buf.WriteByte('@')
	}
	buf.WriteString(fmt.Sprintf("%s:%d", host, port))
	if database != "" {
		buf.WriteByte('/')
		buf.WriteString(database)
	}
	query := buildOptionsQuery(opts)
	if query != "" {
		buf.WriteByte('?')
		buf.WriteString(query)
	}
	return buf.String()
}

// buildKingbaseDSN 构建 Kingbase(人大金仓) DSN
// 采用 gokb 原生 key=value 形式（非 kingbase:// URL 形式）。
// 格式: user=... password=... host=... port=... dbname=... sslmode=...
// 原因:
//  1. gokb 的 NewConnector(conn.go) 对以 kingbase:// 开头的串走 ParseURL（用 net/url 解析），
//     其中 @ 是 userinfo/host 分隔符，密码含 @ 会被【最后一个 @】切断 -> 密码静默损坏。
//     而 gokb 的 parseOpts 以空格为分隔符解析 key=value，password=Dameng@123 无歧义。
//  2. parseOpts 全程不做 QueryUnescape（已确认），故 user/password 必须【原样】写入，不能 url.QueryEscape。
//  3. 若值含空格/`=`/`'`/`\`，按 lib/pq 约定单引号包裹并转义内部 ' -> \' 与 \ -> \\（对应 parseOpts 的引号解析分支）。
func buildKingbaseDSN(username, password, host string, port int, database string, opts map[string]string) string {
	var buf strings.Builder

	writeKV := func(k, v string) {
		if buf.Len() > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(quoteIfNeeded(v))
	}

	if username != "" {
		writeKV("user", username)
	}
	if password != "" {
		writeKV("password", password)
	}
	writeKV("host", host)
	writeKV("port", fmt.Sprintf("%d", port))
	if database != "" {
		writeKV("dbname", database)
	}

	// 追加运行时/配置 options（空格分隔的 key=value，raw 值）
	if len(opts) > 0 {
		keys := make([]string, 0, len(opts))
		for k := range opts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			writeKV(k, opts[k])
		}
	}
	return buf.String()
}

// quoteIfNeeded 仅在值含空格、=、单引号或反斜杠时用单引号包裹，
// 并转义内部单引号与反斜杠，以契合 gokb parseOpts 的引号解析分支。
// 普通密码（如 Dameng@123）直接原样返回，不引入任何多余字符。
func quoteIfNeeded(v string) string {
	if !strings.ContainsAny(v, " ='\\") {
		return v
	}
	var b strings.Builder
	b.WriteByte('\'')
	for i := 0; i < len(v); i++ {
		switch c := v[i]; c {
		case '\'':
			b.WriteString("\\'")
		case '\\':
			b.WriteString("\\\\")
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('\'')
	return b.String()
}
// 格式: file:path/to/database.db?key=value&...
func buildSQLiteDSN(database string, opts map[string]string) string {
	dsn := fmt.Sprintf("file:%s", database)

	query := buildOptionsQuery(opts)
	if query != "" {
		dsn += "?" + query
	}
	return dsn
}

// buildAccessDSN 构建 MS Access DSN (ADODB/OLE DB 格式)
// 格式: Provider=Microsoft.ACE.OLEDB.12.0;Data Source=C:\path\to\database.accdb
func buildAccessDSN(database string, opts map[string]string) string {
	// 根据文件扩展名选择 Provider
	provider := "Microsoft.ACE.OLEDB.12.0" // 默认用 ACE（支持 .accdb）
	if strings.HasSuffix(strings.ToLower(database), ".mdb") {
		provider = "Microsoft.Jet.OLEDB.4.0" // .mdb 用 Jet
	}

	dsn := fmt.Sprintf("Provider=%s;Data Source=%s;", provider, database)

	// 追加额外选项
	if len(opts) > 0 {
		keys := make([]string, 0, len(opts))
		for k := range opts {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			dsn += fmt.Sprintf("%s=%s;", k, opts[k])
		}
	}

	return dsn
}
