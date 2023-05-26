// Copyright © Rob Burke inchworks.com, 2020.

// This file is part of PicInch.
//
// PicInch is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// PicInch is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with PicInch.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golangcollege/sessions"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/inchworks/usage"
	"github.com/inchworks/webparts/etx"
	"github.com/inchworks/webparts/limithandler"
	"github.com/inchworks/webparts/multiforms"
	"github.com/inchworks/webparts/server"
	"github.com/inchworks/webparts/stack"
	"github.com/inchworks/webparts/uploader"
	"github.com/inchworks/webparts/users"
	"github.com/jmoiron/sqlx"
	"github.com/justinas/nosurf"
	"github.com/microcosm-cc/bluemonday"

	"inchworks.com/picinch/internal/emailer"
	"inchworks.com/picinch/internal/models"
	"inchworks.com/picinch/internal/models/mysql"

	"inchworks.com/picinch/internal/tags"
	"inchworks.com/picinch/web"
)

// version and copyright
const (
	version = "1.0.14"
	notice  = `
	Copyright (C) Rob Burke inchworks.com, 2020.
	This website software comes with ABSOLUTELY NO WARRANTY.
	This is free software, and you are welcome to redistribute it under certain conditions.
	For details see the license on https://github.com/inchworks/picinch.
`
)

// file locations on server
var (
	CertPath  = "../certs"  // cached certificates
	GeoDBPath = "../geodb"  // geo database
	ImagePath = "../photos" // pictures
	SitePath  = "../site"   // site-specific resources
	MiscPath  = "../misc"   // misc
)

// database operational parameters
const (
	connMaxLifetime = 200 // (sec) lifetime of idle connections (MySQL wait_timeout is 600)
)

// put context key in its own type,
// to avoid collision with any 3rd-party packages using request context
type contextKey int

const contextKeyUser = contextKey(1)

type AuthenticatedUser struct {
	id   int64
	role int
}

// Site configuration
type Configuration struct {

	// domains served via HTTPS
	Domains   []string `yaml:"domains" env:"domains" env-default:""`
	CertEmail string   `yaml:"certificate-email" env:"certificate-email" env-default:""`

	// from command line only
	AddrHTTP  string `yaml:"http-addr" env:"http" env-default:":8000" env-description:"HTTP address"`
	AddrHTTPS string `yaml:"https-addr" env:"https" env-default:":4000" env-description:"HTTPS address"`

	Secret string `yaml:"session-secret" env:"session-secret" env-default:"Hk4TEiDgq8JaCNR?WaPeWBf4QQYNUjMR" env-description:"Secret key for sessions"`

	// new DSN
	DBSource   string `yaml:"db-source" env:"db-source" env-default:"tcp(picinch_db:3306)/picinch"`
	DBUser     string `yaml:"db-user" env:"db-user" env-default:"server"`
	DBPassword string `yaml:"db-password" env:"db-password" env-default:"<server-password>"`

	// administrator
	AdminName     string `yaml:"admin-name" env:"admin-name" env-default:""`
	AdminPassword string `yaml:"admin-password" env:"admin-password" env-default:"<your-password>"`

	// image sizes
	MaxW      int `yaml:"image-width" env-default:"1600"` // maximum stored image dimensions
	MaxH      int `yaml:"image-height" env-default:"1200"`
	ThumbW    int `yaml:"thumbnail-width" env-default:"278"` // thumbnail size
	ThumbH    int `yaml:"thumbnail-height" env-default:"208"`
	MaxUpload int `yaml:"max-upload" env-default:"64"` // maximum file upload (megabytes)

	// total limits
	MaxHighlightsParent int `yaml:"parent-highlights"  env-default:"16"` // highlights for parent website
	MaxHighlightsTotal  int `yaml:"highlights-page" env-default:"12"`    // highlights for home page, and user's page
	MaxHighlightsTopic  int `yaml:"highlights-topic" env-default:"32"`   // slides in highights slideshow
	MaxSlideshowsTotal  int `yaml:"slideshows-page" env-default:"16"`    // total slideshows on home page

	// per user limits
	MaxHighlights       int `yaml:"highlights-user"  env-default:"2"`  // highlights on home page
	MaxSlides           int `yaml:"slides-show" env-default:"50"`      // slides in a slideshow
	MaxSlideshowsClub   int `yaml:"slideshows-club"  env-default:"2"`  // club slideshows on home page, per user
	MaxSlideshowsPublic int `yaml:"slideshows-public" env-default:"1"` // public slideshows on home page, per user

	// operational settings
	AllowedQueries    []string        `yaml:"allowed-queries" env-default:"fbclid"`                            // URL query names allowed
	BanBadFiles       bool            `yaml:"limit-bad-files" env-default:"false"`                             // apply ban to requests for missing files
	GeoBlock          []string        `yaml:"geo-block" env:"geo-block" env-default:""`                        // blocked countries (ISO 3166-1 alpha-2 codes)
	MaxUploadAge      time.Duration   `yaml:"max-upload-age" env:"max-upload-age" env-default:"8h"`            // maximum time for a slideshow update. Units m or h.
	MaxUnvalidatedAge time.Duration   `yaml:"max-unvalidated-age" env:"max-unvalidated-age" env-default:"48h"` // maximum time for a competition entry to be validated. Units h.
	SiteRefresh       time.Duration   `yaml:"thumbnail-refresh"  env-default:"1h"`                             // refresh interval for topic thumbnails. Units m or h.
	UsageAnonymised   usage.Anonymise `yaml:"usage-anon" env-default:"1"`

	// variants
	HomeSwitch    string        `yaml:"home-switch" env:"home-switch" env-default:""`           // switch home page to specified template, e.g when site disabled
	MiscName      string        `yaml:"misc-name" env:"misc-name" env-default:"misc"`           // path in URL for miscellaneous files, as in "example.com/misc/file"
	Options       string        `yaml:"options" env:"options" env-default:""`                   // site features: main-comp, with-comp
	VideoSnapshot time.Duration `yaml:"video-snapshot"  env-default:"3s"`                       // snapshot time within video. -ve for no snapshots.
	VideoPackage  string        `yaml:"video-package" env:"video-package" env-default:"ffmpeg"` // video processing package
	VideoTypes    []string      `yaml:"video-types" env:"video-types" env-default:""`           // video types (.mp4, .mov, etc.)

	// email
	EmailHost     string `yaml:"email-host" env:"email-host" env-default:""`
	EmailPort     int    `yaml:"email-port" env:"email-port" env-default:"587"`
	EmailUser     string `yaml:"email-user" env:"email-user" env-default:""`
	EmailPassword string `yaml:"email-password" env:"email-password" env-default:""`
	Sender        string `yaml:"sender" env:"sender" env-default:""`
	ReplyTo       string `yaml:"reply-to" env:"reply-to" env-default:""`
}

// Operation to update slideshow images.
type OpUpdateShow struct {
	ShowId  int64
	TopicId int64
	Revised bool
	tx      etx.TxId
}

// Operation to update topic images.
type OpUpdateTopic struct {
	TopicId int64
	Revised bool
	tx      etx.TxId
}

// Application struct supplies application-wide dependencies.
type Application struct {
	cfg *Configuration

	errorLog      *log.Logger
	infoLog       *log.Logger
	threatLog     *log.Logger
	session       *sessions.Session
	templateCache map[string]*template.Template

	// database
	db      *sqlx.DB
	tx      *sqlx.Tx
	statsTx *sqlx.Tx

	SlideStore     *mysql.SlideStore
	GalleryStore   *mysql.GalleryStore
	redoStore      *mysql.RedoStore
	SlideshowStore *mysql.SlideshowStore
	statisticStore *mysql.StatisticStore
	userStore      *mysql.UserStore

	// common components
	geoblocker *server.GeoBlocker
	lhs        *limithandler.Handlers
	tm         *etx.TM
	uploader   *uploader.Uploader
	usage      *usage.Recorder
	users      users.Users

	// HTML sanitizer for titles and captions
	sanitizer *bluemonday.Policy

	// private components
	emailer  emailer.Emailer
	tagger   tags.Tagger
	staticFS fs.FS

	// HTML handlers for threats detected by application logic
	wrongCode http.Handler

	// Channels to background worker
	chComp  chan OpUpdateShow
	chShow  chan OpUpdateShow
	chShows chan []OpUpdateShow
	chTopic chan OpUpdateTopic

	// Since we support just one gallery at a time, we can cache state here.
	// With multiple galleries, we'd need a per-gallery cache.
	galleryState GalleryState

	// slow to evaluate, so worth caching
	ServerAddr string
}

func main() {

	// logging
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	threatLog := log.New(os.Stdout, "THREAT\t", log.Ldate|log.Ltime)
	infoLog.Printf("PicInch Gallery %s", version)
	infoLog.Print(notice)

	// redirect to test folders
	test := os.Getenv("test")
	if test != "" {
		CertPath = filepath.Join(test, filepath.Base(CertPath))
		GeoDBPath = filepath.Join(test, filepath.Base(GeoDBPath))
		ImagePath = filepath.Join(test, filepath.Base(ImagePath))
		MiscPath = filepath.Join(test, filepath.Base(MiscPath))
		SitePath = filepath.Join(test, filepath.Base(SitePath))
	}

	// site configuration
	cfg := &Configuration{}
	if err := cleanenv.ReadConfig(filepath.Join(SitePath, "configuration.yml"), cfg); err != nil {

		// no file - go with just environment variables
		infoLog.Print(err.Error())
		if err := cleanenv.ReadEnv(cfg); err != nil {
			errorLog.Fatal(err)
		}
	}

	// database
	dsn := fmt.Sprintf("%s:%s@%s?parseTime=true", cfg.DBUser, cfg.DBPassword, cfg.DBSource)
	db, err := openDB(dsn)
	if err != nil {
		errorLog.Fatal(err)
	} else {
		infoLog.Print("Connected to database")
	}

	// close DB on exit
	defer db.Close()

	// initialise application
	app := initialise(cfg, errorLog, infoLog, threatLog, db)

	defer app.geoblocker.Stop()
	defer app.lhs.Stop()
	defer app.uploader.Stop()
	defer app.usage.Stop()

	// tickers for refresh and purge
	tr := time.NewTicker(app.cfg.SiteRefresh)
	defer tr.Stop()
	tp := time.NewTicker(app.cfg.MaxUnvalidatedAge / 8)
	defer tp.Stop()

	// closing this channel signals worker goroutines to return
	chDone := make(chan bool, 1)
	defer close(chDone)

	// start background worker
	go app.galleryState.worker(app.chComp, app.chShow, app.chShows, app.chTopic, tr.C, tp.C, chDone)

	// redo any pending operations
	infoLog.Print("Starting operation recovery")
	if err := app.tm.Recover(&app.galleryState, app.uploader); err != nil {
		errorLog.Fatal(err)
	}

	// preconfigured HTTP/HTTPS server
	srv := &server.Server{
		ErrorLog: app.newServerLog(os.Stdout, "SERVER\t", log.Ldate|log.Ltime),
		InfoLog:  infoLog,

		CertEmail: cfg.CertEmail,
		CertPath:  CertPath,
		Domains:   cfg.Domains,

		// port addresses
		AddrHTTP:  cfg.AddrHTTP,
		AddrHTTPS: cfg.AddrHTTPS,
	}

	srv.Serve(app)
}

// ** INTERFACE FUNCTIONS FOR WEBPARTS/USERS **

// Authenticated adds a logged-in user's ID to the session.
func (app *Application) Authenticated(r *http.Request, id int64) {
	app.session.Put(r, "authenticatedUserID", id)
}

// Flash adds a confirmation message to the next page, via the session.
func (app *Application) Flash(r *http.Request, msg string) {
	app.session.Put(r, "flash", msg)
}

// GetRedirect returns the next page after log-in, probably from a session key.
func (app *Application) GetRedirect(r *http.Request) string { return "/" }

// Log optionally records an error.
func (app *Application) Log(err error) {
	app.errorLog.Print(err)
}

// LogThreat optionally records a rejected request to sign-up or log-in.
func (app *Application) LogThreat(msg string, r *http.Request) {
	app.threat(msg, r)
}

// OnAddUser is called to add any additional application data for a user.
func (app *Application) OnAddUser(user *users.User) {
	// not needed for this application
}

// OnRemoveUser is called to delete any application data for a user.
func (app *Application) OnRemoveUser(tx etx.TxId, user *users.User) {

	app.galleryState.OnRemoveUser(tx, user)
}

// Render writes an HTTP response using the specified template and field (embedded as Users).
func (app *Application) Render(w http.ResponseWriter, r *http.Request, template string, usersField interface{}) {
	app.render(w, r, template, &usersFormData{Users: usersField})
}

// Rollback specifies that the transaction started by Serialise be cancelled.
func (app *Application) Rollback() {
	app.galleryState.rollbackTx = true
}

// Serialise optionally requests application-level serialisation.
// If updates=true, the store is to be updated and a transaction might be started (especially if a user is to be added or deleted).
// The returned function will be called at the end of the operation.
func (app *Application) Serialise(updates bool) func() {
	return app.galleryState.updatesGallery()
}

// Token returns a token to be added to the form as the hidden field csrf_token.
func (app *Application) Token(r *http.Request) string {
	return nosurf.Token(r)
}

// Initialisation, common to live and test

func initialise(cfg *Configuration, errorLog *log.Logger, infoLog *log.Logger, threatLog *log.Logger, db *sqlx.DB) *Application {

	// package templates
	var pts []fs.FS

	// templates for user management
	pt, err := fs.Sub(users.WebFiles, "web/template")
	pts = append(pts, pt)

	// application templates
	forApp, err := fs.Sub(web.Files, "template")
	if err != nil {
		errorLog.Fatal(err)
	}

	// initialise template cache
	templateCache, err := stack.NewTemplates(pts, forApp, os.DirFS(filepath.Join(SitePath, "templates")), templateFuncs)
	if err != nil {
		errorLog.Fatal(err)
	}

	// session manager
	session := sessions.New([]byte(cfg.Secret))
	session.Lifetime = 12 * time.Hour

	// dependency injection
	app := &Application{
		cfg:           cfg,
		errorLog:      errorLog,
		infoLog:       infoLog,
		threatLog:     threatLog,
		session:       session,
		templateCache: templateCache,
		db:            db,
		sanitizer:     bluemonday.UGCPolicy(),
	}

	// embedded static files from packages
	staticForms, err := fs.Sub(multiforms.WebFiles, "web/static")
	if err != nil {
		errorLog.Fatal(err)
	}
	staticUploader, err := fs.Sub(uploader.WebFiles, "web/static")
	if err != nil {
		errorLog.Fatal(err)
	}

	// embedded static files from app
	staticApp, err := fs.Sub(web.Files, "static")
	if err != nil {
		errorLog.Fatal(err)
	}

	// combine embedded static files with site customisation
	// ## perhaps site resources should be under "static"?
	app.staticFS, err = stack.NewFS(staticForms, staticUploader, staticApp, os.DirFS(SitePath))
	if err != nil {
		errorLog.Fatal(err)
	}

	// initialise gallery state
	app.galleryState.Init(app)

	// initialise data stores
	gallery := app.initStores(cfg)

	// cached state
	if err := app.galleryState.setupCache(gallery); err != nil {
		errorLog.Fatal(err)
	}

	// setup emailing
	var localHost string
	// ## If not set by the SMTP relay service.
	// if len(cfg.Domains) > 0 {
	// 	localHost = cfg.Domains[0]
	// }

	if app.cfg.EmailHost == "mailgun" {
		app.emailer = emailer.NewGunner(app.cfg.EmailUser, app.cfg.EmailPassword, app.cfg.Sender, app.cfg.ReplyTo, app.templateCache)

	} else if app.cfg.EmailHost != "" {
		app.emailer = emailer.NewDialer(app.cfg.EmailHost, app.cfg.EmailPort, app.cfg.EmailUser, app.cfg.EmailPassword, app.cfg.Sender, app.cfg.ReplyTo, localHost, app.templateCache)
	}

	// set up extended transaction manager, and recover
	app.tm = etx.New(app, app.redoStore)

	// setup media upload processing
	app.uploader = &uploader.Uploader{
		FilePath:     ImagePath,
		MaxW:         app.cfg.MaxW,
		MaxH:         app.cfg.MaxH,
		ThumbW:       app.cfg.ThumbW,
		ThumbH:       app.cfg.ThumbH,
		MaxAge:       app.cfg.MaxUploadAge,
		SnapshotAt:   app.cfg.VideoSnapshot,
		VideoPackage: app.cfg.VideoPackage,
		VideoTypes:   app.cfg.VideoTypes,
	}
	app.uploader.Initialise(app.errorLog, &app.galleryState, app.tm)

	// setup tagging
	app.tagger.ErrorLog = app.errorLog
	app.tagger.UserStore = app.userStore

	// initialise rate limiters
	app.lhs = limithandler.Start(6*time.Hour, 24*time.Hour)

	// handlers for HTTP threats detected by application logic
	app.wrongCode = app.codeNotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Access code not found", http.StatusNotFound)
	}))

	// setup usage, with defaults
	if app.usage, err = usage.New(app.statisticStore, cfg.UsageAnonymised, 0, 0, 0, 0, 0); err != nil {
		errorLog.Fatal(err)
	}
	app.usage.SetSaverCallback(func(u *usage.Recorder) {
		u.Add("blocked", "bad-req", app.lhs.RejectsCounted()+app.geoblocker.RejectsCounted())
	})

	// user management
	app.users = users.Users{
		App:   app,
		Roles: []string{"unknown", "friend", "member", "curator", "admin"},
		Store: app.userStore,
		TM:    app.tm,
	}

	// geo-blocking
	app.geoblocker = &server.GeoBlocker{
		ErrorLog: errorLog,
		ReportSingle: true,
		Store: GeoDBPath,
	}
	app.geoblocker.Start(cfg.GeoBlock)

	// create worker channels
	app.chComp = make(chan OpUpdateShow, 10)
	app.chShow = make(chan OpUpdateShow, 10)
	app.chShows = make(chan []OpUpdateShow, 1)
	app.chTopic = make(chan OpUpdateTopic, 10)

	return app
}

// Initialise data stores

func (app *Application) initStores(cfg *Configuration) *models.Gallery {

	defer app.galleryState.updatesGallery()()

	// setup stores, with reference to a common transaction
	// ## transaction should be per-gallery if we support multiple galleries
	app.SlideStore = mysql.NewSlideStore(app.db, &app.tx, app.errorLog)
	app.GalleryStore = mysql.NewGalleryStore(app.db, &app.tx, app.errorLog)
	app.redoStore = mysql.NewRedoStore(app.db, &app.tx, app.errorLog)
	app.SlideshowStore = mysql.NewSlideshowStore(app.db, &app.tx, app.errorLog)
	app.statisticStore = mysql.NewStatisticStore(app.db, &app.statsTx, app.errorLog)
	app.tagger.TagStore = mysql.NewTagStore(app.db, &app.tx, app.errorLog)
	app.tagger.TagRefStore = mysql.NewTagRefStore(app.db, &app.tx, app.errorLog)
	app.userStore = mysql.NewUserStore(app.db, &app.tx, app.errorLog)

	// database change to users table, to use webparts
	// The first part must be done before we add a missing admin,
	var err error
	if err = mysql.MigrateWebparts1(app.tx); err != nil {
		app.errorLog.Fatal(err)
	}

	// setup new database and administrator, if needed, and get gallery record
	g, err := mysql.Setup(app.GalleryStore, app.userStore, 1, cfg.AdminName, cfg.AdminPassword)
	if err != nil {
		app.errorLog.Fatal(err)
	}

	// save gallery ID for stores that need it
	app.SlideshowStore.GalleryId = g.Id
	app.tagger.TagStore.GalleryId = g.Id
	app.userStore.GalleryId = g.Id

	// highlights topic ID
	app.SlideshowStore.HighlightsId = 1

	// database changes from previous version(s)
	topicStore := mysql.NewTopicStore(app.db, &app.tx, app.errorLog)
	topicStore.GalleryId = g.Id
	if err = mysql.MigrateTopics(topicStore, app.SlideshowStore, app.SlideStore); err != nil {
		app.errorLog.Fatal(err)
	}
	if err = mysql.MigrateWebparts2(app.userStore, app.tx); err != nil {
		app.errorLog.Fatal(err)
	}
	if err = mysql.MigrateTags(app.tagger.TagStore); err != nil {
		app.errorLog.Fatal(err)
	}
	if err = mysql.MigrateRedo(app.redoStore); err != nil {
		app.errorLog.Fatal(err)
	}

	return g
}

// newServerLog returns a logger that filters common events cause by background noise from internet idiots.
// (Typically probes using unsupported TLS versions or attempting HTTPS connection without a domain name.
// Also continuing access attempts with the domain of a previous holder of the server's IP address.)
func (app *Application) newServerLog(out io.Writer, prefix string, flag int) *log.Logger {

	filter := []string{"TLS handshake error"}

	return app.usage.NewLogger(out, prefix, flag, filter, "bad-req")
}

// Open database

// ## jmoiron/sqlx recommends github.com/mattn/go-sqlite3

func openDB(dsn string) (db *sqlx.DB, err error) {

	// Running under Docker, the DB container may not be ready yet - retry for 30s
	nRetries := 30

	for ; nRetries > 0; nRetries-- {
		db, err = sqlx.Open("mysql", dsn)
		if err == nil {
			break
		}
		time.Sleep(1000 * time.Millisecond)
	}

	// test a connection to DB
	for ; nRetries > 0; nRetries-- {
		err = db.Ping()
		if err == nil {
			break
		}
		time.Sleep(1000 * time.Millisecond)
	}

	if nRetries == 0 {
		return nil, err
	}

	// Close idle connections before MySQL drops them. Otherwise we get an error after idling.
	db.SetConnMaxLifetime(connMaxLifetime * time.Second)

	return db, nil
}
