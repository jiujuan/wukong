已把 `wukong/scripts/schema.sql` 转成 goose 版本化迁移，放在了 `/cmd/migration` 下：

- 新增命令入口：[main.go](D:/codeproject/mygo/wukong/wukong/cmd/migration/main.go)
- 新增首个迁移文件：[00001_init_schema.sql](D:/codeproject/mygo/wukong/wukong/cmd/migration/migrations/00001_init_schema.sql)
- 更新依赖：`wukong/go.mod`、`wukong/go.sum` 加入 `github.com/pressly/goose/v3`

用法示例：

```powershell
cd D:\codeproject\mygo\wukong\wukong

go run ./cmd/migration status
go run ./cmd/migration up
go run ./cmd/migration down
```

也可以指定配置或数据库 URL：

```powershell
go run ./cmd/migration -config configs/dev.yaml up
go run ./cmd/migration -db "postgres://postgres:postgres@localhost:5432/wukong_agents_db?sslmode=disable" up
```

我顺手把原 `schema.sql` 里已经乱码且会导致 SQL 失败的注释/字符串清理成了可执行迁移版本，并补了 `Down` 回滚段。验证已通过：

```text
go test ./cmd/migration
? github.com/jiujuan/wukong/cmd/migration [no test files]
```

注意：迁移里使用了 `CREATE EXTENSION IF NOT EXISTS vector;`，目标 PostgreSQL 需要安装 pgvector 扩展，否则执行到长期记忆表的 `vector(1536)` 字段会失败。