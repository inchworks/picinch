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

	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/v2"
	_ "github.com/go-sql-driver/mysql"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/inchworks/usage"
	"github.com/inchworks/webparts/v2/etx"
	"github.com/inchworks/webparts/v2/limithandler"
	"github.com/inchworks/webparts/v2/multiforms"
	"github.com/inchworks/webparts/v2/server"
	"github.com/inchworks/webparts/v2/stack"
	"github.com/inchworks/webparts/v2/uploader"
	"github.com/inchworks/webparts/v2/users"
	"github.com/jmoiron/sqlx"
	"github.com/justinas/nosurf"
	"github.com/microcosm-cc/bluemonday"

	"inchworks.com/picinch/internal/cache"
	"inchworks.com/picinch/internal/emailer"
	"inchworks.com/picinch/internal/models"
	"inchworks.com/picinch/internal/models/mysql"

	"inchworks.com/picinch/internal/tags"
	"inchworks.com/picinch/web"
)

// version and copyright
const (
	version = "1.3.0"
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

	// new DSN
	DBSource   string `yaml:"db-source" env:"db-source" env-default:"tcp(picinch_db:3306)/picinch"`
	DBUser     string `yaml:"db-user" env:"db-user" env-default:"server"`
	DBPassword string `yaml:"db-password" env:"db-password" env-default:"<server-password>"`

	// administrator
	AdminName     string `yaml:"admin-name" env:"admin-name" env-default:""`
	AdminPassword string `yaml:"admin-password" env:"admin-password" env-default:"<your-password>"`

	// image sizes
	MaxW       int `yaml:"image-width" env-default:"1600"`     // maximum stored image dimensions
	MaxH       int `yaml:"image-height" env-default:"1200"`    //
	MaxAV      int `yaml:"max-audio-visual" env-default:"16"`  // maximum unprocessed AV file size (megabytes)
	ThumbW     int `yaml:"thumbnail-width" env-default:"278"`  // thumbnail size
	ThumbH     int `yaml:"thumbnail-height" env-default:"208"` //
	MaxDecoded int `yaml:"max-decoded" env-default:"512"`      // maximum decoded image size (megabytes)
	MaxUpload  int `yaml:"max-upload" env-default:"64"`        // maximum file upload (megabytes)

	// total limits
	MaxHighlightsParent int `yaml:"parent-highlights"  env-default:"16"` // highlights for parent website
	MaxHighlightsTotal  int `yaml:"highlights-page" env-default:"12"`    // highlights for home page, and user's page
	MaxHighlightsTopic  int `yaml:"highlights-topic" env-default:"32"`   // slides in highights slideshow
	MaxNextEvents       int `yaml:"events-page" env-default:"1"`         // total events on home page
	MaxSlideshowsTotal  int `yaml:"slideshows-page" env-default:"16"`    // total slideshows on home page

	// per user limits
	MaxHighlights       int `yaml:"highlights-user"  env-default:"2"`  // highlights on home page
	MaxSlides           int `yaml:"slides-show" env-default:"50"`      // slides in a slideshow
	MaxSlidesTopic      int `yaml:"slides-topic" env-default:"8"`      // slides in a topic contribution
	MaxSlideshowsClub   int `yaml:"slideshows-club"  env-default:"2"`  // club slideshows on home page, per user
	MaxSlideshowsPublic int `yaml:"slideshows-public" env-default:"1"` // public slideshows on home page, per user

	// operational settings
	AllowedQueries    []string        `yaml:"allowed-queries" env-default:"fbclid"`                            // URL query names allowed
	BanBadFiles       bool            `yaml:"limit-bad-files" env-default:"false"`                             // apply ban to requests for missing media files
	DropDelay         time.Duration   `yaml:"drop-delay" env:"drop-delay" env-default:"8h"`                    // delay before access drops and deletes are finalised. Units h.
	GeoBlock          []string        `yaml:"geo-block" env:"geo-block" env-default:""`                        // blocked countries (ISO 3166-1 alpha-2 codes)
	MaxCacheAge       time.Duration   `yaml:"max-cache-age" env:"max-cache-age" env-default:"1h"`              // browser cache control, maximum age. Units s, m or h.
	MaxUnvalidatedAge time.Duration   `yaml:"max-unvalidated-age" env:"max-unvalidated-age" env-default:"48h"` // maximum time for a competition entry to be validated. Units h.
	MaxUploadAge      time.Duration   `yaml:"max-upload-age" env:"max-upload-age" env-default:"8h"`            // maximum time for a slideshow update. Units m or h.
	SiteRefresh       time.Duration   `yaml:"thumbnail-refresh"  env-default:"1h"`                             // refresh interval for topic thumbnails. Units m or h.
	TimeoutDownload   time.Duration   `yaml:"timeout-download" env-default:"2m"`                               // maximum time for file download. Units m.
	TimeoutUpload     time.Duration   `yaml:"timeout-upload" env-default:"5m"`                                 // maximum time for file upload. Units m.
	TimeoutWeb        time.Duration   `yaml:"timeout-web" env-default:"20s"`                                   // maximum time for web request, same for response (default). Units s or m.
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

// Operation to finalise deletion or reduction in access of a slideshow or user.
type OpDrop struct {
	Id     int64
	Access int
}

// Operation to release slideshow from topic.
type OpReleaseShow struct {
	ShowId  int64
	TopicId int64
}

// Operation to claim slideshow images.
type OpUpdateShow struct {
	ShowId  int64
	TopicId int64
	Revised bool
}

// Operation to update topic images.
type OpUpdateTopic struct {
	TopicId int64
	Revised bool
	tx      etx.TxId
}

// Operation to validate slideshow submission.
type OpValidate struct {
	ShowId int64
}

// Application struct supplies application-wide dependencies.
type Application struct {
	cfg *Configuration

	errorLog      *log.Logger
	infoLog       *log.Logger
	threatLog     *log.Logger
	session       *scs.SessionManager
	templateCache map[string]*template.Template

	// database
	db      *sqlx.DB
	tx      *sqlx.Tx
	statsTx *sqlx.Tx

	GalleryStore   *mysql.GalleryStore
	PageStore      *mysql.PageStore
	redoStore      *mysql.RedoStore
	redoV1Store    *mysql.RedoV1Store
	SlideStore     *mysql.SlideStore
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

	// worker
	chTopic chan OpUpdateTopic

	// HTML sanitizer for titles and captions
	sanitizer *bluemonday.Policy

	// private components
	publicPages *cache.PageCache
	emailer     emailer.Emailer
	tagger      tags.Tagger
	staticFS    fs.FS

	// HTML handlers for threats detected by application logic
	wrongCode http.Handler

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
	app.chTopic = make(chan OpUpdateTopic, 4)
	go app.galleryState.worker(app.chTopic, tr.C, tp.C, chDone)

	// redo any pending operations
	infoLog.Print("Starting operation recovery")
	if app.redoV1Store != nil {
		if err := app.tm.RecoverV1(app.redoV1Store, &app.galleryState, app.uploader); err != nil {
			errorLog.Fatal(err)
		}
		app.uploader.V1() // uploader should request timeouts
	}
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

		Timeout: cfg.TimeoutWeb,
	}

	srv.Serve(app)
}

// ** INTERFACE FUNCTIONS FOR WEBPARTS/USERS **

// Authenticated adds a logged-in user's ID to the session.
func (app *Application) Authenticated(r *http.Request, id int64) {

	// renew session token on privilege level change, to prevent session fixation attack
	if err := app.session.RenewToken(r.Context()); err != nil {
		app.Log(err)
	}

	app.session.Put(r.Context(), "authenticatedUserID", id)
}

// Flash adds a confirmation message to the next page, via the session.
func (app *Application) Flash(r *http.Request, msg string) {
	app.session.Put(r.Context(), "flash", msg)
}

// GetRedirect returns the next page after log-in.
func (app *Application) GetRedirect(r *http.Request) string {
	url := app.session.PopString(r.Context(), "afterLogin")
	if url == "" {
		url = "/members"
	}
	return url
}

// Log optionally records an error.
func (app *Application) Log(err error) {
	app.errorLog.Print(err)
}

// LogThreat optionally records a rejected request to sign-up or log-in.
func (app *Application) LogThreat(msg string, r *http.Request) {
	app.threat(msg, r)
}

// OnAddUser is called to add any additional application data for a user.
func (app *Application) OnAddUser(tx etx.TxId, user *users.User) {
	// not needed for this application
}

// OnRemoveUser is called to delete any application data for a user.
func (app *Application) OnRemoveUser(tx etx.TxId, user *users.User) {

	app.galleryState.onRemoveUser(tx, user)
}

// OnUpdateUser is called to change any application data for a modified user.
func (app *Application) OnUpdateUser(tx etx.TxId, from *users.User, to *users.User) {

	app.galleryState.onUpdateUser(tx, from, to)
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

type UserNoDelete struct {
	*mysql.UserStore
}

func (st *UserNoDelete) DeleteId(id int64) error {
	return nil // deletion of users is deferred
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

	// access to items removed should be for longer than the cache time
	if cfg.DropDelay < cfg.MaxCacheAge {
		cfg.DropDelay = cfg.MaxCacheAge * 2
	}

	// dependency injection
	app := &Application{
		cfg:           cfg,
		errorLog:      errorLog,
		infoLog:       infoLog,
		threatLog:     threatLog,
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
	app.staticFS, err = stack.NewFS(staticForms, staticUploader, staticApp, os.DirFS(SitePath))
	if err != nil {
		errorLog.Fatal(err)
	}

	// initialise gallery state
	app.galleryState.Init(app)

	// initialise data stores
	gallery := app.initStores(cfg)

	// initialise session manager
	app.session = initSession(len(cfg.Domains) > 0, db)

	// cached state
	if err := app.galleryState.setupCache(gallery); err != nil {
		errorLog.Fatal(err)
	}
	warn := app.galleryState.cachePages()
	if len(warn) > 0 {
		infoLog.Print("Conflicting page menu items:")
		for _, w := range warn {
			infoLog.Print("\t" + w)
		}
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

	// setup extended transaction manager
	app.tm = etx.New(app, app.redoStore)

	// setup media upload processing
	app.uploader = &uploader.Uploader{
		FilePath:     ImagePath,
		MaxW:         app.cfg.MaxW,
		MaxH:         app.cfg.MaxH,
		MaxDecoded:   app.cfg.MaxDecoded * 1024 * 1024,
		MaxSize:      app.cfg.MaxAV * 1024 * 1024,
		ThumbW:       app.cfg.ThumbW,
		ThumbH:       app.cfg.ThumbH,
		DeleteAfter:  app.cfg.DropDelay,
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
		Store: &UserNoDelete{UserStore: app.userStore}, // ignores DeleteId
		TM:    app.tm,
	}

	// geo-blocking
	app.geoblocker = &server.GeoBlocker{
		ErrorLog:     errorLog,
		ReportSingle: true,
		Store:        GeoDBPath,
	}
	app.geoblocker.Start(cfg.GeoBlock)

	return app
}

// initSession returns the session manager.
func initSession(live bool, db *sqlx.DB) *scs.SessionManager {

	sm := scs.New()

	sm.Cookie.Name = "session_v2" // changed from previous implementation
	sm.Lifetime = 24 * time.Hour
	sm.Store = mysqlstore.New(db.DB)

	// secure cookie over HTTPS except in test
	if live {
		sm.Cookie.Secure = true
	}

	return sm
}

// Initialise data stores

func (app *Application) initStores(cfg *Configuration) *models.Gallery {

	defer app.galleryState.updatesGallery()()

	// setup stores, with reference to a common transaction
	// ## transaction should be per-gallery if we support multiple galleries
	app.PageStore = mysql.NewPageStore(app.db, &app.tx, app.errorLog)
	app.SlideStore = mysql.NewSlideStore(app.db, &app.tx, app.errorLog)
	app.GalleryStore = mysql.NewGalleryStore(app.db, &app.tx, app.errorLog)
	app.redoStore = mysql.NewRedoStore(app.db, &app.tx, app.errorLog)
	app.SlideshowStore = mysql.NewSlideshowStore(app.db, &app.tx, app.errorLog)
	app.statisticStore = mysql.NewStatisticStore(app.db, &app.statsTx, app.errorLog)
	app.tagger.TagStore = mysql.NewTagStore(app.db, &app.tx, app.errorLog)
	app.tagger.TagRefStore = mysql.NewTagRefStore(app.db, &app.tx, app.errorLog)
	app.userStore = mysql.NewUserStore(app.db, &app.tx, app.errorLog)

	// this is to handle V1 transactions from before upgrade, to be deleted if not needed
	app.redoV1Store = mysql.NewRedoV1Store(app.db, &app.tx, app.errorLog)

	// setup new database and administrator, if needed, and get gallery record
	g, err := mysql.Setup(app.GalleryStore, app.userStore, 1, cfg.AdminName, cfg.AdminPassword)
	if err != nil {
		app.errorLog.Fatal(err)
	}

	// save gallery ID for stores that need it, and link stores that update joins
	app.PageStore.GalleryId = g.Id
	app.SlideshowStore.GalleryId = g.Id
	app.tagger.TagStore.GalleryId = g.Id
	app.userStore.GalleryId = g.Id
	app.PageStore.SlideshowStore = app.SlideshowStore

	// highlights topic ID
	app.SlideshowStore.HighlightsId = 1

	// database changes from previous version(s) after v1.0
	if err = mysql.MigrateRedo2(app.redoStore, app.SlideshowStore); err != nil {
		app.errorLog.Fatal(err)
	}
	if !mysql.MigrateRedoV1(app.redoV1Store) {
		app.redoV1Store = nil
	}
	if err = mysql.MigrateSessions(mysql.NewSessionStore(app.db, &app.tx, app.errorLog)); err != nil {
		app.errorLog.Fatal(err)
	}
	if err = mysql.MigrateInfo(app.userStore, app.SlideshowStore, app.PageStore); err != nil {
		app.errorLog.Fatal(err)
	}
	if err = mysql.MigrateMB4(app.GalleryStore); err != nil {
		app.errorLog.Fatal(err)
	}

	app.userStore.InitSystem()
	
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
