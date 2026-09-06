---
name: sqlo
description: sqlo 多数据库 SQL 命令行工具（Agent 使用指南）。当 Agent 需要查询、探查或操作数据库时使用：例如用户说"查一下订单表""这个库有哪些表""users 表结构是什么""统计一下今天的注册量""连上测试库跑这条 SQL"。即使用户没提 sqlo，只要意图是"用命令行工具连数据库查数据 / 看表结构 / 跑 SQL"，也应触发本 Skill。支持的数据库包括 MySQL / PostgreSQL / Oracle / SQL Server / SQLite / MS Access / ClickHouse / TDengine / 达梦 DM / 人大金仓 Kingbase，以及对 TiDB / OceanBase / openGauss / GaussDB / TDSQL / PolarDB 的兼容协议连接。本 Skill 封装了 connect / tables / describe / databases / exec 的正确用法与机器可读的 `-o json` 输出约定，避免 Agent 自行拼装各数据库的系统表方言。
---

# sqlo — 多数据库 SQL 命令行工具（Agent 使用指南）

sqlo 是一个单二进制、纯 Go、零 CGO 的多数据库 SQL CLI。本 Skill 告诉 Agent 如何用它连接数据库、探查库表结构、并以机器可读格式执行查询。**所有驱动的内省 SQL 已内置于驱动层，Agent 不需要记忆或拼装任何数据库的系统表方言。**

## 何时使用

- 用户想"查一下 / 统计 / 看看某张表 / 这个库有哪些表 / 表结构是什么"
- 用户给出一条 SQL，要你"连上 X 库跑一下"
- 需要把查询结果交给程序 / 脚本进一步处理（用 `-o json`）
- 需要跨多种数据库用同一套命令探查与查询

## 前置：连接从哪来

`exec` / `tables` / `describe` / `databases` **都要求一个已保存的连接**（存于 `~/.sqlo.json`），通过 `-S <name>` 指定，或用默认连接（`sqlo connect use <name>` 设置）。

> ⚠️ 这三条命令**不支持**行内 `-t -H -P -u -p` 直连。只有 `tui` 支持行内直连。若用户没给连接名，先 `sqlo connect list` 看有哪些可用；若没有，请用户先 `sqlo connect add`（或用 `tui -t ...` 直连做一次性探查）。

```bash
sqlo connect list          # 列出已保存连接（标注默认）
sqlo tables -S prod-pg     # 用指定连接
sqlo tables                # 用默认连接
```

## 探查（优先于盲写 SQL）

```bash
sqlo drivers                          # 离线：sqlo 支持哪些驱动类型
sqlo databases -S <name>              # 这台服务器上有哪些库
sqlo tables -S <name> -o json         # 当前库有哪些表（→ JSON 数组）
sqlo describe users -o json           # users 表结构（列名/类型/可空/默认）
sqlo describe public.users            # 需 schema 的库（PG/MySQL/Oracle/MSSQL/金仓/CH）显式带 schema
sqlo describe                         # 不带表名 → 退化为列出表
```

要点：
- `tables` / `describe` 的元数据 SQL 由对应驱动内置，`-o json` 输出稳定结构，跨库写法一致。
- `describe` 支持 `schema.table` 限定；达梦显式 `owner.table` 会自动注入 owner 过滤，避免多 owner 下字段重复。
- 探查命令 `tables` / `describe` **不受行数限制**，结果集一般很小。

## 执行查询

```bash
sqlo exec -S <name> -e "SELECT * FROM users LIMIT 10" -o json
sqlo exec -S <name> -e "SELECT count(*) FROM orders WHERE created_at > '2026-01-01'" -o json
sqlo exec -S <name> -f query.sql -o json
sqlo exec -S <name> -e "..." -d other_db          # 临时切库（不改配置）
sqlo exec -S <name> -e "..." --row-limit 0        # 关闭行数上限
```

输出约定：
- **查询类（SELECT / SHOW / DESCRIBE / EXPLAIN / WITH）** 走结果集格式化。
- **非查询类（INSERT / UPDATE / DELETE / CREATE …）** 只回显 `✓ 执行成功,影响 N 行`，不返回结构化数据。
- `-o json` 输出到 stdout；超过 `--row-limit`（仅 `exec` 支持，默认 500）的**截断提示写 stderr**，stdout 的 JSON 仍可解析。`--row-limit 0` 表示无限制。

## 只读纪律（约定，非强制）

> sqlo **没有任何读写护栏**——Agent 直接执行任何 SQL，含 `DROP TABLE` / `DELETE` 都不会被拦截。

因此约定：
1. **默认只读**：只跑 `SELECT` / `SHOW` / `DESCRIBE` / `EXPLAIN` / `WITH`。
2. **写操作先问人**：遇到 INSERT / UPDATE / DELETE / DROP / CREATE / ALTER / TRUNCATE，先向用户确认意图与范围，不要自行执行。
3. **危险语句二次确认**：`DROP` / `TRUNCATE` / `DELETE` 无 `WHERE` / `UPDATE` 无 `WHERE` 等，必须显式取得用户授权。

## 已知坑 / 方言差异

- **密码明文**：连接配置（含密码）以明文写在 `~/.sqlo.json`，确保该文件权限受控。
- **达梦 / TDengine 的 `tables` 已支持**：这两个驱动的 `ListTables` 模板带 `?` 占位符（分别是 owner / db_name），`BuildListTables` 会自动注入当前连接用户名 / 当前库，所以 `sqlo tables`（及 TUI `\dt`）可直接使用，无需绕过。
- **无会话态**：`exec` 每次一连接，临时表 / 未提交事务 / 会话变量不跨语句保留。需要会话态用 `tui`。
- **方言**：`SHOW` 在 MySQL 系可用、PG 不可用；`\d` 等价 `describe`；分页 `LIMIT` / `TOP` / `ROWNUM` 各有差异。
- **`-S` 与行内直连**：`tui -t ...` 行内直连时 `-S` 被忽略；`exec` / `tables` / `describe` 无行内直连能力。

## 推荐工作流

1. `sqlo connect list` → 确认有可用连接（没有则请用户 `connect add`）。
2. `sqlo tables -S <name> -o json` → 拿到表清单。
3. `sqlo describe <表> -o json` → 拿列结构。
4. `sqlo exec -S <name> -e "..." -o json` → 跑查询，机器解析结果。
5. 写操作一律先问人。
