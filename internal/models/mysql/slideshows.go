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

import (
	"log"
	"time"

	"github.com/jmoiron/sqlx"

	"inchworks.com/picinch/internal/models"
)

const (
	slideshowDelete = `DELETE FROM slideshow WHERE id = ?`

	slideshowInsert = `
		INSERT INTO slideshow (gallery, gallery_order, access, visible, user, shared, topic, created, revised, title, caption, format, image)
		VALUES (:gallery, :gallery_order, :access, :visible, :user, :shared, :topic, :created, :revised, :title, :caption, :format, :image)`

	slideshowUpdate = `
		UPDATE slideshow
		SET gallery_order=:gallery_order, access=:access, visible=:visible, shared=:shared, topic=:topic, created=:created, revised=:revised, title=:title, caption=:caption, format=:format, image=:image
		WHERE id = :id
	`

	slideshowSet = `
		INSERT INTO slideshow (id, gallery, gallery_order, access, visible, user, shared, topic, created, revised, title, caption, format, image)
		VALUES (:id, :gallery, :gallery_order, :access, :visible, :user, :shared, :topic, :created, :revised, :title, :caption, :format, :image)`
)

const (
	// note that ID is included for stable ordering of selections for editing
	slideshowSelect       = `SELECT * FROM slideshow`
	slideshowOrderRevised = ` ORDER BY gallery_order DESC, revised DESC, id`
	slideshowOrderTitle   = ` ORDER BY title, id`

	slideshowCountForUser  = `SELECT COUNT(*) FROM slideshow WHERE user = ? AND  visible > -10`

	slideshowWhereId       = slideshowSelect + ` WHERE id = ?`
	slideshowWhereTopic    = slideshowSelect + ` WHERE topic = ? AND user = ? AND  visible > -10`

	slideshowsWhereTopic     = slideshowSelect + ` WHERE topic = ? AND  visible > -10`
	slideshowsWhereTopicUser = slideshowSelect + ` WHERE topic = ? AND user = ? AND  visible > -10`
	slideshowsWhereUser      = slideshowSelect + ` WHERE user = ? AND visible >= ?` + slideshowOrderRevised
	slideshowsNotTopics      = slideshowSelect + ` WHERE gallery = ? AND user IS NOT NULL AND visible > -10` + slideshowOrderTitle

	slideshowWhereShared = slideshowSelect + ` WHERE shared = ? AND visible > -10`

	// next slideshow ID in sequence for a topic, excluding suspended users
	slideshowWhereTopicAfter = `
		SELECT slideshow.id FROM slideshow
		JOIN user ON user.id = slideshow.user
		WHERE topic = ? AND revised > ? AND visible > -10 AND user.status > 0
		ORDER BY revised ASC LIMIT 1
	`

	// previous slideshow ID in sequence for a topic, excluding suspended users
	slideshowWhereTopicBefore = `
		SELECT slideshow.id FROM slideshow
		JOIN user ON user.id = slideshow.user
		WHERE topic = ? AND revised < ? AND visible > -10 AND user.status > 0
		ORDER BY revised DESC LIMIT 1
	`

	// a user's slideshow for a topic, if visible
	slideshowWhereTopicVisible = `
		SELECT slideshow.* FROM slideshow
		JOIN slideshow AS topic ON topic.id = slideshow.topic
		WHERE slideshow.topic = ? AND visible > -10 AND slideshow.user = ? AND topic.visible >= ?
	`

	// tagged slideshows
	slideshowsWhereTag = `
		SELECT slideshow.* FROM slideshow
		JOIN tagref ON tagref.item = slideshow.id
		JOIN tag ON tag.id = tagref.tag
		WHERE tag.parent = ? AND tag.name = ? AND slideshow.visible > -10
		ORDER BY tagref.added ASC
		LIMIT ?
	`

	slideshowsWhereTagOld = `
		SELECT slideshow.* FROM slideshow
		JOIN tagref ON tagref.item = slideshow.id
		JOIN tag ON tag.id = tagref.tag
		WHERE tag.gallery = ? AND tag.parent = ? AND tag.name = ?
		AND slideshow.revised < ? AND slideshow.visible > -10
	`

	slideshowsWhereTagSystem = `
		SELECT slideshow.*, tagref.id AS tagrefid
		FROM slideshow
		JOIN tagref ON tagref.item = slideshow.id
		WHERE tagref.tag = ? AND tagref.user IS NULL
		AND slideshow.visible > -10
		ORDER BY tagref.added ASC
		LIMIT ?
	`

	slideshowsWhereTagTopic = `
		SELECT slideshow.* FROM slideshow
		JOIN tagref ON tagref.item = slideshow.id
		JOIN tag ON tag.id = tagref.tag
		WHERE tag.parent = ? AND tag.name = ?
		AND slideshow.topic = ? AND slideshow.visible > -10
		ORDER BY tagref.added ASC
		LIMIT ?
	`

	slideshowsWhereTagUser = `
		SELECT slideshow.*, tagref.id AS tagrefid
		FROM slideshow
		JOIN tagref ON tagref.item = slideshow.id
		WHERE tagref.tag = ? AND tagref.user = ? AND slideshow.visible > -10
		ORDER BY tagref.added ASC
		LIMIT ?
	`

	// published slideshows for a user
	slideshowsUserPublished = `
		SELECT slideshow.* FROM slideshow
		LEFT JOIN slideshow AS topic ON topic.id = slideshow.topic
		WHERE slideshow.user = ?
		AND (slideshow.visible >= ? OR slideshow.visible = -1 AND topic.visible >= ?)
		AND slideshow.image <> ""
		ORDER BY slideshow.created DESC
	`

	topicsWhereEditable = slideshowSelect + ` WHERE gallery = ? AND user IS NULL AND id <> ? AND slideshow.visible > -10` + slideshowOrderTitle
	topicsWhereFormat   = slideshowSelect + ` WHERE gallery = ? AND user IS NULL AND format LIKE ? AND slideshow.visible > -10` + slideshowOrderTitle
	topicsWhereGallery  = slideshowSelect + ` WHERE gallery = ? AND user IS NULL AND slideshow.visible > -10` + slideshowOrderRevised

	// most recent visible topics and slideshows, with a per-user limit, excluding suspended users
	slideshowsRecentPublished = `
		WITH s1 AS (
			SELECT slideshow.*,
				RANK() OVER (PARTITION BY user
									ORDER BY created DESC, id
							) AS rnk
			FROM slideshow
			WHERE gallery = ? AND visible >= ? AND slideshow.image <> ""
		)
		SELECT s1.id, visible, user, title, caption, format, image
		FROM s1
		LEFT JOIN user ON user.id = s1.user
		WHERE s1.user IS NULL OR (rnk <= ? AND user.status > 0) 
		ORDER BY s1.created DESC
	`
	slideshowsTopicPublished = `
		SELECT slideshow.id, slideshow.title, slideshow.image, user.id AS userid, user.name 
		FROM slideshow
		INNER JOIN user ON user.id = slideshow.user
		WHERE slideshow.topic = ? AND slideshow.visible = -1 AND slideshow.image <> "" AND user.status > 0
		ORDER BY slideshow.revised`
)

type SlideshowStore struct {
	GalleryId    int64
	HighlightsId int64
	store
}

func NewSlideshowStore(db *sqlx.DB, tx **sqlx.Tx, log *log.Logger) *SlideshowStore {

	return &SlideshowStore{
		store: store{
			DBX:       db,
			ptx:       tx,
			errorLog:  log,
			sqlDelete: slideshowDelete,
			sqlInsert: slideshowInsert,
			sqlUpdate: slideshowUpdate,
		},
	}
}

// All returns all slideshows for all galleries, unordered and ignoring access.
// It is used only for migrations.
func (st *SlideshowStore) All() []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowSelect); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// AllForUsers returns all slideshows except topics.
func (st *SlideshowStore) AllForUsers() []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsNotTopics, st.GalleryId); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// All topics

func (st *SlideshowStore) AllTopics() []*models.Slideshow {

	var topics []*models.Slideshow

	if err := st.DBX.Select(&topics, topicsWhereGallery, st.GalleryId); err != nil {
		st.logError(err)
		return nil
	}
	return topics
}

// AllTopicsEditable returns topics with editable definitions.
func (st *SlideshowStore) AllTopicsEditable() []*models.Slideshow {

	var topics []*models.Slideshow

	if err := st.DBX.Select(&topics, topicsWhereEditable, st.GalleryId, st.HighlightsId); err != nil {
		st.logError(err)
		return nil
	}
	return topics
}

// AllTopicsFormatted returns topics matching a format specification.
func (st *SlideshowStore) AllTopicsFormatted(like string) []*models.Slideshow {

	var topics []*models.Slideshow

	if err := st.DBX.Select(&topics, topicsWhereFormat, st.GalleryId, like); err != nil {
		st.logError(err)
		return nil
	}
	return topics
}

// CountForUser returns the number of slideshows for a user.
func (st *SlideshowStore) CountForUser(userId int64) int {

	var n int

	if err := st.DBX.Get(&n, slideshowCountForUser, userId); err != nil {
		st.logError(err)
		return 0
	}

	return n
}

// ForTag returns all slideshows for a tag.
// ## not needed?
func (st *SlideshowStore) ForTag(tag string, nLimit int) []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsWhereTag, st.GalleryId, tag, nLimit); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// ForTagOld returns old slideshows for a tag.
func (st *SlideshowStore) ForTagOld(parent int64, tag string, before time.Time) []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsWhereTagOld, st.GalleryId, parent, tag, before); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// ForTagSystem returns slideshows tagged by the system.
func (st *SlideshowStore) ForTagSystem(tagId int64, nLimit int) []*models.SlideshowTagRef {

	var slideshows []*models.SlideshowTagRef

	if err := st.DBX.Select(&slideshows, slideshowsWhereTagSystem, tagId, nLimit); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// ForTagTopic returns tagged slideshows, for a topic.
// ## not needed?
func (st *SlideshowStore) ForTagTopic(tag string, topicId int64, nLimit int) []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsWhereTagTopic, st.GalleryId, tag, topicId, nLimit); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// ForTagUser returns tagged slideshows for a user.
func (st *SlideshowStore) ForTagUser(tagId int64, userId int64, nLimit int) []*models.SlideshowTagRef {

	var slideshows []*models.SlideshowTagRef

	if err := st.DBX.Select(&slideshows, slideshowsWhereTagUser, tagId, userId, nLimit); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// Slideshows for topic

func (st *SlideshowStore) ForTopic(topicId int64) []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsWhereTopic, topicId); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// Published slideshows for a topic, in specfied order

func (st *SlideshowStore) ForTopicPublished(topicId int64, latest bool) []*models.SlideshowUser {

	var order string
	if latest {
		order = " DESC"
	} else {
		order = " ASC"
	}

	var shows []*models.SlideshowUser

	if err := st.DBX.Select(&shows, slideshowsTopicPublished+order, topicId); err != nil {
		st.logError(err)
		return nil
	}

	return shows
}

// ForTopicSeq returns the next or previous slideshow ID in sequence for a topic, or 0 if there is none.
func (st *SlideshowStore) ForTopicSeq(topicId int64, current time.Time, after bool) int64 {

	var r int64

	var q string
	if after {
		q = slideshowWhereTopicAfter
	} else {
		q = slideshowWhereTopicBefore
	}

	if err := st.DBX.Get(&r, q, topicId, current); err != nil {
		err = st.convertError(err)
		if err != models.ErrNoRecord {
			st.logError(err)
		}
		return 0
	}

	return r
}

// ForTopicUserAll returns all slideshows for a topic and user.
func (st *SlideshowStore) ForTopicUserAll(topicId int64, userId int64) []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsWhereTopicUser, topicId, userId); err != nil {
		st.logError(err)
		return nil
	}

	return slideshows
}

// ForTopicUserIf returns the slideshow for a topic and user, if it exists.
func (st *SlideshowStore) ForTopicUserIf(topicId int64, userId int64) *models.Slideshow {

	var r models.Slideshow

	if err := st.DBX.Get(&r, slideshowWhereTopic, topicId, userId); err != nil {
		return nil
	}

	return &r
}

// ForTopicUserVisibleIf returns the slideshow for a topic and user, if it exists and is visible.
func (st *SlideshowStore) ForTopicUserVisibleIf(topicId int64, userId int64, visible int) *models.Slideshow {

	var r models.Slideshow

	if err := st.DBX.Get(&r, slideshowWhereTopicVisible, topicId, userId, visible); err != nil {
		return nil
	}

	return &r
}

// All slideshows for user, in latest published order, specified visibility

func (st *SlideshowStore) ForUser(userId int64, visible int) []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsWhereUser, userId, visible); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// All published slideshows for user, in published order, including topics

func (st *SlideshowStore) ForUserPublished(userId int64, visible int) []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsUserPublished, userId, visible, visible); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// Slideshow by number

func (st *SlideshowStore) Get(id int64) (*models.Slideshow, error) {

	var r models.Slideshow

	if err := st.DBX.Get(&r, slideshowWhereId, id); err != nil {
		return nil, st.logError(err)
	}

	return &r, nil
}

// Sideshow, if it exists

func (st *SlideshowStore) GetIf(id int64) *models.Slideshow {

	var r models.Slideshow

	if err := st.DBX.Get(&r, slideshowWhereId, id); err != nil {
		if st.convertError(err) != models.ErrNoRecord {
			st.logError(err)
		}
		return nil
	}

	return &r
}

// GetIfShared returns a shared slideshow.
func (st *SlideshowStore) GetIfShared(shared int64) *models.Slideshow {

	var r models.Slideshow

	if err := st.DBX.Get(&r, slideshowWhereShared, shared); err != nil {
		if st.convertError(err) != models.ErrNoRecord {
			st.logError(err)
		}
		return nil
	}

	return &r
}

// Most recent shows, up to N per user, excluding RecentPublic and including topics, in descending publication date

func (st *SlideshowStore) RecentPublished(visible int, max int) []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsRecentPublished, st.GalleryId, visible, max); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// Insert or update slideshow

func (st *SlideshowStore) Update(r *models.Slideshow) error {
	r.Gallery = st.GalleryId

	return st.updateData(&r.Id, r)
}

// Set slideshow with specified ID (temporary function used to migrate topics)

func (st *SlideshowStore) Set(r *models.Slideshow) error {
	r.Gallery = st.GalleryId

	tx := *st.ptx
	if tx == nil {
		panic("Transaction not begun")
	}

	if _, err := tx.NamedExec(slideshowSet, r); err != nil {
		st.logError(err)
		return st.convertError(err)
	}

	return nil
}
