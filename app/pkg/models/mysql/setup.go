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
	"time"
	"github.com/jmoiron/sqlx"

	"inchworks.com/picinch/pkg/models"
)

var cmds = [...]string {

	"SET NAMES utf8;",

	"SET time_zone = '+00:00';",

	"SET foreign_key_checks = 0;",

	"SET sql_mode = 'NO_AUTO_VALUE_ON_ZERO';",

	`CREATE TABLE gallery (
	id int(11) NOT NULL AUTO_INCREMENT,
	organiser varchar(60) COLLATE utf8_unicode_ci NOT NULL,
	n_max_slides int(11) NOT NULL,
	n_showcased int(11) NOT NULL,
	PRIMARY KEY (id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;`,

	`INSERT INTO gallery (id, organiser, n_max_slides, n_showcased) VALUES
	(1,	'PicInch Gallery', 10, 2);`,

	`CREATE TABLE slide (
	id int(11) NOT NULL AUTO_INCREMENT,
	slideshow int(11) NOT NULL,
	format int(11) NOT NULL,
	show_order int(11) NOT NULL,
	created datetime NOT NULL,
	revised datetime NOT NULL,
	title varchar(512) COLLATE utf8_unicode_ci NOT NULL,
	caption varchar(512) COLLATE utf8_unicode_ci NOT NULL,
	image varchar(256) COLLATE utf8_unicode_ci NOT NULL,
	PRIMARY KEY (id),
	KEY IDX_SLIDESHOW (slideshow),
	CONSTRAINT FK_SLIDESHOW FOREIGN KEY (slideshow) REFERENCES slideshow (id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;`,


	`CREATE TABLE slideshow (
	id int(11) NOT NULL AUTO_INCREMENT,
	gallery int(11) NOT NULL,
	gallery_order int(11) NOT NULL,
	visible smallint(6) NOT NULL,
	user int(11) NOT NULL,
	shared int(11) NOT NULL,
	topic int(11) NOT NULL,
	created datetime NOT NULL,
	revised datetime NOT NULL,
	title varchar(128) COLLATE utf8_unicode_ci NOT NULL,
	caption varchar(512) COLLATE utf8_unicode_ci NOT NULL,
	format varchar(16) COLLATE utf8_unicode_ci NOT NULL,
	image varchar(256) COLLATE utf8_unicode_ci NOT NULL,
	PRIMARY KEY (id),
	KEY IDX_SLIDESHOW_GALLERY (gallery),
	KEY IDX_USER (user),
	KEY IDX_TOPIC (topic),
	KEY IDX_TITLE (title),
	CONSTRAINT FK_SLIDESHOW_GALLERY FOREIGN KEY (gallery) REFERENCES gallery (id),
	CONSTRAINT FK_SLIDESHOW_USER FOREIGN KEY (user) REFERENCES user (id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;`,

	`CREATE TABLE statistic (
		id int(11) NOT NULL AUTO_INCREMENT,
		event varchar(60) COLLATE utf8_unicode_ci NOT NULL,
		category varchar(60) COLLATE utf8_unicode_ci NOT NULL,
		count int(11) NOT NULL,
		start datetime NOT NULL,
		period smallint(6) NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY IDX_STATISTIC (event, start, period),
		KEY IDX_START_PERIOD (start, period)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;`,

	`CREATE TABLE topic (
		id int(11) NOT NULL AUTO_INCREMENT,
		gallery int(11) NOT NULL,
		gallery_order int(11) NOT NULL,
		visible smallint(6) NOT NULL,
		shared int(11) NOT NULL,
		created datetime NOT NULL,
		revised datetime NOT NULL,
		title varchar(128) COLLATE utf8_unicode_ci NOT NULL,
		caption varchar(512) COLLATE utf8_unicode_ci NOT NULL,
		format varchar(16) COLLATE utf8_unicode_ci NOT NULL,
		image varchar(256) COLLATE utf8_unicode_ci NOT NULL,
		PRIMARY KEY (id),
		KEY IDX_TOPIC_GALLERY (gallery),
		CONSTRAINT FK_TOPIC_GALLERY FOREIGN KEY (gallery) REFERENCES gallery (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;`,

	`INSERT INTO topic (id, gallery, gallery_order, visible, shared, created, revised, title, caption, format, image) VALUES
	(1,	1,	1,	2,	0,	'2020-04-25 15:52:42',	'2020-04-25 15:52:42',	'Highlights',	'',	'H.4', '');`,

	`CREATE TABLE user (
		id int(11) NOT NULL AUTO_INCREMENT,
		gallery int(11) NOT NULL,
		username varchar(60) COLLATE utf8_unicode_ci NOT NULL,
		name varchar(60) COLLATE utf8_unicode_ci NOT NULL,
		status smallint(6) NOT NULL,
		password char(60) COLLATE utf8_unicode_ci NOT NULL,
		created datetime NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY IDX_USERNAME (username),
		KEY IDX_USER_GALLERY (gallery),
		CONSTRAINT FK_USER_GALLERY FOREIGN KEY (gallery) REFERENCES gallery (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;`,

}

// Setup new database, if it has no tables.
// Add specifed administrator.

func Setup(st *UserStore, adminName string, adminPW string) error {

	// ## should look this up, but I'm planning to get rid of the gallery record anyway
	st.GalleryId = 1

	// setup database in one transaction
	var commit bool
	tx := st.DBX.MustBegin()
	st.ptx = &tx

	// commit or rollback at the end
	defer func() {
		if commit {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	// look for admin user
	admin, err := st.GetNamed(adminName)
	if err != nil && err != models.ErrNoRecord {

		// no user table - make the database
		if err = setupTables(st.DBX, tx); err != nil {
			return err
		}
	}
	if admin == nil && len(adminName) > 0 {

		// configured admin user doesn't exist - add one
		if err := setupAdmin(st, adminName, adminPW); err != nil {
			return err
		}

	}

	// successful setup
	commit = true
	return nil
}

// create admin user

func setupAdmin(st *UserStore, adminName string, adminPW string) error {

	admin := &models.User{
		Username: adminName,
		Name: "Administrator",
		Status:   models.UserAdmin,
		Created: time.Now(),
	
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

func setupTables(db *sqlx.DB, tx *sqlx.Tx) error {

	for _, cmd := range cmds {
		if _, err := tx.Exec(cmd); err != nil {
			return err
		}
	}
	return nil
}