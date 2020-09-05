
# Step 3: Commands
## Service
`systemctl start picinch` When issued the first time, sets up the database, creates the directories to hold content and certificates (in`/srv/picinch/`), and starts PicInch. On later invocations, just starts PicInch as a service.

`systemctl stop picinch` Stops the PicInch service.

## Docker
`docker-compose logs --tail=100` View the last e.g. 100 entries in application logs.
Look here for any startup errors, as well as details of security threats.

To pull updated images from Docker Hub:
1. Stop the service.
1. `docker-compose pull` to fetch updated images from Docker Hub.
1. Restart the service.

For new features, check Docker Hub for an `inchworks/picinch` image tagged `1.0`, `1.1`, `2.0` etc, and edit `docker-compose.yml` to match. A different major version number for PicInch indicates that configuration changes will be needed.

If for any reason you do not want to run PicInch as a service, you can also start it from `/srv/picinch` with the command `docker-compose up`, and stop it with `docker-compose down`.

## Database
If you wish to delete all site content and start again, stop the server, delete `/srv/picinch/mysql` and `/srv/picinch/photos`, and restart the server.