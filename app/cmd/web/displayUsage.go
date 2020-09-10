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

package main

// Display usage statistics

import (
	"inchworks.com/gallery/pkg/usage"
)

// Daily and monthly usage statistics

func (s *GalleryState) ForUsage(period int) *DataUsagePeriods {

	var title string
	var fmt string
	switch period {
	case usage.Day:
		title = "Daily Usage"
		fmt = "Mon 2 Jan"

	case usage.Month:
		title = "Monthly Usage"
		fmt = "January 2006"

	default:
		return nil
	}

	// serialisation
	defer s.updatesNone()()

	// get stats
	stats := usage.Get(s.app.StatisticStore, period)
	var dataUsage []*DataUsage

	for _, s := range stats {

		dataUsage = append(dataUsage, &DataUsage{
			Date:  s[0].Start.Format(fmt),
			Stats: s,
		})
	}

	return &DataUsagePeriods{
		Title: title,
		Usage: dataUsage,
	}
}
