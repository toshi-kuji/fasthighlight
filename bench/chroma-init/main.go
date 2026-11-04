package main

import (
	"fmt"

	"github.com/alecthomas/chroma/v2/lexers"
)

func main() { fmt.Println(len(lexers.Names(false))) }
