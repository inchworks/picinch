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
	"errors"
	"time"

	"github.com/inchworks/webparts/users"
	"github.com/jmoiron/sqlx"
	"github.com/go-sql-driver/mysql"

	"inchworks.com/picinch/pkg/models"
)

var cmds = [...]string{

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
	user int(11) NULL,
	shared bigint(20) NOT NULL,
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
	KEY IDX_SHARED (shared),
	KEY IDX_TOPIC (topic),
	CONSTRAINT FK_SLIDESHOW_GALLERY FOREIGN KEY (gallery) REFERENCES gallery (id),
	CONSTRAINT FK_SLIDESHOW_USER FOREIGN KEY (user) REFERENCES user (id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;`,

	`INSERT INTO slideshow (id, gallery, gallery_order, visible, user, shared, topic, created, revised, title, caption, format, image) VALUES
	(1,	1, 10, 2, 0, 0, 0, '2020-04-25 15:52:42', '2020-04-25 15:52:42', 'Highlights', '', 'H.4', '');`,

	`CREATE TABLE statistic (
		id int(11) NOT NULL AUTO_INCREMENT,
		event varchar(60) COLLATE utf8_unicode_ci NOT NULL,
		category varchar(60) COLLATE utf8_unicode_ci NOT NULL,
		count int(11) NOT NULL,
		detail smallint(6) NOT NULL,
		start datetime NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY IDX_STATISTIC (event, start, detail),
		KEY IDX_START_DETAIL (start, detail)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;`,

	`CREATE TABLE tag (
		id int(11) NOT NULL AUTO_INCREMENT,
		gallery int(11) NOT NULL,
		parent int(11) NOT NULL,
		user int(11) NOT NULL,
		name varchar(60) COLLATE utf8_unicode_ci NOT NULL,
		action varchar(60) COLLATE utf8_unicode_ci NOT NULL,
		format varchar(60) COLLATE utf8_unicode_ci NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY IDX_TAG (parent, name, user),
		CONSTRAINT FK_TAG_GALLERY FOREIGN KEY (gallery) REFERENCES gallery (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;`,

	`CREATE TABLE tagref (
		id int(11) NOT NULL AUTO_INCREMENT,
		slideshow int(11) NOT NULL,
		tag int(11) NOT NULL,
		user int(11) NULL,
		added datetime NOT NULL,
		detail varchar(512) COLLATE utf8_unicode_ci NOT NULL,
		PRIMARY KEY (id),
		KEY IDX_TAG_SLIDESHOW (slideshow),
		KEY IDX_TAG_TAG (tag),
		KEY IDX_TAG_USER (user),
		UNIQUE KEY IDX_TAGREF (slideshow, tag, user),
		CONSTRAINT FK_TAG_SLIDESHOW FOREIGN KEY (slideshow) REFERENCES slideshow (id) ON DELETE CASCADE,
		CONSTRAINT FK_TAG_TAG FOREIGN KEY (tag) REFERENCES tag (id) ON DELETE CASCADE,
		CONSTRAINT FK_TAG_USER FOREIGN KEY (user) REFERENCES user (id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;`,

	`CREATE TABLE user (
		id int(11) NOT NULL AUTO_INCREMENT,
		parent int(11) NOT NULL,
		username varchar(60) COLLATE utf8_unicode_ci NOT NULL,
		name varchar(60) COLLATE utf8_unicode_ci NOT NULL,
		role smallint(6) NOT NULL,
		status smallint(6) NOT NULL,
		password char(60) COLLATE utf8_unicode_ci NOT NULL,
		created datetime NOT NULL,
		PRIMARY KEY (id),
		UNIQUE KEY IDX_USERNAME (username),
		KEY IDX_USER_PARENT (parent),
		CONSTRAINT FK_USER_GALLERY FOREIGN KEY (parent) REFERENCES gallery (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci;`,
}

// Setup initialises a new database, if it has no tables.
// It adds a gallery record and the specified administrator if needed, and returns the gallery record.
func Setup(stGallery *GalleryStore, stUser *UserStore, galleryId int64, adminName string, adminPW string) (*models.Gallery, error) {

	// look for gallery record
	g, err := stGallery.Get(galleryId)
	if err != nil && err != models.ErrNoRecord {

		// no gallery table - make the database
		if err = setupTables(stGallery.DBX, *stGallery.ptx); err != nil {
			return nil, err
		}
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

func setupTables(db *sqlx.DB, tx *sqlx.Tx) error {

	for _, cmd := range cmds {
		if _, err := tx.Exec(cmd); err != nil {
			return err
		}
	}
	return nil
}

// MigrateTopics replaces old topic records with corresponding slideshow records.
// Needed for version 0.9.4.
func MigrateTopics(stTopic *TopicStore, stSlideshow *SlideshowStore, stSlide *SlideStore) error {

	var cmdSlideshow = `ALTER TABLE slideshow MODIFY COLUMN shared bigint(20), MODIFY COLUMN user int(11) NULL;`
	var cmdTopic = `DROP TABLE topic;`

	// do we have topics?
	t := stTopic.GetIf(stSlideshow.HighlightsId)
	if t == nil {
		return nil // nothing to do
	}

	// allow null references to user
	tx := *stSlideshow.ptx
	if _, err := tx.Exec(cmdSlideshow); err != nil {
		return err
	}

	// move existing first slideshow, if there is one
	s := stSlideshow.GetIf(stSlideshow.HighlightsId)
	if s != nil {
		oldId := s.Id
		s.Id = 0
		err := stSlideshow.Update(s) // sets s.Id
		if err != nil {
			return err
		}

		// reassign slides
		slides := stSlide.ForSlideshow(oldId, 1000)
		for _, slide := range slides {
			slide.Slideshow = s.Id
			err := stSlide.Update(slide)
			if err != nil {
				return err
			}
		}

		// delete old slideshow
		err = stSlideshow.DeleteId(stSlideshow.HighlightsId)
		if err != nil {
			return err
		}
	}

	// move topics
	ts := stTopic.All()
	for _, t = range ts {

		// corresponding slideshow for topic
		topicShow := &models.Slideshow{
			Gallery:      t.Gallery,
			GalleryOrder: t.GalleryOrder,
			Visible:      t.Visible,
			Shared:       t.Shared,
			Created:      t.Created,
			Revised:      t.Revised,
			Title:        t.Title,
			Caption:      t.Caption,
			Format:       t.Format,
			Image:        t.Image,
		}

		var err error

		// preserve highlights ID, and use new ordering scheme
		if t.Id == stSlideshow.HighlightsId {
			topicShow.Id = stSlideshow.HighlightsId
			topicShow.GalleryOrder = 10
			err = stSlideshow.Set(topicShow)

		} else {
			err = stSlideshow.Update(topicShow)
		}
		if err != nil {
			return err
		}

		// reassign slideshows for topic
		var latest time.Time
		ss := stSlideshow.ForTopic(t.Id)
		for _, s = range ss {
			s.Topic = topicShow.Id
			s.Visible = models.SlideshowTopic
			s.GalleryOrder = 5
			if err := stSlideshow.Update(s); err != nil {
				return err
			}
			// latest revision, for highlights topic
			if s.Revised.After(latest) {
				latest = s.Revised
			}
		}

		// change creation date for highlights topic
		if topicShow.Id == stSlideshow.HighlightsId {
			if latest.After(topicShow.Created) {
				topicShow.Created = latest
				topicShow.Revised = latest
				if err := stSlideshow.Update(topicShow); err != nil {
					return err
				}
			}
		}
	}

	// delete topics table
	if _, err := tx.Exec(cmdTopic); err != nil {
		return err
	}

	return nil
}

// MigrateWebparts1 upgrades the database with changes needed by inchworks/webparts,
// before first table access. Needed for version 0.9.4.
func MigrateWebparts1(tx *sqlx.Tx) error {

	var cmdUser1 =
		`ALTER TABLE user
		DROP FOREIGN KEY FK_USER_GALLERY,
		CHANGE COLUMN gallery parent int(11),
		ADD COLUMN role smallint(6) NOT NULL;`

	var cmdUser2 =
		`ALTER TABLE user
		ADD CONSTRAINT FK_USER_GALLERY FOREIGN KEY (parent) REFERENCES gallery (id);`

	// new user table definition, if needed
	_, err := tx.Exec(cmdUser1)
	if driverErr, ok := err.(*mysql.MySQLError); ok {
		if driverErr.Number == 1054 || driverErr.Number == 1146 {	
			return nil // ER_BAD_FIELD_ERROR is expected
		}
	}
	if err != nil {
		return err
	}

	// reinstate foreign key (cannot be done in same command - I hate SQL)
	_, err = tx.Exec(cmdUser2)
 
	return err
}

// MigrateWebparts2 upgrades the database with changes needed by inchworks/webparts,
// after stores are ready. Needed for version 0.9.4.
func MigrateWebparts2(stUser *UserStore, tx *sqlx.Tx) error {

	var cmdStatistic = `ALTER TABLE statistic CHANGE COLUMN period detail smallint(6);`

	// has statistics column been renamed yet?
	if _, err := tx.Exec(cmdStatistic); err != nil {
		return nil
	}

	// assign roles for all users
	us := stUser.All()
	for _, u := range us {

		switch u.Status {
		case 0, 1, 2: // Suspended, Known, Active
			// don't overwrite a newly added admin
			if u.Role == 0 {
				u.Role = models.UserMember
			}

		case 3: // Curator
			u.Status = users.UserActive
			u.Role = models.UserCurator

		case 4: // Admin
			u.Status = users.UserActive
			u.Role = models.UserAdmin

		default:
			return errors.New("Unknown user status")
		}

		if err := stUser.Update(u); err != nil {
			return err
		}
	}

	return nil
}