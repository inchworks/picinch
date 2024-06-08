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

	"inchworks.com/picinch/internal/models"
)

const (
	slideDelete = `DELETE FROM slide WHERE id = ?`

	slideInsert = `
		INSERT INTO slide (slideshow, format, show_order, created, revised, title, caption, image)
		VALUES (:slideshow, :format, :show_order, :created, :revised, :title, :caption, :image)`

	slideUpdate = `
		UPDATE slide
		SET format=:format, show_order=:show_order, created=:created, revised=:revised, title=:title, caption=:caption, image=:image
		WHERE id = :id
	`
)

const (
	slideSelect = `SELECT * FROM slide`

	slideOrder  = ` ORDER BY show_order ASC, revised DESC LIMIT ?`
	slideRecent = ` ORDER BY revised DESC LIMIT ?`

	slideWhereId          = slideSelect + ` WHERE id = ?`
	slidesWhereShow = slideSelect + ` WHERE slideshow = ?`
	slidesWhereShowOlder       = slideSelect + ` WHERE slideshow = ?` + slideOrder
	slidesWhereShowRecent = slideSelect + ` WHERE slideshow = ?` + slideRecent

	// all images for a topic, excluding suspended users and temporary versions
	imagesWhereTopic = `
		SELECT slide.image FROM slide
		INNER JOIN slideshow ON slideshow.id = slide.slideshow
		INNER JOIN user ON user.id = slideshow.user
		WHERE slideshow.topic = ? AND slideshow.visible > -10 AND slide.image LIKE 'P%' AND user.status > 0
		`

	// most recent slides for a topic, excluding suspended users
	slidesRecentTopic = `
		WITH s1 AS (
			SELECT slide.*, user.name,
				RANK() OVER (PARTITION BY slideshow.user
									ORDER BY slide.created DESC, slide.id DESC
							) AS rnk
			FROM slide
			INNER JOIN slideshow ON slideshow.id = slide.slideshow
			INNER JOIN user ON user.id = slideshow.user
			WHERE slideshow.topic = ? AND slideshow.visible > -10 AND slide.image <> '' AND user.status > 0
			)
		SELECT format, title, caption, image, name
		FROM s1
		WHERE rnk <= ?
		ORDER BY revised DESC, id DESC LIMIT ?
	`
)

type SlideStore struct {
	store
}

func NewSlideStore(db *sqlx.DB, tx **sqlx.Tx, log *log.Logger) *SlideStore {

	return &SlideStore{
		store: store{
			DBX:       db,
			ptx:       tx,
			errorLog:  log,
			sqlDelete: slideDelete,
			sqlInsert: slideInsert,
			sqlUpdate: slideUpdate,
		},
	}
}

// ForSlideshow returns all slides for slideshow, unordered.
func (st *SlideStore) ForSlideshow(showId int64) []*models.Slide {

	var slides []*models.Slide

	if err := st.DBX.Select(&slides, slidesWhereShow, showId); err != nil {
		st.logError(err)
		return nil
	}
	return slides
}

// ImagesForTopic returns all the images for a topic, excluding those for suspended users.
func (st *SlideStore) ImagesForTopic(topicId int64) []string {

	var tns []string

	if err := st.DBX.Select(&tns, imagesWhereTopic, topicId); err != nil {
		st.logError(err)
		return nil
	}

	return tns
}

// Recent slides for slideshow

func (st *SlideStore) ForSlideshowOrdered(showId int64, recent bool, max int) []*models.Slide {

	var slides []*models.Slide
	var q string

	if recent {
		q = slidesWhereShowRecent
	} else {
		q = slidesWhereShowOlder
	}

	if err := st.DBX.Select(&slides, q, showId, max); err != nil {
		st.logError(err)
		return nil
	}

	return slides
}

// RecentForTopic returns the most recent slides, in order with a per-user limit, and excluding suspended users.

func (st *SlideStore) RecentForTopic(topicId int64, perUser int, max int) []*models.TopicSlide {

	var slides []*models.TopicSlide

	if err := st.DBX.Select(&slides, slidesRecentTopic, topicId, perUser, max); err != nil {
		st.logError(err)
		return nil
	}

	return slides
}

// Insert or update slide. Round ID must be set in struct.

func (st *SlideStore) Update(q *models.Slide) error {

	return st.updateData(&q.Id, q)
}
