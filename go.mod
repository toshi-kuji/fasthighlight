module github.com/toshi-kuji/fasthighlight

go 1.25

toolchain go1.27.4

// v0.6.1 doubled the size of fhl (net/http for an update notice, removed in v0.6.2).
retract v0.6.1

require github.com/yuin/goldmark v1.8.7

require github.com/yuin/goldmark/v2 v2.1.16
