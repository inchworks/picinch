
# Step 3: Commands
## Service
`systemctl start picinch` When issued the first time, sets up the database, creates the folders to hold images and certificates (in`/srv/picinch/`), and starts PicInch. On later invocations, just starts PicInch as a service.

`systemctl stop picinch` Stops the PicInch service.

`systemctl restart picinch` Restarts the service.
## Docker
If for any reason you have not defines PicInch as a service, you can also start it from `/srv/picinch` with the command `docker-compose up`, and stop it with `docker-compose down`.

`docker-compose logs --tail=100` View the last 100 entries in application logs.

## Database
If you wish to delete all site content and start again, stop the server, delete `/srv/picinch/mysql` and `/srv/picinch/photos` and restart the server,