## Step 2: System service
1. `/etc/systemd/system/picinch.service` defines PicInch as a service. There is nothing special about PicInch requirements and this is provided as a convenience for those with little experience of Linux system operation.

```
# /etc/systemd/system/picinch.service

[Unit]
Description=PicInch Gallery
Requires=docker.service
After=network-online.target docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/srv/picinch
ExecStart=/usr/local/bin/docker-compose up -d
ExecStop=/usr/local/bin/docker-compose down
ExecReload=/usr/local/bin/docker-compose up -d
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
```

1. Set PicInch to start when the system is restarted: `systemctl enable picinch`