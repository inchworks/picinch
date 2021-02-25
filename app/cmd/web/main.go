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
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"inchworks.com/picinch/pkg/images"
	"inchworks.com/picinch/pkg/models"
	"inchworks.com/picinch/pkg/models/mysql"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golangcollege/sessions"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/inchworks/usage"
	"github.com/inchworks/webparts/server"
	"github.com/inchworks/webparts/users"
	"github.com/jmoiron/sqlx"
	"github.com/justinas/nosurf"
	"github.com/microcosm-cc/bluemonday"
)

// version and copyright
const (
	version = "0.9.5"
	notice  = `
	Copyright (C) Rob Burke inchworks.com, 2020.
	This website software comes with ABSOLUTELY NO WARRANTY.
	This is free software, and you are welcome to redistribute it under certain conditions.
	For details see the license on https://github.com/inchworks/picinch.
`
)

// file locations on server
const (
	CertPath   = "../certs"         // cached certificates
	ImagePath  = "../photos"        // pictures
	SitePath   = "../site"          // site-specific resources
	StaticPath = "../app/ui/static" // static resources
	UIPath     = "../app/ui"        // user inteface resources
	MiscPath   = "../misc"          // misc
)

// database operational parameters
const (
	connMaxLifetime = 200 // (sec) lifetime of idle connections (MySQL wait_timeout is 600)
)

// put context key in its own type,
// to avoid collision with any 3rd-party packages using request context
type contextKey string

const contextKeyUser = contextKey("authenticatedUser")

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
	MaxW int `yaml:"image-width" env-default:"1600"` // maximum stored image dimensions
	MaxH int `yaml:"image-height" env-default:"1200"`

	ThumbW int `yaml:"thumbnail-width" env-default:"278"` // thumbnail size
	ThumbH int `yaml:"thumbnail-height" env-default:"208"`

	// total limits
	MaxHighlightsParent int `yaml:"parent-highlights"  env-default:"16"` // highlights for parent website
	MaxHighlightsTotal  int `yaml:"highlights-page" env-default:"12"`    // highlights for home page, and user's page
	MaxHighlightsTopic  int `yaml:"highlights-topic" env-default:"32"`   // total slides in H format topic // ## misleading name?
	MaxSlideshowsTotal  int `yaml:"slideshows-page" env-default:"16"`   // total slideshows on home page

	// per user limits
	MaxHighlights       int `yaml:"highlights-user"  env-default:"2"`  // highlights on home page
	MaxSlides           int `yaml:"slides-show" env-default:"10"`      // ## not implemented
	MaxSlideshowsClub   int `yaml:"slideshows-club"  env-default:"2"`  // club slideshows on home page, per user
	MaxSlideshowsPublic int `yaml:"slideshows-public" env-default:"1"` // public slideshows on home page, per user

	MiscName        string          `yaml:"misc-name" env:"misc-name" env-default:"misc"` // path in URL for miscelleneous files, as in "example.com/misc/file"
	SiteRefresh     time.Duration   `yaml:"thumbnail-refresh"  env-default:"1h"`          // refresh interval for topic thumbnails. Units m or h.
	UsageAnonymised usage.Anonymise `yaml:"usage-anon" env-default:"1"`
}

// Request to update slideshow images.
type reqUpdateShow struct {
	showId  int64
	userId  int64
	revised bool
}

// Request to update topic images.
type reqUpdateTopic struct {
	topicId  int64
	revised bool
}

// Application struct supplies application-wide dependencies.
type Application struct {
	cfg *Configuration

	errorLog      *log.Logger
	infoLog       *log.Logger
	threatLog     *log.Logger
	session       *sessions.Session
	templateCache map[string]*template.Template

	db      *sqlx.DB
	tx      *sqlx.Tx
	statsTx *sqlx.Tx

	SlideStore     *mysql.SlideStore
	GalleryStore   *mysql.GalleryStore
	SlideshowStore *mysql.SlideshowStore
	statisticStore *mysql.StatisticStore
	userStore      *mysql.UserStore

	// common components
	usage *usage.Recorder
	users users.Users

	// HTML sanitizer for titles and captions
	sanitizer *bluemonday.Policy

	// Image processing
	imager *images.Imager

	// Channels to background worker
	chImage   chan images.ReqSave
	chShow    chan reqUpdateShow
	chShows   chan []reqUpdateShow
	chTopic chan reqUpdateTopic

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

	defer app.usage.Stop()

	// start background worker
	// ## how can we shutdown cleanly?
	t := time.NewTicker(app.cfg.SiteRefresh)
	defer t.Stop()

	chDone := make(chan bool, 1)
	go app.galleryState.worker(app.chImage, app.chShow, app.chShows, app.chTopic, t.C, chDone)

	// preconfigured HTTP/HTTPS server
	// ## name stutter!
	srv := &server.Server{
		ErrorLog:  errorLog,
		InfoLog:   infoLog,
		ThreatLog: threatLog,

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
func (app *Application) OnRemoveUser(user *users.User) {

	app.galleryState.OnRemoveUser(user)
}

// Render writes an HTTP response using the specified template and field (embedded as Users).
func (app *Application) Render(w http.ResponseWriter, r *http.Request, template string, usersField interface{}) {
	app.render(w, r, template, &usersFormData{Users: usersField})
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

// Import customisation files

func importFiles(toDir, fromDir string) error {

	files, err := filepath.Glob(fromDir)
	if err != nil {
		return err // no error if dir doesn't exist
	}

	for _, file := range files {

		if err = copyFile(toDir, file); err != nil {
			return err
		}
	}
	return nil
}

// Initialisation, common to live and test

func initialise(cfg *Configuration, errorLog *log.Logger, infoLog *log.Logger, threatLog *log.Logger, db *sqlx.DB) *Application {

	// initialise template cache
	templateCache, err := newTemplateCache(filepath.Join(UIPath, "html"), filepath.Join(SitePath, "templates"))
	if err != nil {
		errorLog.Fatal(err)
	}

	// import custom images
	err = importFiles(filepath.Join(StaticPath, "images"), filepath.Join(SitePath, "images/*.*"))
	if err != nil {
		errorLog.Print(err)
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

	// initialise gallery state
	app.galleryState.Init(app)

	// initialise data stores
	gallery := app.initStores(cfg)

	// cached state
	if err := app.galleryState.setupCache(gallery); err != nil {
		errorLog.Fatal(err)
	}

	// setup image processing
	app.imager = &images.Imager{
		ImagePath: ImagePath,
		MaxW:      app.cfg.MaxW,
		MaxH:      app.cfg.MaxH,
		ThumbW:    app.cfg.ThumbW,
		ThumbH:    app.cfg.ThumbH,
	}

	// setup usage, with defaults
	if app.usage, err = usage.New(app.statisticStore, cfg.UsageAnonymised, 0, 0, 0, 0, 0); err != nil {
		errorLog.Fatal(err)
	}

	// user management
	app.users = users.Users{
		App:   app,
		Roles: []string{"unknown", "friend", "member", "curator", "admin"},
		Store: app.userStore,
	}

	// create worker channels
	app.chImage = make(chan images.ReqSave, 20)
	app.chShow = make(chan reqUpdateShow, 10)
	app.chShows = make(chan []reqUpdateShow, 1)
	app.chTopic = make(chan reqUpdateTopic, 10)

	return app
}

// Initialise data stores

func (app *Application) initStores(cfg *Configuration) *models.Gallery {

	defer app.galleryState.updatesGallery()()

	// setup stores, with reference to a common transaction
	// ## transaction should be per-gallery if we support multiple galleries
	app.SlideStore = mysql.NewSlideStore(app.db, &app.tx, app.errorLog)
	app.GalleryStore = mysql.NewGalleryStore(app.db, &app.tx, app.errorLog)
	app.SlideshowStore = mysql.NewSlideshowStore(app.db, &app.tx, app.errorLog)
	app.statisticStore = mysql.NewStatisticStore(app.db, &app.statsTx, app.errorLog)
	app.userStore = mysql.NewUserStore(app.db, &app.tx, app.errorLog)

	// setup new database and administrator, if needed, and get gallery record
	g, err := mysql.Setup(app.GalleryStore, app.userStore, 1, cfg.AdminName, cfg.AdminPassword)
	if err != nil {
		app.errorLog.Fatal(err)
	}

	// save gallery ID for stores that need it
	app.SlideshowStore.GalleryId = g.Id
	app.userStore.GalleryId = g.Id

	// highlights topic ID
	app.SlideshowStore.HighlightsId = 1

	// database changes from previous version(s)
	topicStore := mysql.NewTopicStore(app.db, &app.tx, app.errorLog)
	topicStore.GalleryId = g.Id
	err = mysql.MigrateTopics(topicStore, app.SlideshowStore, app.SlideStore)
	if err != nil {
		app.errorLog.Fatal(err)
	}

	return g
}

// Make HTTP server

func newServer(addr string, handler http.Handler, log *log.Logger, main bool) *http.Server {

	// common server parameters for HTTP/HTTPS
	s := &http.Server{
		Addr:     addr,
		ErrorLog: log,
		Handler:  handler,
	}

	// set timeouts so that a slow or malicious client doesn't hold resources forever
	if main {

		// These are lax ones, but suggested in
		//   https://medium.com/@simonfrey/go-as-in-golang-standard-net-http-config-will-break-your-production-environment-1360871cb72b
		s.ReadHeaderTimeout = 20 * time.Second // this is the one that matters for SlowLoris?
		// ReadTimeout:  1 * time.Minute, // remove if variable timeouts in handlers
		s.WriteTimeout = 2 * time.Minute // starts after reading of request headers
		s.IdleTimeout = 2 * time.Minute

	} else {
		// tighter limits for HTTP certificate renewal and redirection to HTTPS
		s.ReadTimeout = 5 * time.Second   // remove if variable timeouts in handlers
		s.WriteTimeout = 10 * time.Second // starts after reading of request headers
		s.IdleTimeout = 1 * time.Minute
	}

	return s
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

// Redirect HTTP requests to HTTPS, taken from autocert. Changed to do 301 redirect.

func handleHTTPRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "Use HTTPS", http.StatusBadRequest)
		return
	}
	target := "https://" + stripPort(r.Host) + r.URL.RequestURI()
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return net.JoinHostPort(host, "443")
}
