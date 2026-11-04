module github.com/toshi-kuji/fasthighlight/compat

go 1.25

require (
	github.com/alecthomas/chroma/v2 v2.27.0
	github.com/toshi-kuji/fasthighlight v0.0.0
	github.com/yuin/goldmark v1.8.7
)

require github.com/dlclark/regexp2/v2 v2.2.1 // indirect

replace github.com/toshi-kuji/fasthighlight => ../
