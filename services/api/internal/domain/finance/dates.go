package finance

import (
	"fmt"
	"strings"
	"time"
)

// dateLayouts ordered by preference: ISO first, then DMY (common in ID bank
// exports), then MDY. SniffFormat picks the first layout that parses every
// non-empty value, so a whole file is interpreted consistently.
var dateLayouts = []string{
	"2006-01-02",
	"02/01/2006",
	"01/02/2006",
	"02-01-2006",
	"01-02-2006",
	"2006/01/02",
	"02 Jan 2006",
	"2 Jan 2006",
	"Jan 2, 2006",
	"January 2, 2006",
	"2 January 2006",
}

// SniffDateLayout picks the layout that parses the most values (empty or
// broken cells are tolerated), preferring earlier layouts on ties so a whole
// file is interpreted consistently.
func SniffDateLayout(values []string) string {
	var nonEmpty []string
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			nonEmpty = append(nonEmpty, v)
		}
	}
	if len(nonEmpty) == 0 {
		return dateLayouts[0]
	}
	bestIdx, bestScore := -1, 0
	for i, layout := range dateLayouts {
		score := 0
		for _, v := range nonEmpty {
			if _, err := time.Parse(layout, strings.TrimSpace(v)); err == nil {
				score++
			}
		}
		if score > bestScore {
			bestIdx, bestScore = i, score
		}
		if bestScore == len(nonEmpty) {
			break
		}
	}
	if bestIdx < 0 {
		return ""
	}
	return dateLayouts[bestIdx]
}

// ParseDate normalizes a single date cell to YYYY-MM-DD using an explicit
// layout when provided, otherwise trying layouts in preference order.
func ParseDate(value, layout string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", fmt.Errorf("empty date")
	}
	if layout != "" {
		t, err := time.Parse(layout, v)
		if err != nil {
			return "", fmt.Errorf("date %q does not match format %s", value, layout)
		}
		return t.Format("2006-01-02"), nil
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, v); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("unrecognized date %q", value)
}
