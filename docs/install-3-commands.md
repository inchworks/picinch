
# Commands

## Docker
`docker compose up -d` When issued the first time, sets up the database, creates the directories to hold content and certificates (in`/srv/picinch/`), and starts PicInch. On later invocations, checks for updates to PicInch, and restarts it if needed.

`docker compose restart` Restarts PicInch, reading any changes to `configuration.yml` and site-specific templates.

`docker compose down` Stops PicInch.

`docker compose logs --tail=100` View the last e.g. 100 entries in application logs.
Look here for any startup errors, as well as details of security threats.

## Database
If you wish to delete all site content and start again, stop the server, delete `/srv/picinch/mysql` and `/srv/picinch/photos`, and restart the server.