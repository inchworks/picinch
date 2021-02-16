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
	topicOrderTitle   = ` ORDER BY title, id`

	topicWhereId         = topicSelect + ` WHERE id = ?`
	topicsWhereGallery0  = topicSelect + ` WHERE gallery = ?` + topicOrderTitle
)

type TopicStore struct {
	GalleryId     int64
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

	if err := st.DBX.Select(&topics, topicsWhereGallery0, st.GalleryId); err != nil {
		st.logError(err)
		return nil
	}
	return topics
}

// Topic, if it or the table still exists

func (st *TopicStore) GetIf(id int64) *models.Topic {

	var r models.Topic

	if err := st.DBX.Get(&r, topicWhereId, id); err != nil {
		return nil
	}

	return &r
}
