module inchworks.com/picinch

go 1.16

require (
	github.com/go-mail/mail/v2 v2.3.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golangcollege/sessions v1.2.0
	github.com/ilyakaznacheev/cleanenv v1.2.6
	github.com/inchworks/usage v0.2.1
	github.com/inchworks/webparts v0.13.1
	github.com/jmoiron/sqlx v1.3.4
	github.com/julienschmidt/httprouter v1.3.0
	github.com/justinas/alice v1.2.0
	github.com/justinas/nosurf v1.1.1
	github.com/mailgun/mailgun-go/v4 v4.6.0
	github.com/microcosm-cc/bluemonday v1.0.17
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/mail.v2 v2.3.1 // indirect
)

// replace github.com/inchworks/usage v0.2.1 => ../../usage
// replace github.com/inchworks/webparts v0.13.1 => ../../webparts
