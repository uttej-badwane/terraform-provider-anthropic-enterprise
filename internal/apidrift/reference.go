// Package apidrift compares the fields this provider decodes against what
// Anthropic's API reference marks deprecated, so a deprecated field is found
// while it is still being sent rather than on the day it stops.
//
// The reference is published as markdown: every page is listed in llms.txt and
// served with a .md suffix, fields are nested bullets whose indentation gives
// their position in the object, and a deprecated field carries a
// "**Deprecated**" line, usually naming its replacement.
package apidrift

import (
	"regexp"
	"strings"
)

// Deprecation is one response field the reference marks deprecated.
type Deprecation struct {
	Page string
	// Path locates the field from the root of the returned object, for
	// example ["scope", "workspace_id"]. The "data" wrapper of list responses
	// is removed, so list and retrieve pages produce the same paths.
	Path []string
	// Replacement is the field to use instead, relative to the deprecated
	// field's parent, for example ["group", "type"]. Nil when the reference
	// names none.
	Replacement []string
	// Note is the reference's own explanation, trimmed for a report.
	Note string
}

var (
	// "- `name: type`" is a field; the name is what matters.
	fieldLine = regexp.MustCompile("^(\\s*)- `([a-z0-9_]+):")
	// "- `BetaSomething object`" introduces one variant of a union. It is a
	// container, not a field, so it contributes no path segment.
	variantLine = regexp.MustCompile("^(\\s*)- `[^`:]+ object`")
	// "Use `group.type` instead", in either case.
	useInstead = regexp.MustCompile("(?i)use `([a-z0-9_.]+)` instead")
)

type frame struct {
	indent int
	name   string
}

// ParseDeprecations returns the deprecated response fields on one reference
// page. Only the "## Returns" section is read: request parameters can share a
// name with a response field without sharing its deprecation, which is exactly
// the confusion a name-only scan gets wrong.
func ParseDeprecations(page, markdown string) []Deprecation {
	lines := strings.Split(markdown, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "## Returns") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}

	var (
		out   []Deprecation
		stack []frame
		// last is the most recent field and whether a deprecation has already
		// been recorded for it, since the marker is often repeated.
		last       []string
		lastMarked bool
	)
	for i := start; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(l, "## ") {
			break
		}
		if m := fieldLine.FindStringSubmatch(l); m != nil {
			indent := len(m[1])
			for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
				stack = stack[:len(stack)-1]
			}
			stack = append(stack, frame{indent: indent, name: m[2]})
			last = pathOf(stack)
			lastMarked = false
			continue
		}
		if variantLine.MatchString(l) {
			continue
		}
		if !strings.Contains(l, "**Deprecated**") || last == nil || lastMarked {
			continue
		}
		note := explanation(lines, i)
		var repl []string
		if m := useInstead.FindStringSubmatch(note); m != nil {
			repl = strings.Split(m[1], ".")
		}
		out = append(out, Deprecation{Page: page, Path: last, Replacement: repl, Note: note})
		lastMarked = true
	}
	return out
}

// pathOf returns the field path for the current stack, without the list
// wrapper.
func pathOf(stack []frame) []string {
	p := make([]string, 0, len(stack))
	for _, f := range stack {
		p = append(p, f.name)
	}
	if len(p) > 1 && p[0] == "data" {
		p = p[1:]
	}
	return p
}

// explanation joins the deprecation line with the description that follows
// it, stopping at the next bullet.
func explanation(lines []string, i int) string {
	var parts []string
	for j := i; j < len(lines) && j < i+4; j++ {
		t := strings.TrimSpace(lines[j])
		if j > i && (strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "#")) {
			break
		}
		if t != "" {
			parts = append(parts, t)
		}
	}
	s := strings.Join(parts, " ")
	s = strings.ReplaceAll(s, "**Deprecated**:", "")
	s = strings.ReplaceAll(s, "**Deprecated**", "")
	s = strings.TrimSpace(s)
	if len(s) > 280 {
		s = s[:277] + "..."
	}
	return s
}
