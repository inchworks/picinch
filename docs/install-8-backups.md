## Step 8: Backups
PicInch uses the following files and directories, under `/srv/picinch`:
- `certs` Let’s Encrypt certificates. Backup not required.
- `misc` Serves files placed there. Backup the originals.
- `mysql` The live database. Not suitable for backup.
- `photos` Users’s photos. Backup required.
- `site` Site configuration. Backup whenever changes are made.
- `docker-compose.yml` Backup whenever changes are made (or keep parameters in `site/configuration.yml` if preferred).

There is no inbuilt support for backups. Some VM hosts offer a periodic backup of the whole server.

For a manual backup:
1. Stop the service.
2. Export the database: `docker exec picinch_db /usr/bin/mysqldump -uroot -<root-password> picinch > backup.sql` 
3. Copy the resulting backup.sql file to a backup system.
4. Copy `/srv/picinch/photos` to the backup system.
5. Restart the service.

To restore from backup:
1. Install PicInch if necessary, or stop the service.
2. Import a database backup: `docker exec -i picinch_db /usr/bin/mysql -u root -<root-password> picinch < backup.sql`
3. Restore `/srv/picinch/photos` from backup.
4. Restart the service.
