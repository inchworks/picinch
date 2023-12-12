module inchworks.com/picinch

go 1.16

require (
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/go-chi/chi/v5 v5.0.10 // indirect
	github.com/go-mail/mail/v2 v2.3.0
	github.com/go-sql-driver/mysql v1.7.1
	github.com/golangcollege/sessions v1.2.0
	github.com/ilyakaznacheev/cleanenv v1.5.0
	github.com/inchworks/usage v1.3.0
	github.com/inchworks/webparts v1.4.1
	github.com/jmoiron/sqlx v1.3.5
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/julienschmidt/httprouter v1.3.0
	github.com/justinas/alice v1.2.0
	github.com/justinas/nosurf v1.1.1
	github.com/mailgun/mailgun-go/v4 v4.11.0
	github.com/microcosm-cc/bluemonday v1.0.25
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/pkg/errors v0.9.1 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/mail.v2 v2.3.1 // indirect
)

// replace github.com/inchworks/usage v1.3.0 => ../usage
replace github.com/inchworks/webparts v1.4.1 => ../webparts
