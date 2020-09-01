
# Step 3: Commands
## Service
`systemctl start picinch.service` When issued the first time, sets up the database, creates the folders to hold images and certificates (in`/srv/picinch/`), and starts the server. On later invocations, starts the server.
`systemctl stop picinch.service` Stops the server.
`systemctl reload picinch.service` Restarts the server. **What about version updates?**
## Docker
If for any reason you have not defines PicInch as a service, you can also start it from `/srv/picinch` with the command `docker-compose up`, and stop it with `docker-compose down`.
`docker-compose -f dc-live.yml logs --tail=100` View the last 100 entries in application logs. Note that the logs are cleared when the service is restarted. **can this be changed?**
## Database
If you wish to delete all site content and start again, stop the server, delete `/srv/picinch/mysql` and `/srv/picinch/photos` and restart the server,