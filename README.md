## Example server config

```toml
# /etc/wbot/server.conf
[server]
port = 8080

[engine]
exec_path = "/usr/local/bin/wordsmith"
index_path = "/etc/wbot/index.txt"
max_concurrent_users = 2
solve_timeout = 5000
coach_timeout = 4000
```

## Example systemd service file
```ini
# /etc/systemd/system/wbot-server.service
[Unit]
Description=Wordsmith web server
After=network.target nss-lookup.target

[Service]
Type=simple
User=wordsmith
Group=wordsmith
ExecStart=/usr/local/bin/wbot-server
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```
