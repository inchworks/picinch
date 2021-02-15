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
	topicDelete = `DELETE FROM topic WHERE id = ?`

	topicInsert = `
		INSERT INTO topic (gallery, gallery_order, visible, shared, created, revised, title, caption, format, image)
		VALUES (:gallery, :gallery_order, :visible, :shared, :created, :revised, :title, :caption, :format, :image)`

	topicUpdate = `
		UPDATE topic
		SET gallery_order=:gallery_order, visible=:visible, shared=:shared, created=:created, revised=:revised, title=:title, caption=:caption, format=:format, image=:image
		WHERE id = :id
	`
)

const (
	// note that ID is included for stable ordering of selections for editing
	topicSelect       = `SELECT * FROM topic`
	topicOrderDisplay = ` ORDER BY gallery_order ASC, revised DESC`
	topicOrderTitle   = ` ORDER BY title, id`

	topicWhereId         = topicSelect + ` WHERE id = ?`
	topicWhereShared = topicSelect + ` WHERE shared = ?`
	topicsWhereEditable  = topicSelect + ` WHERE gallery = ? AND id <> ?` + topicOrderTitle
	topicsWhereGallery   = topicSelect + ` WHERE gallery = ?` + topicOrderDisplay
	topicsWherePublished = topicSelect + ` WHERE gallery = ? AND visible = ?` + topicOrderDisplay
)

type TopicStore struct {
	GalleryId    int64
	HighlightsId int64
	store
}

func NewTopicStore(db *sqlx.DB, tx **sqlx.Tx, log *log.Logger) *TopicStore {

	return &TopicStore{
		store: store{
			DBX:       db,
			ptx:       tx,
			errorLog:  log,
			sqlDelete: topicDelete,
			sqlInsert: topicInsert,
			sqlUpdate: topicUpdate,
		},
	}
}

// All topics

func (st *TopicStore) All() []*models.Topic {

	var topics []*models.Topic

	if err := st.DBX.Select(&topics, topicsWhereGallery, st.GalleryId); err != nil {
		st.logError(err)
		return nil
	}
	return topics
}

// All editable topics

func (st *TopicStore) AllEditable() []*models.Topic {

	var topics []*models.Topic

	if err := st.DBX.Select(&topics, topicsWhereEditable, st.GalleryId, st.HighlightsId); err != nil {
		st.logError(err)
		return nil
	}
	return topics
}

// topic by ID

func (st *TopicStore) Get(id int64) (*models.Topic, error) {

	var r models.Topic

	if err := st.DBX.Get(&r, topicWhereId, id); err != nil {
		return nil, st.logError(err)
	}

	return &r, nil
}

// Topic, if it still exists

func (st *TopicStore) GetIf(id int64) *models.Topic {

	var r models.Topic

	if err := st.DBX.Get(&r, topicWhereId, id); err != nil {
		if st.convertError(err) != models.ErrNoRecord {
			st.logError(err)
		}
		return nil
	}

	return &r
}

// Shared topic

func (st *TopicStore) GetIfShared(shared int64) *models.Topic {

	var r models.Topic

	if err := st.DBX.Get(&r, topicWhereShared, shared); err != nil {
		if st.convertError(err) != models.ErrNoRecord {
			st.logError(err)
		}
		return nil
	}

	return &r
}

// Published topics

func (st *TopicStore) Published(visible int) []*models.Topic {

	var topics []*models.Topic

	if err := st.DBX.Select(&topics, topicsWherePublished, st.GalleryId, visible); err != nil {
		st.logError(err)
		return nil
	}
	return topics
}

// Insert or update topic

func (st *TopicStore) Update(r *models.Topic) error {
	r.Gallery = st.GalleryId

	return st.updateData(&r.Id, r)
}
