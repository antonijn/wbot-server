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

## Example nginx config
```nginx
# /etc/nginx/sites-available/wbot
server {
	listen 80;

	server_name WBot;
	root /var/www/static;

	location / {
		# First attempt to serve request as file, then
		# as directory, then fall back to displaying a 404.
		try_files $uri $uri/ =404;
	}

	location /api/ {
		# Pass along to wbot-server on port 8080
		proxy_set_header Host $host;
		proxy_set_header X-Real-IP $remote_addr;
		proxy_pass http://127.0.0.1:8080/;
	}
}
```
