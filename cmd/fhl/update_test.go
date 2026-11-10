package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --version notes a newer release, and says nothing when there is none, when this is not a release build,
// or when the lookup fails.
func TestNoteNewerRelease(t *testing.T) {
	latest := "v0.7.0"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch latest {
		case "slow":
			time.Sleep(3 * time.Second)
		case "error":
			http.Error(w, "no", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"tag_name":"` + latest + `"}`))
	}))
	defer srv.Close()
	for _, c := range []struct {
		latest, current string
		note            bool
	}{
		{"v0.7.0", "v0.6.0", true},
		{"v0.6.1", "v0.6.0", true},
		{"v1.0.0", "v0.9.9", true},
		{"v0.6.0", "v0.6.0", false},
		{"v0.5.9", "v0.6.0", false},
		{"v0.7.0", "devel", false},       // a development build
		{"v0.7.0-rc.1", "v0.6.0", false}, // not a release
		{"error", "v0.6.0", false},
		{"slow", "v0.6.0", false}, // gives up after two seconds
	} {
		latest = c.latest
		var w bytes.Buffer
		noteNewerRelease(&w, srv.URL, c.current)
		if got := strings.Contains(w.String(), c.latest+" is available"); got != c.note {
			t.Errorf("latest %s, this %s: %q", c.latest, c.current, w.String())
		}
	}
	// With no URL (FHL_NO_UPDATE_CHECK, and the tests), there is no lookup.
	var w bytes.Buffer
	noteNewerRelease(&w, "", "v0.6.0")
	if w.Len() != 0 {
		t.Errorf("no URL: %q", w.String())
	}
}
