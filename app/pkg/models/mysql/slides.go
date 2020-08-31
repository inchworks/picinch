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

	"inchworks.com/gallery/pkg/models"
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

	slideOrder = ` ORDER BY show_order ASC, revised DESC LIMIT ?`
	slideRecent = ` ORDER BY revised DESC LIMIT ?`

	slideWhereId    = slideSelect + ` WHERE id = ?`
	slidesWhereShow = slideSelect + ` WHERE slideshow = ?` + slideOrder
	slidesWhereShowRecent = slideSelect + ` WHERE slideshow = ?` + slideRecent

	imagesWhereTopic = `
		SELECT slide.image FROM slide
		INNER JOIN slideshow ON slideshow.id = slide.slideshow
		WHERE slideshow.topic = ? AND slide.image <> ''
	`

	slidesWhereTopic = `
		SELECT slide.format, slide.title, slide.caption, slide.image, user.name as name FROM slide
		INNER JOIN slideshow ON slideshow.id = slide.slideshow
		INNER JOIN user ON user.id = slideshow.user
		WHERE slideshow.topic = ? AND slide.image <> ''
		ORDER BY slide.created DESC, slide.id DESC LIMIT ?
	`

	// most recent slides for a topic
	slidesRecentTopic = `
		WITH s1 AS (
			SELECT slide.*, user.name,
				RANK() OVER (PARTITION BY slideshow.user
									ORDER BY slide.created DESC, slide.id DESC
							) AS rnk
			FROM slide
			INNER JOIN slideshow ON slideshow.id = slide.slideshow
			INNER JOIN user ON user.id = slideshow.user
			WHERE slideshow.topic = ? AND slide.image <> ''
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

// All slides for slideshow, in order

func (st *SlideStore) ForSlideshow(showId int64, max int) []*models.Slide {

	var slides []*models.Slide

	if err := st.DBX.Select(&slides, slidesWhereShow, showId, max); err != nil {
		st.logError(err)
		return nil
	}
	return slides
}

// Slides for topic, in recent order

func (st *SlideStore) ForTopic(topicId int64, max int) []*models.TopicSlide {

	var slides []*models.TopicSlide

	if err := st.DBX.Select(&slides, slidesWhereTopic, topicId, max); err != nil {
		st.logError(err)
		return nil
	}

	return slides
}

// Images for topic

func (st *SlideStore) ImagesForTopic(topicId int64) []string {

	var tns []string

	if err := st.DBX.Select(&tns, imagesWhereTopic, topicId); err != nil {
		st.logError(err)
		return nil
	}

	return tns
}

// Recent slides for slideshow

func (st *SlideStore) RecentForSlideshow(showId int64, max int) []*models.Slide {

	var slides []*models.Slide

	if err := st.DBX.Select(&slides, slidesWhereShowRecent, showId, max); err != nil {
		st.logError(err)
		return nil
	}

	return slides
}

// Recent slides for topic, in order, limited per user

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
