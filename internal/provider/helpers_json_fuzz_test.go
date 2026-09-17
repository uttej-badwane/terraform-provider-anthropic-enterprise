package provider

import (
	"encoding/json"
	"testing"
)

// These helpers parse JSON that arrives from the API and from user
// configuration, so they see input the provider does not control. The
// properties below hold for every input: the functions may return an error,
// but they may not panic, disagree with themselves, or report two identical
// documents as different.
//
// Seeds cover the shapes that actually occur — agent tool definitions, which
// the API answers with extra keys filled in — alongside the awkward corners of
// JSON itself: deep nesting, duplicate keys, big and small numbers, unicode
// escapes and bare scalars.

func addJSONSeeds(f *testing.F, pairs bool) {
	seeds := []string{
		`{}`,
		`[]`,
		`null`,
		`0`,
		`""`,
		`{"a":1}`,
		`{"a":1,"b":{"c":[1,2,3]}}`,
		`[{"type":"agent_toolset_20260401","configs":[{"name":"bash"}]}]`,
		`[{"type":"agent_toolset_20260401","configs":[{"name":"bash","permission_policy":{"type":"always_ask"}}]}]`,
		`{"a":1,"a":2}`,
		`{"n":1e308}`,
		`{"n":-0.0}`,
		`{"n":9007199254740993}`,
		"{\"s\":\"\\u0000\\uFFFD\"}",
		`{"deep":{"deep":{"deep":{"deep":{"deep":1}}}}}`,
		`  {"spaced" : true }  `,
		`{`,
		`not json`,
		``,
	}
	for _, s := range seeds {
		if pairs {
			f.Add([]byte(s), []byte(s))
		} else {
			f.Add([]byte(s))
		}
	}
}

// FuzzJSONSubsetEqual checks that comparing arbitrary documents never panics,
// and that a document is always a subset of itself. The second property is the
// one that matters in practice: it is what stops a configuration the user did
// not change from showing as drift.
func FuzzJSONSubsetEqual(f *testing.F) {
	addJSONSeeds(f, true)

	f.Fuzz(func(t *testing.T, want, got []byte) {
		equal, err := jsonSubsetEqual(want, got)
		if err != nil {
			// An error is a valid outcome for input that is not JSON, but it
			// must not also claim the documents are equal.
			if equal {
				t.Fatalf("returned equal along with an error: %v", err)
			}
			return
		}

		// Reaching here means both parsed. A document is a subset of itself.
		var v any
		if json.Unmarshal(want, &v) != nil {
			t.Fatalf("no error reported but want does not parse")
		}
		self, err := jsonSubsetEqual(want, want)
		if err != nil {
			t.Fatalf("comparing a parseable document with itself errored: %v", err)
		}
		if !self {
			t.Fatalf("document is not a subset of itself: %s", want)
		}
	})
}

// FuzzCanonicalJSON checks that canonicalisation never panics and is stable:
// running it twice must produce the same bytes, or state comparisons that rely
// on it would flap between plans.
func FuzzCanonicalJSON(f *testing.F) {
	addJSONSeeds(f, false)

	f.Fuzz(func(t *testing.T, raw []byte) {
		once, err := canonicalJSON(raw)
		if err != nil {
			return // not JSON; rejecting it is the correct behaviour
		}

		twice, err := canonicalJSON([]byte(once))
		if err != nil {
			t.Fatalf("canonical output failed to canonicalise: %v (output %q)", err, once)
		}
		if once != twice {
			t.Fatalf("not idempotent:\n first: %q\nsecond: %q", once, twice)
		}

		// Canonical output must still be valid JSON, unless the input was
		// empty, which canonicalJSON maps to the empty string by contract.
		if once != "" && !json.Valid([]byte(once)) {
			t.Fatalf("produced invalid JSON: %q", once)
		}
	})
}

// FuzzCanonicalJSONAgreesWithSubset ties the two together: documents that
// canonicalise identically describe the same data, so neither can be anything
// other than a subset of the other.
func FuzzCanonicalJSONAgreesWithSubset(f *testing.F) {
	addJSONSeeds(f, true)

	f.Fuzz(func(t *testing.T, a, b []byte) {
		ca, err := canonicalJSON(a)
		if err != nil || ca == "" {
			return
		}
		cb, err := canonicalJSON(b)
		if err != nil || cb != ca {
			return
		}

		equal, err := jsonSubsetEqual(a, b)
		if err != nil {
			t.Fatalf("documents canonicalised but did not compare: %v", err)
		}
		if !equal {
			t.Fatalf("identical canonical forms reported as not equal:\n a: %s\n b: %s\n canonical: %s", a, b, ca)
		}
	})
}
