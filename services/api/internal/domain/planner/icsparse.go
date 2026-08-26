package planner

import (
	"fmt"
	"strings"
	"time"
)

// ---- ICS (iCalendar) VEVENT parsing (phase 13c) ----
//
// Deliberately minimal: unfolds continuation lines, extracts VEVENT fields,
// and degrades gracefully — RRULEs are ignored (single occurrence kept, noted
// in the description), TZID times are read as UTC (documented limitation).

// ICSEvent is one imported calendar event, ready for the store.
type ICSEvent struct {
	UID         string
	Title       string
	Description string
	Location    string
	StartsAt    string // RFC3339 UTC
	EndsAt      *string
}

type icsProps map[string]string // e.g. "DTSTART" -> "...", "DTSTART;TZID=X" -> "..."

// ParseICS extracts VEVENTs from raw iCalendar text.
func ParseICS(text string) []ICSEvent {
	lines := unfoldICSLines(text)
	var out []ICSEvent
	var cur icsProps
	inEvent := false
	for _, line := range lines {
		upper := strings.ToUpper(line)
		switch {
		case upper == "BEGIN:VEVENT":
			cur = icsProps{}
			inEvent = true
		case upper == "END:VEVENT":
			if inEvent {
				if ev := buildICSEvent(cur); ev != nil {
					out = append(out, *ev)
				}
			}
			inEvent = false
		case inEvent:
			name, value := splitICSLine(line)
			if name != "" {
				cur[name] = value
			}
		}
	}
	return out
}

// unfoldICSLines joins RFC 5545 folded lines (continuations start with space/tab).
func unfoldICSLines(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var out []string
	for _, l := range raw {
		if len(l) > 0 && (l[0] == ' ' || l[0] == '\t') {
			if len(out) > 0 {
				out[len(out)-1] += l[1:]
			}
			continue
		}
		out = append(out, l)
	}
	return out
}

// splitICSLine splits "NAME;PARAM=V:value", keeping params in the name so
// DTSTART;VALUE=DATE and DTSTART;TZID=... stay distinguishable.
func splitICSLine(line string) (string, string) {
	i := strings.Index(line, ":")
	if i < 0 {
		return "", ""
	}
	name := strings.TrimSpace(line[:i])
	value := strings.TrimRight(line[i+1:], " ")
	return name, unescapeICS(value)
}

func unescapeICS(s string) string {
	r := strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`)
	return r.Replace(s)
}

func buildICSEvent(p icsProps) *ICSEvent {
	title := p["SUMMARY"]
	if title == "" {
		return nil // skip untitled events
	}
	startKey, startVal := findProp(p, "DTSTART")
	start, allDay, ok := parseICSDateTime(p["DTSTART;VALUE=DATE"], startVal, startKey == "DTSTART;VALUE=DATE")
	if !ok {
		return nil
	}
	desc := p["DESCRIPTION"]
	if p["RRULE"] != "" {
		note := "[imported from ICS — recurrence ignored, showing first occurrence only]"
		if desc == "" {
			desc = note
		} else {
			desc = desc + "\n" + note
		}
	}
	ev := &ICSEvent{
		UID:         p["UID"],
		Title:       title,
		Description: desc,
		Location:    p["LOCATION"],
		StartsAt:    start,
	}
	if _, endRaw := findProp(p, "DTEND"); endRaw != "" {
		if end, _, ok := parseICSDateTime(p["DTEND;VALUE=DATE"], endRaw, false); ok && end > start {
			t := end
			if allDay {
				// All-day DTEND is exclusive; keep it exclusive-ish by not
				// extending past start+24h for display purposes.
				if st, err := time.Parse(time.RFC3339, start); err == nil {
					if et, err := time.Parse(time.RFC3339, end); err == nil && et.Sub(st) > 24*time.Hour {
						t = st.Add(24 * time.Hour).Format(time.RFC3339)
					}
				}
			}
			ev.EndsAt = &t
		}
	}
	return ev
}

func allDayFlag(p icsProps) bool {
	return p["DTSTART;VALUE=DATE"] != ""
}

// findProp returns the value of the first property whose name equals key or
// starts with key+";" (parameters like TZID/VALUE present).
func findProp(p icsProps, key string) (string, string) {
	if v, ok := p[key]; ok {
		return key, v
	}
	prefix := key + ";"
	best, bestVal := "", ""
	for name, val := range p {
		if strings.HasPrefix(name, prefix) {
			if best == "" || name < best { // deterministic pick
				best, bestVal = name, val
			}
		}
	}
	return best, bestVal
}

// parseICSDateTime handles VALUE=DATE (YYYYMMDD), UTC (…Z), and TZID/naive
// local forms (read as UTC — documented limitation).
func parseICSDateTime(dateValue, dateTimeValue string, allDay bool) (string, bool, bool) {
	if allDay && dateValue != "" {
		t, err := time.Parse("20060102", dateValue)
		if err != nil {
			return "", false, false
		}
		return t.UTC().Format(time.RFC3339), true, true
	}
	if dateTimeValue == "" {
		return "", false, false
	}
	layouts := []string{"20060102T150405Z", "20060102T150405"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateTimeValue); err == nil {
			return t.UTC().Format(time.RFC3339), false, true
		}
	}
	return "", false, false
}

// ValidateICSURL is a tiny guard for server-side fetches.
func ValidateICSURL(raw string) error {
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return fmt.Errorf("url must be http(s)")
	}
	return nil
}
