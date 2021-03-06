package query

import (
	"fmt"
	"testing"
	"time"

	log "github.com/cihub/seelog"
	"github.com/stretchr/testify/require"
)

func TestParsing(t *testing.T) {
	// Don't put semicolons on the end; that happens automatically
	expectations := map[string]bool{ // query: expect success parsing?
		// EVENT-only
		"EVENT a b":                                 true,
		"EVENT SEQ(a b)":                            true,
		"EVENT SEQ(a b, a d)":                       true,
		"EVENT ANY(a b)":                            true,
		"EVENT ANY(a b, c d)":                       true,
		"EVENT SEQ(a b, ANY(c d, e f))":             true,
		"EVENT SEQ(a e1, !(c e2), ANY(c e3, d e4))": true,
		// Errors
		"EVENT":               false, // No capture
		"EVENT a":             false, // No alias
		"EVENT 1a b":          false, // Identifiers must begin with alpha
		"EVENT a b, c d":      false, // No SEQ
		"EVENT SEQ()":         false, // Empty SEQ (no capture)
		"EVENT ANY()":         false, // Empty ANY (no capture)
		"EVENT SEQ(a b, c b)": false, // Clashing capture aliases

		// EVENT + WHERE
		"EVENT a b WHERE b.foo == 'bar'":                                  true,
		"EVENT a b WHERE b.foo == \"bar\"":                                true,
		"EVENT a b WHERE b.foo != 'bar'":                                  true,
		"EVENT a b WHERE b.foo == 'bar' AND b.bar == 'baz'":               true,
		"EVENT SEQ(t1 e1, t2 e2, ANY(t3 e3, t4 e4)) WHERE e1.a1 == e2.a2": true,
		"EVENT a b WHERE b.n == 1.0":                                      true,
		"EVENT a b WHERE b.n == -1.0":                                     true,
		"EVENT a b WHERE b.n != 1.0":                                      true,
		"EVENT a b WHERE b.n < 1.0":                                       true,
		"EVENT a b WHERE b.n > 1.0":                                       true,
		"EVENT a b WHERE b.n <= 1.0":                                      true,
		"EVENT a b WHERE b.n >= 1.0":                                      true,
		"EVENT SEQ(t a, t b) WHERE a.n == b.n":                            true,
		"EVENT SEQ(t a, t b) WHERE a.n != b.n":                            true,
		"EVENT SEQ(t a, t b) WHERE a.n < b.n":                             true,
		"EVENT SEQ(t a, t b) WHERE a.n > b.n":                             true,
		"EVENT SEQ(t a, t b) WHERE a.n <= b.n":                            true,
		"EVENT SEQ(t a, t b) WHERE a.n >= b.n":                            true,
		// Errors
		"EVENT a b WHERE b.foo == 'bar":    false, // Unterminated quote
		"EVENT a b WHERE b.foo == \"bar":   false, // Unterminated quote
		"EVENT a b WHERE a.foo == \"bar\"": false, // Nonexistant event
		"EVENT a b WHERE b.foo == a.bar":   false, // Nonexistant event

		// EVENT + WITHIN
		"EVENT a b WITHIN 1h":                                  true,
		"EVENT SEQ(a b) WITHIN 30m":                            true,
		"EVENT SEQ(a b, a d) WITHIN 2h30m20s":                  true,
		"EVENT ANY(a b) WITHIN 100h":                           true,
		"EVENT ANY(a b, c d) WITHIN 100000h":                   true,
		"EVENT SEQ(a b, ANY(c d, e f)) WITHIN 200h30m20s100ns": true,
		"EVENT SEQ(a e1, !(c e2), ANY(c e3, d e4)) WITHIN 1h":  true,
		// Errors
		"EVENT a b WITHIN 100000000000000h": false, // Duration overflow
		"EVENT a b WITHIN -4h":              false, // Negative duration
	}

	te := func(queryText string, expectSuccess bool) {
		require.NotPanics(t, func() {
			q, err := Parse(queryText)
			if expectSuccess {
				require.NoError(t, err, fmt.Sprintf("Unexpected error parsing \"%s\"", queryText))
				require.NotNil(t, q, "Query unexpectedly nil for \"%s\"", queryText)

				// If we output the query again, and re-parse it, outputs should be the same
				// We can't compare strings directly because we deliberately standardise output
				output := q.QueryText()
				q2, err := Parse(output)
				require.NoError(t, err, fmt.Sprintf("Unexpected error parsing generated output \"%s\" (original: \"%s\")",
					output, queryText))
				require.Equal(t, output, q2.QueryText(), fmt.Sprintf("Generated outputs do not match for input \"%s\"",
					queryText))
			} else {
				require.Error(t, err, fmt.Sprintf("Error expected parsing \"%s\"", queryText))
				require.Nil(t, q, "Query unexpectedly not-nil for \"%s\"", queryText)
			}
		}, fmt.Sprintf("Unexpected panic parsing \"%s\"", queryText))
	}

	for queryText, expectSuccess := range expectations {
		log.Tracef("[sase:TestParsing] Trying \"%s\"…", queryText)
		te(queryText, expectSuccess)
		te(queryText+";", expectSuccess)
		te(queryText+"     ;", expectSuccess)
	}
}

func TestParsingCaptureNames(t *testing.T) {
	expectations := map[string]map[string]string{ // query text: [alias: type…]
		"EVENT SEQ(t a, t b) WHERE a.n == b.n": {
			"a": "t",
			"b": "t",
		},
		"EVENT SEQ(t a, t b, !(t c))": {
			"a": "t",
			"b": "t",
			"c": "t",
		},
		"EVENT SEQ(t1 e1, t2 e2, ANY(t3 e3, t4 e4))": {
			"e1": "t1",
			"e2": "t2",
			"e3": "t3",
			"e4": "t4",
		},
	}

	for queryText, expectedCaptures := range expectations {
		q, err := Parse(queryText)
		require.NoError(t, err, fmt.Sprintf("Error parsing %s", queryText))
		require.Equal(t, expectedCaptures, q.Captures(), "Unexpected capture result")
	}
}

func TestParsingWindow(t *testing.T) {
	expectations := map[string]time.Duration{
		"1m":    time.Minute,
		"10m":   10 * time.Minute,
		"1h":    time.Hour,
		"1h10m": time.Hour + 10*time.Minute,
	}

	for queryText, expectedDuration := range expectations {
		t.Logf("Trying %s (%s)", queryText, expectedDuration.String())
		q, err := Parse("EVENT t0 e0 WITHIN " + queryText)
		require.NoError(t, err)
		require.Equal(t, expectedDuration, q.Window())
	}
}

func BenchmarkParsing(b *testing.B) {
	queryText := "EVENT SEQ(t1 e1, ANY(t2 e2, t3 e3), !(t4 e4), t5, e5) WHERE e1.foo == e2.bar AND e3.baz == e4.boop WITHIN 2h;"
	for i := 0; i < b.N; i++ {
		Parse(queryText)
	}
}

func BenchmarkParsingParallel(b *testing.B) {
	queryText := "EVENT SEQ(t1 e1, ANY(t2 e2, t3 e3), !(t4 e4), t5, e5) WHERE e1.foo == e2.bar AND e3.baz == e4.boop WITHIN 2h;"
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Parse(queryText)
		}
	})
}
