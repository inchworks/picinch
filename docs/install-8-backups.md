## Step 8: Backups
PicInch uses the following files and directories, under `/srv/picinch`:
- `certs` Let’s Encrypt certificates. Don’t require backup.
- `mysql` The live database. Not suitable for backup.
- `photos` Users’s photos. Backup required.
- `site` Site configuration. Backup whenever changes are made.
- `videos` Unsupported, except to serve files placed there. Backup originals.
- `docker-compose.yml` Backup whenever changes are made (or keep parameters in `site/configuration.yml` if preferred).

There is no inbuilt support for backups. Some VM hosts offer a periodic backup of the whole server.

For a manual backup:
1. Stop the service.
2. Export the database: `docker exec gallery_db /usr/bin/mysqldump -uroot -ptGERUtJvbYRjuF 50p-gallery > backup.sql` 
3. Copy the resulting backup.sql file to a backup system.
4. Copy `/srv/picinch/photos` to the backup system.
5. Restart the service.

To restore from backup:
1. Install PicInch if necessary, or stop the service.
2. Import a database backup: `cat backup.sql | docker exec -i gallery_db /usr/bin/mysql -u root -ptGERUtJvbYRjuF 50p-gallery`
3. Restore `/srv/picinch/photos` from backup.
4. Restart the service.
