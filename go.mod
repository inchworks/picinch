module inchworks.com/picinch

go 1.22

toolchain go1.23.1

require (
	github.com/alexedwards/scs/v2 v2.8.0
	github.com/go-mail/mail/v2 v2.3.0
	github.com/go-sql-driver/mysql v1.8.1
	github.com/ilyakaznacheev/cleanenv v1.5.0
	github.com/inchworks/usage v1.3.1
	github.com/inchworks/webparts/v2 v2.1.1
	github.com/jmoiron/sqlx v1.4.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/justinas/alice v1.2.0
	github.com/justinas/nosurf v1.1.1
	github.com/mailgun/mailgun-go/v4 v4.20.6
	github.com/microcosm-cc/bluemonday v1.0.27
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/alexedwards/scs/mysqlstore v0.0.0-20240316134038-7e11d57e8885
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/go-chi/chi/v5 v5.1.0 // indirect
	github.com/gomarkdown/markdown v0.0.0-20241205020045-f7e15b2f3e62
	github.com/gorilla/css v1.0.1 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailgun/errors v0.4.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/oschwald/maxminddb-golang v1.13.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/crypto v0.30.0 // indirect
	golang.org/x/image v0.23.0 // indirect
	golang.org/x/net v0.32.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/mail.v2 v2.3.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	olympos.io/encoding/edn v0.0.0-20201019073823-d3554ca0b0a3 // indirect
)

// replace github.com/inchworks/usage v1.3.1 => ../usage
replace github.com/inchworks/webparts/v2 v2.1.1 => ../webparts
