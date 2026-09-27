package apidrift

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed manifest.json
var manifestJSON []byte

// Manifest maps reference pages to the client type each one's response
// populates. Nested objects are followed through the Go struct fields, so only
// the root type is listed.
type Manifest struct {
	Pages  []PageEntry   `json:"pages"`
	Ignore []IgnoreEntry `json:"ignore"`
}

// PageEntry names a reference page and the client type its response decodes
// into.
type PageEntry struct {
	Page string `json:"page"`
	Type string `json:"type"`
}

// IgnoreEntry suppresses one finding that has been looked at and judged
// harmless. The reason is required, so the judgement is recorded.
type IgnoreEntry struct {
	Type   string `json:"type"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// LoadManifest returns the embedded manifest.
func LoadManifest() (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		return m, fmt.Errorf("manifest.json: %w", err)
	}
	return m, nil
}

// Kind classifies a finding.
type Kind string

const (
	// KindMigrate: a deprecated field is decoded and its replacement is not.
	// The day the deprecated field stops being sent, this reads back empty.
	KindMigrate Kind = "migrate"
	// KindNoReplacement: a deprecated field with no named replacement is
	// decoded. Usually it should simply stop being read.
	KindNoReplacement Kind = "no-replacement"
	// KindFallback: both the deprecated field and its replacement are
	// decoded. Safe; the fallback can be dropped once the API stops sending
	// the old field.
	KindFallback Kind = "fallback"
)

// Finding is one deprecation that touches a field the client decodes.
type Finding struct {
	Kind        Kind
	Page        string
	Type        string // the struct holding the deprecated field
	Field       string
	Path        string // the field's path from the page's root object
	Replacement string
	Note        string
}

// Result is the outcome of a check.
type Result struct {
	Findings      []Finding // KindMigrate and KindNoReplacement
	Fallbacks     []Finding
	UnusedIgnores []IgnoreEntry
	// Unchecked lists client types no manifest page reaches, so a
	// deprecation in them would go unnoticed.
	Unchecked []string
}

// Validate reports manifest entries that cannot work: an unknown root type
// would silently check nothing.
func Validate(m Manifest, types Types) []error {
	var errs []error
	seen := map[string]bool{}
	for _, p := range m.Pages {
		if seen[p.Page] {
			errs = append(errs, fmt.Errorf("page %q is listed twice", p.Page))
		}
		seen[p.Page] = true
		if _, ok := types[p.Type]; !ok {
			errs = append(errs, fmt.Errorf("page %q maps to %s, which is not a client type", p.Page, p.Type))
		}
	}
	for _, ig := range m.Ignore {
		if strings.TrimSpace(ig.Reason) == "" {
			errs = append(errs, fmt.Errorf("ignore entry %s.%s has no reason", ig.Type, ig.Field))
		}
	}
	return errs
}

// Check compares each page's deprecations against the types that page's
// response decodes into.
func Check(m Manifest, types Types, deps map[string][]Deprecation) Result {
	var r Result
	ignored := map[string]bool{}
	used := map[string]bool{}
	for _, ig := range m.Ignore {
		ignored[ig.Type+"."+ig.Field] = true
	}
	seen := map[string]bool{}

	for _, p := range m.Pages {
		for _, d := range deps[p.Page] {
			owner, _, decoded := types.Resolve(p.Type, d.Path)
			if !decoded {
				continue // this client does not read the field at all
			}
			field := d.Path[len(d.Path)-1]
			f := Finding{
				Page: p.Page, Type: owner, Field: field,
				Path: strings.Join(d.Path, "."), Replacement: strings.Join(d.Replacement, "."), Note: d.Note,
			}
			switch {
			case d.Replacement == nil:
				f.Kind = KindNoReplacement
			case replacementDecoded(types, p.Type, d):
				f.Kind = KindFallback
			default:
				f.Kind = KindMigrate
			}
			key := string(f.Kind) + "|" + f.Type + "|" + f.Field
			if seen[key] {
				continue // the same field on another page of the same resource
			}
			seen[key] = true
			if f.Kind == KindFallback {
				r.Fallbacks = append(r.Fallbacks, f)
				continue
			}
			if ignored[f.Type+"."+f.Field] {
				used[f.Type+"."+f.Field] = true
				continue
			}
			r.Findings = append(r.Findings, f)
		}
	}
	for _, ig := range m.Ignore {
		if !used[ig.Type+"."+ig.Field] {
			r.UnusedIgnores = append(r.UnusedIgnores, ig)
		}
	}
	r.Unchecked = unchecked(m, types)
	sort.Slice(r.Findings, func(i, j int) bool {
		return r.Findings[i].Type+r.Findings[i].Field < r.Findings[j].Type+r.Findings[j].Field
	})
	sort.Slice(r.Fallbacks, func(i, j int) bool {
		return r.Fallbacks[i].Type+r.Fallbacks[i].Field < r.Fallbacks[j].Type+r.Fallbacks[j].Field
	})
	return r
}

// replacementDecoded reports whether the replacement path resolves from the
// deprecated field's parent object.
func replacementDecoded(types Types, root string, d Deprecation) bool {
	parent := append(append([]string{}, d.Path[:len(d.Path)-1]...), d.Replacement...)
	_, _, ok := types.Resolve(root, parent)
	return ok
}

// requestSuffixes marks types the client sends rather than receives. They are
// not expected to be reachable from a response.
var requestSuffixes = []string{"Create", "Update", "Add", "Write", "Params", "Options", "Request", "Exchange", "Message"}

func unchecked(m Manifest, types Types) []string {
	reach := map[string]bool{}
	var walk func(string)
	walk = func(t string) {
		if reach[t] {
			return
		}
		fields, ok := types[t]
		if !ok {
			return
		}
		reach[t] = true
		for _, f := range fields {
			walk(f.Type)
		}
	}
	for _, p := range m.Pages {
		walk(p.Type)
	}
	var out []string
	for t := range types {
		// Reachable, unexported, or not an API payload at all: a type with no
		// JSON fields, or an error type, never holds a response.
		if reach[t] || strings.HasPrefix(t, strings.ToLower(t[:1])) || len(types[t]) == 0 || strings.HasSuffix(t, "Error") {
			continue
		}
		request := false
		for _, s := range requestSuffixes {
			if strings.HasSuffix(t, s) {
				request = true
				break
			}
		}
		if !request {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}
