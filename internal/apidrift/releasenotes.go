package apidrift

import (
	"regexp"
	"strings"
	"time"
)

// Announcement is one release note worth a maintainer's attention.
type Announcement struct {
	Date time.Time
	Text string
}

var (
	dateHeading = regexp.MustCompile(`^#{2,4}\s+([A-Z][a-z]+ \d{1,2}, \d{4})\s*$`)
	// Field markers only catch deprecations Anthropic attaches to a field.
	// Retired scopes, removed endpoints and behaviour changes are announced in
	// prose, so the release notes are read for these words as well.
	attention = regexp.MustCompile(`(?i)\b(deprecat\w*|retir\w*|remov\w*|sunset\w*|no longer|end[- ]of[- ]life|breaking|now always)\b`)
	mdLink    = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	// A note must also touch something this provider manages. Most release
	// notes are about the Messages API, and a change there matches the words
	// above without mattering here.
	relevant = regexp.MustCompile(`(?i)\b(admin api|organi[sz]ation|workspace|api keys?|compliance|analytics|spend limit|rbac|groups?|members?|invites?|service accounts?|federation|workload identity|rate limits?|managed agents|agents?|environments?|vaults?|deployments?|memory stores?|skills?|usage report|cost report|external keys?|tunnels?|scopes?|retir\w*)\b`)
)

// ParseReleaseNotes returns the release notes published on or after since that
// mention a deprecation, retirement, removal or change of behaviour.
func ParseReleaseNotes(md string, since time.Time) []Announcement {
	var (
		out     []Announcement
		current time.Time
		inRange bool
		item    []string
	)
	flush := func() {
		if len(item) == 0 {
			return
		}
		text := strings.Join(item, " ")
		item = nil
		if inRange && attention.MatchString(text) && relevant.MatchString(text) {
			text = mdLink.ReplaceAllString(text, "$1")
			if len(text) > 400 {
				text = text[:397] + "..."
			}
			out = append(out, Announcement{Date: current, Text: text})
		}
	}
	for _, l := range strings.Split(md, "\n") {
		if m := dateHeading.FindStringSubmatch(l); m != nil {
			flush()
			d, err := time.Parse("January 2, 2006", m[1])
			inRange = err == nil && !d.Before(since)
			current = d
			continue
		}
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "* ") || strings.HasPrefix(t, "- "):
			flush()
			item = []string{strings.TrimSpace(t[2:])}
		case t == "" || strings.HasPrefix(t, "#"):
			flush()
		case len(item) > 0:
			item = append(item, t)
		}
	}
	flush()
	return out
}
