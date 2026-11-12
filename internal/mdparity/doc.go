// Package mdparity holds only tests: they check that the markdown/v2 package (goldmark v2) gives the same
// results as the markdown package (goldmark v1). They are kept out of markdown/v2 so that its tests do not
// import goldmark v1; a program that uses markdown/v2 then does not get goldmark v1 in its go.sum.
package mdparity
