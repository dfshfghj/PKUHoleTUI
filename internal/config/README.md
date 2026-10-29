# 配置

首次启动自动生成 `data/config.json`，编辑后重启生效：

```json
{
  "username": "学号（可选）",
  "password": "密码（可选）",
  "secret_key": "TOTP 密钥（可选）",
  "database": {
    "type": "sqlite3",
    "db_file": "./data/treehole.db",
    "max_conns": 8
  }
}
```

- 优先复用 `data/cookies.json`；失效时用 `username` + `password` 自动登录。
- SQLite：`type=sqlite3` + `db_file`。
- PostgreSQL：`type=postgres` + `host` / `port` / `user` / `password` / `name`。

认证细节见 [internal/client](../client/README.md)。
