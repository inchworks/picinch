// Copyright © Rob Burke inchworks.com, 2024.

// This file is part of PicInch.
//
// PicInch is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// PicInch is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY orBoo FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with PicInch.  If not, see <https://www.gnu.org/licenses/>.

package cache

// Configurable home page content.

import (
	"inchworks.com/picinch/internal/models"
)

type HomeCache struct {
	Main     *models.Slideshow
	Sections map[int64]*models.Slide // ## itemised in some useful way
}

func (*HomeCache) Build(page *models.Slideshow) {
}
