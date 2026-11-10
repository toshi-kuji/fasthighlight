package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// latestReleaseURL is the GitHub API address of the latest release, which --version asks to tell the user
// about a newer one. Setting FHL_NO_UPDATE_CHECK (to anything) turns the check off.
const latestReleaseURL = "https://api.github.com/repos/toshi-kuji/fasthighlight/releases/latest"

// releasesPage is where a newer release is downloaded.
const releasesPage = "https://github.com/toshi-kuji/fasthighlight/releases/latest"

// noteNewerRelease writes a note to w if the latest release at url is newer than current. It writes nothing
// when current is not a release version (a development build), when the check fails or takes over two
// seconds, or when there is nothing newer: the check must never get in the way of --version.
func noteNewerRelease(w io.Writer, url, current string) {
	cur, ok := parseVersion(current)
	if url == "" || !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&rel) != nil {
		return
	}
	if latest, ok := parseVersion(rel.TagName); ok && newer(latest, cur) {
		fmt.Fprintf(w, "fhl: %s is available (this is %s): %s\n"+
			"     or, if you installed with Go: go install github.com/toshi-kuji/fasthighlight/cmd/fhl@latest\n",
			rel.TagName, current, releasesPage)
	}
}

// parseVersion reads a release version, vX.Y.Z. A pre-release or build suffix (v1.2.3-rc.1) is not a release,
// so it is not read.
func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if !strings.HasPrefix(s, "v") || len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p != strconv.Itoa(n) {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// newer reports whether version a is newer than b.
func newer(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
