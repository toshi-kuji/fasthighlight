package main

import (
	"testing"

	"github.com/yuin/goldmark/ast"
)

// TestHeadingIDs checks the ids of headings: ASCII text as goldmark makes it, letters and digits of other
// scripts kept, and numbers added to repeated ids.
func TestHeadingIDs(t *testing.T) {
	ids := newHeadingIDs()
	ids.Put([]byte("given"))
	for _, c := range []struct{ heading, want string }{
		{"Results and exit status", "results-and-exit-status"},
		{"  `--theme` MODE:THEME  ", "--theme-modetheme"},
		{"インストール", "インストール"},
		{"使い方（基本）", "使い方基本"},
		{"Go 1.25 で動かす", "go-125-で動かす"},
		{"Ünïcode Straße", "ünïcode-straße"},
		{"インストール", "インストール-1"},
		{"インストール", "インストール-2"},
		{"!!!", "heading"},
		{"？", "heading-1"},
		{"given", "given-1"},
	} {
		if got := string(ids.Generate([]byte(c.heading), ast.KindHeading)); got != c.want {
			t.Errorf("%q: id %q, want %q", c.heading, got, c.want)
		}
	}
}
