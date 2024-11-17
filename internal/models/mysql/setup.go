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

package mysql

// Setup application database

import (
	"database/sql"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/inchworks/webparts/v2/users"
	"github.com/jmoiron/sqlx"

	"inchworks.com/picinch/internal/models"
)

var cmds = [...]string{

	"SET NAMES 'utf8mb4' COLLATE 'utf8mb4_unicode_ci';",

	"SET character_set_server = 'utf8mb4';",

	"SET collation_server = 'utf8mb4_unicode_ci';",

	"SET time_zone = '+00:00';",

	"SET foreign_key_checks = 0;",

	"SET sql_mode = 'NO_AUTO_VALUE_ON_ZERO';",

	`CREATE TABLE gallery (
	id int(11) NOT NULL AUTO_INCREMENT,
	organiser varchar(60) COLLATE utf8_unicode_ci NOT NULL,
	n_max_slides int(11) NOT NULL,
	n_showcased int(11) NOT NULL,
	PRIMARY KEY (id));`,

	`INSERT INTO gallery (id, version, organiser, n_max_slides, n_showcased) VALUES
	(1,	1, 'PicInch Gallery', 10, 2);`,

	`CREATE TABLE redoV2 (
		id BIGINT NOT NULL,
		tx BIGINT NOT NULL,
		manager varchar(32) NOT NULL,
		redotype int(11) NOT NULL,
		delay int(11) NOT NULL,
		optype int(11) NOT NULL,
		operation JSON NOT NULL,
		PRIMARY KEY (id));`,

	`CREATE TABLE sessions (
		token CHAR(43) PRIMARY KEY,
		data BLOB NOT NULL,
		expiry TIMESTAMP(6) NOT NULL);`,
	
	`CREATE INDEX sessions_expiry_idx ON sessions (expiry);`,
	
	`CREATE TABLE slide (
	id int(11) NOT NULL AUTO_INCREMENT,
	slideshow int(11) NOT NULL,
	format int(11) NOT NULL,
	show_order int(11) NOT NULL,
	created datetime NOT NULL,
	revised datetime NOT NULL,
	title varchar(512) NOT NULL,
	caption varchar(512) NOT NULL,
	image varchar(256) NOT NULL,
	PRIMARY KEY (id),
	KEY IDX_SLIDESHOW (slideshow),
	CONSTRAINT FK_SLIDESHOW FOREIGN KEY (slideshow) REFERENCES slideshow (id) ON DELETE CASCADE);`,

	`CREATE TABLE slideshow (
	id int(11) NOT NULL AUTO_INCREMENT,
	gallery int(11) NOT NULL,
	gallery_order int(11) NOT NULL,
	access smallint(6) NOT NULL,
	visible smallint(6) NOT NULL,
	user int(11) NULL,
	shared bigint(20) NOT NULL,
	topic int(11) NOT NULL,
	created datetime NOT NULL,
	revised datetime NOT NULL,
	title varchar(128) NOT NULL,
	caption varchar(512) NOT NULL,
	format varchar(16) NOT NULL,
	image varchar(256) NOT NULL,
	etag varchar(64) NOT NULL,
	PRIMARY KEY (id),
	KEY IDX_SLIDESHOW_GALLERY (gallery),
	KEY IDX_USER (user),
	KEY IDX_SHARED (shared),
	KEY IDX_TOPIC (topic),
	CONSTRAINT FK_SLIDESHOW_GALLERY FOREIGN KEY (gallery) REFERENCES gallery (id),
	CONSTRAINT FK_SLIDESHOW_USER FOREIGN KEY (user) REFERENCES user (id) ON DELETE CASCADE)`,

	`INSERT INTO slideshow (id, gallery, gallery_order, visible, user, shared, topic, created, revised, title, caption, format, image) VALUES
	(1,	1, 10, 2, NULL, 0, 0, '2020-04-25 15:52:42', '2020-04-25 15:52:42', 'Highlights', '', 'H.4', '');`,

	`CREATE TABLE statistic (
		id int(11) NOT NULL AUTO_INCREMENT,
		event varchar(60) NOT NULL,
		category varchar(60) NOT NULL,
		count int(11) NOT NULL,
		detail smallint(6) NOT NULL,
		start datetime NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY IDX_STATISTIC (event, start, detail),
		KEY IDX_START_DETAIL (start, detail));`,

	`CREATE TABLE tag (
		id int(11) NOT NULL AUTO_INCREMENT,
		gallery int(11) NOT NULL,
		parent int(11) NOT NULL,
		name varchar(60) NOT NULL,
		action varchar(60) NOT NULL,
		format varchar(60) NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY IDX_TAG (gallery, parent, name),
		CONSTRAINT FK_TAG_GALLERY FOREIGN KEY (gallery) REFERENCES gallery (id));`,

	`INSERT INTO tag (id, gallery, parent, name, action, format) VALUES
		(1, 1, 0, 'new', '', ''),
		(2, 1, 0, 'agreements', '', '');`,

	`CREATE TABLE tagref (
		id int(11) NOT NULL AUTO_INCREMENT,
		item int(11) NULL,
		tag int(11) NOT NULL,
		user int(11) NULL,
		added datetime NOT NULL,
		detail varchar(512) NOT NULL,
		PRIMARY KEY (id),
		KEY IDX_TAG_ITEM (item),
		KEY IDX_TAG_TAG (tag),
		KEY IDX_TAG_USER (user),
		UNIQUE KEY IDX_TAGREF (item, tag, user),
		CONSTRAINT FK_TAG_SLIDESHOW FOREIGN KEY (item) REFERENCES slideshow (id) ON DELETE CASCADE,
		CONSTRAINT FK_TAG_TAG FOREIGN KEY (tag) REFERENCES tag (id) ON DELETE CASCADE,
		CONSTRAINT FK_TAG_USER FOREIGN KEY (user) REFERENCES user (id) ON DELETE CASCADE);`,

	`CREATE TABLE user (
		id int(11) NOT NULL AUTO_INCREMENT,
		parent int(11) NOT NULL,
		username varchar(60) NOT NULL,
		name varchar(60) NOT NULL,
		role smallint(6) NOT NULL,
		status smallint(6) NOT NULL,
		password char(60) NOT NULL,
		created datetime NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY IDX_USERNAME (username),
		KEY IDX_USER_PARENT (parent),
		CONSTRAINT FK_USER_GALLERY FOREIGN KEY (parent) REFERENCES gallery (id));`,
}

var cmdsRedo = [...]string{

	`CREATE TABLE redoV2 (
		id BIGINT NOT NULL,
		tx BIGINT NOT NULL,
		manager varchar(32) NOT NULL,
		redotype int(11) NOT NULL,
		delay int(11) NOT NULL,
		optype int(11) NOT NULL,
		operation JSON NOT NULL,
		PRIMARY KEY (id));`,

	`ALTER TABLE slideshow
		ADD COLUMN access smallint(6) NOT NULL,
		ADD COLUMN etag varchar(64) NOT NULL;`,
}

var cmdsSessions = [...]string{
	`CREATE TABLE sessions (
		token CHAR(43) PRIMARY KEY,
		data BLOB NOT NULL,
		expiry TIMESTAMP(6) NOT NULL);`,
	
	`CREATE INDEX sessions_expiry_idx ON sessions (expiry);`,
}

// Setup initialises a new database, if it has no tables.
// It adds a gallery record and the specified administrator if needed, and returns the gallery record.
func Setup(stGallery *GalleryStore, stUser *UserStore, galleryId int64, adminName string, adminPW string) (*models.Gallery, error) {

	// look for gallery record
	g, err := stGallery.Get(galleryId)
	if err != nil {
		if driverErr, ok := err.(*mysql.MySQLError); ok {
			if driverErr.Number == 1146 {

				// no gallery table - make the database
				err = setupTables(stGallery.DBX, *stGallery.ptx, cmds[:])
			}
		} else if stGallery.convertError(err) != models.ErrNoRecord {
			// ok if no gallery record yet
			err = nil
		}
	}

	if err != nil {
		return nil, stGallery.logError(err)
	}

	if g == nil {
		// create first gallery
		g = &models.Gallery{Id: 1}
		if err = stGallery.Update(g); err != nil {
			return nil, err
		}
	}

	// look for admin user
	stUser.GalleryId = g.Id
	admin, err := stUser.GetNamed(adminName)
	if err != nil && err != models.ErrNoRecord {
		return nil, err
	}

	if admin == nil && len(adminName) > 0 {

		// configured admin user doesn't exist - add one
		if err := setupAdmin(stUser, adminName, adminPW); err != nil {
			return nil, err
		}

	}
	return g, nil
}

// create admin user

func setupAdmin(st *UserStore, adminName string, adminPW string) error {

	admin := &users.User{
		Username: adminName,
		Name:     "Administrator",
		Role:     models.UserAdmin,
		Status:   users.UserActive,
		Created:  time.Now(),
	}
	if err := admin.SetPassword(adminPW); err != nil {
		return err
	}

	if err := st.Update(admin); err != nil {
		return err
	}

	return nil
}

// create database tables

func setupTables(_ *sqlx.DB, tx *sqlx.Tx, cmds []string) error {

	for _, cmd := range cmds {
		if _, err := tx.Exec(cmd); err != nil {
			return err
		}
	}
	return nil
}

// MigrateRedo2 adds the redo V2 table, and upgrades the slideshow table. Needed for version 1.1.0.
func MigrateRedo2(stRedo *RedoStore, stSlideshow *SlideshowStore) error {

	if _, err := stRedo.Count(); err == nil {
		return nil
	}

	if err := setupTables(stRedo.DBX, *stRedo.ptx, cmdsRedo[:]); err != nil {
		return err
	}

	// initialise slideshow access fields
	ss := stSlideshow.All()
	for _, s := range ss {

		s.Access = s.Visible

		if err := stSlideshow.Update(s); err != nil {
			return err
		}
	}
	
	return nil
}

// MigrateRedoV1 checks to see if we have a V1 redo table with records, as created before version 1.1.0.
func MigrateRedoV1(stRedoV1 *RedoV1Store) bool {

	n, err := stRedoV1.Count()
	return err == nil && n > 0
}

// MigrateSessions adds a sessions table. Needed for version 1.2.1.
func MigrateSessions(stSession *SessionStore) error {

	if _, err := stSession.Count(); err != nil {
		return setupTables(stSession.DBX, *stSession.ptx, cmdsSessions[:])
	}
	return nil
}

// MigrateInfo adds the user and slideshows for club information. Needed for version 1.3.0.
func MigrateInfo(stUser *UserStore, stSlideshow *SlideshowStore) error {

	infoName := "Info"

	// dummy user to own gallery information
	_, err := stUser.GetNamed(infoName)
	if err == nil || err != models.ErrNoRecord {
		return err
	}

	u := &users.User{
		Username: infoName,
		Name:     infoName,
		Role:     models.UserSystem,
		Status:   users.UserActive,
		Password: []byte(""),
		Created:  time.Now(),
	}

	if err := stUser.Update(u); err != nil {
		return err
	}

	// events
	t := time.Now()
	e := &models.Slideshow{
		GalleryOrder: 10,
		Access: models.SlideshowSystem, 
		Visible: models.SlideshowSystem,
		User:         sql.NullInt64{Int64: u.Id, Valid: true},
		Created:      t,
		Revised:      t,
		Title: "Events",
		Format: "E",
	}

	if err = stSlideshow.Update(e); err != nil {
		return err
	}
	stSlideshow.EventsId = e.Id
	return nil
}

// MigrateMB4 converts text fields to accept 4-byte Unicode characters, instead of 3-byte.
// It also adds a database version for future migrations. Needed for version 1.3.0.
func MigrateMB4(stGallery *GalleryStore) error {

	var cmd1 = `ALTER TABLE gallery ADD COLUMN version smallint(6);`

	var cmds2 = [...]string{
		"SET NAMES 'utf8mb4' COLLATE 'utf8mb4_unicode_ci';",
		"SET character_set_server = 'utf8mb4';",
		"SET collation_server = 'utf8mb4_unicode_ci';",
	
		`ALTER TABLE gallery CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;`,
		`ALTER TABLE slide CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;`,
		`ALTER TABLE slideshow CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;`,
		`ALTER TABLE tag CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;`,
		`ALTER TABLE tagref CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;`,
		`ALTER TABLE user CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;`,

		`UPDATE gallery SET version = 1;`,
	}

	// add database version
	tx := *stGallery.ptx
	if _, err := tx.Exec(cmd1); err != nil {
		if err.(*mysql.MySQLError).Number == 1060 {
			return nil // duplicate column - already migrated
		} else {
			return err
		}
	}

	// set v1 and convert tables
	return setupTables(stGallery.DBX, *stGallery.ptx, cmds2[:])
}
