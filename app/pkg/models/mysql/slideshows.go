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

	"github.com/jmoiron/sqlx"

	"inchworks.com/picinch/pkg/models"
)

const (
	slideshowDelete = `DELETE FROM slideshow WHERE id = ?`

	slideshowInsert = `
		INSERT INTO slideshow (gallery, gallery_order, visible, user, shared, topic, created, revised, title, caption, format, image)
		VALUES (:gallery, :gallery_order, :visible, :user, :shared, :topic, :created, :revised, :title, :caption, :format, :image)`

	slideshowUpdate = `
		UPDATE slideshow
		SET gallery_order=:gallery_order, visible=:visible, topic=:topic, created=:created, revised=:revised, title=:title, caption=:caption, format=:format, image=:image
		WHERE id = :id
	`
)

const (
	slideshowSelect     = `SELECT * FROM slideshow`
	slideshowOrder      = ` ORDER BY gallery_order ASC, created DESC`
	slideshowOrderRevised = ` ORDER BY revised DESC`
	slideshowOrderTitle = ` ORDER BY title ASC, id`
	slideshowRevisedSeq = ` ORDER BY revised ASC LIMIT ?,1`

	slideshowCountForTopic = `SELECT COUNT(*) FROM slideshow WHERE topic = ?`

	slideshowWhereId       = slideshowSelect + ` WHERE id = ?`
	slideshowWhereTopic    = slideshowSelect + ` WHERE topic = ? AND user = ?`
	slideshowWhereTopicSeq = slideshowSelect + ` WHERE topic = ?` + slideshowRevisedSeq

	slideshowsWhereTopic      = slideshowSelect + ` WHERE topic = ?`
	slideshowsWhereUser       = slideshowSelect + ` WHERE user = ?  AND visible >= ?` + slideshowOrder
	slideshowsWhereGallery    = slideshowSelect + ` WHERE gallery = ?` + slideshowOrderTitle
	slideshowsUserPublished   = slideshowSelect + ` WHERE user = ? AND visible <> 0 AND slideshow.image <> ""` + slideshowOrderRevised

	// most recent public slideshow for each user
	slideshowsRecentPublished = `
		WITH s1 AS (
			SELECT slideshow.*,
				RANK() OVER (PARTITION BY user
									ORDER BY created DESC, id
							) AS rnk
			FROM slideshow
			WHERE gallery = ? AND visible >= ? AND slideshow.image <> ""
		)
		SELECT id, visible, user, title, caption, format, image
		FROM s1
		WHERE rnk <= ?
		ORDER BY created DESC
	`
	slideshowsTopicPublished = `
		SELECT slideshow.id, slideshow.title, slideshow.image, user.name 
		FROM slideshow
		INNER JOIN user ON user.id = slideshow.user
		WHERE slideshow.topic = ? AND slideshow.visible <> 0 AND slideshow.image <> ""
		ORDER BY slideshow.revised`
)

type SlideshowStore struct {
	GalleryId int64
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

// All slideshows

func (st *SlideshowStore) All() []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsWhereGallery, st.GalleryId); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// Count of slideshows for topic

func (st *SlideshowStore) CountForTopic(topicId int64) int {

	var n int

	if err := st.DBX.Get(&n, slideshowCountForTopic, topicId); err != nil {
		st.logError(err)
		return 0
	}

	return n
}


// Slideshow for topic

func (st *SlideshowStore) ForTopic(topicId int64) []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsWhereTopic); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// Published slideshows for a topic, in specfied order

func (st *SlideshowStore) ForTopicPublished(topicId int64, latest bool) []*models.SlideshowUser {

	var order string
	if latest { order = " DESC" } else { order = " ASC" }

	var shows []*models.SlideshowUser

	if err := st.DBX.Select(&shows, slideshowsTopicPublished + order, topicId); err != nil {
		st.logError(err)
		return nil
	}

	return shows
}

// Slideshow in sequence for topic

func (st *SlideshowStore) ForTopicSeq(topicId int64, seq int) *models.Slideshow {

	var r models.Slideshow

	if err := st.DBX.Get(&r, slideshowWhereTopicSeq, topicId, seq); err != nil {
		err = st.convertError(err)
		if err != models.ErrNoRecord {
			st.logError(err)
		}
		return nil
	}

	return &r
}

// Slideshow for topic and user

func (st *SlideshowStore) ForTopicUser(topicId int64, userId int64) *models.Slideshow {

	var r models.Slideshow

	if err := st.DBX.Get(&r, slideshowWhereTopic, topicId, userId); err != nil {
		return nil
	}

	return &r
}

// All slideshows for user, in published order, specified visibility

func (st *SlideshowStore) ForUser(userId int64, visible int) []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsWhereUser, userId, visible); err != nil {
		st.logError(err)
		return nil
	}
	return slideshows
}

// All published slideshows for user, in published order, including topics

func (st *SlideshowStore) ForUserPublished(userId int64) []*models.Slideshow {

	var slideshows []*models.Slideshow

	if err := st.DBX.Select(&slideshows, slideshowsUserPublished, userId); err != nil {
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

func (st *SlideshowStore) GetIf(id int64) (*models.Slideshow) {

	var r models.Slideshow

	if err := st.DBX.Get(&r, slideshowWhereId, id); err != nil {
		if st.convertError(err) != models.ErrNoRecord { st.logError(err) }
		return nil
	}

	return &r
}

// Most recent shows, up to N per user, excluding RecentPublic, in descending publication date

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
