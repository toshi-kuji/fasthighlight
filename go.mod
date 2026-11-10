module github.com/toshi-kuji/fasthighlight

go 1.24

toolchain go1.25.0

// v0.6.1 doubled the size of fhl (net/http for an update notice, removed in v0.6.2).
retract v0.6.1

require github.com/yuin/goldmark v1.8.7
