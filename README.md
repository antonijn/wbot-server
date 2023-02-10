## Example server config

```toml
# /etc/wbot/server.conf

[engine]
exec_path = "/usr/local/bin/wordsmith"
index_path = "/etc/wbot/index.txt"
max_concurrent_users = 2
solve_timeout = 5000
coach_timeout = 4000
```
