package fasthighlight

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Language is the tokenizer for one language.
//
// A language is registered with Register when its package is imported (the same mechanism as
// database/sql drivers in Go). To use all languages, import
// github.com/toshi-kuji/fasthighlight/lang/all.
//
// The information about the languages fasthighlight provides (names and so on) is in the built-in
// catalog (Builtins). A language package receives its information with Builtin, adds Lex and
// Prepare, and registers it.
type Language struct {
	// Name is the name of the language. It is the same as name in the Chroma language definition (e.g. "TypeScript").
	Name string
	// Aliases are the names accepted as a code block's language name. They are the same as Chroma's alias.
	// The lower-cased Name is also accepted automatically.
	Aliases []string
	// Filenames are file name patterns (e.g. "*.ts"). They are the same as Chroma's filename.
	// When a language name is in neither the names nor the aliases, it is matched as an extension (e.g. "mts")
	// or as a file name, as in Chroma.
	Filenames []string
	// EnsureNewline is Chroma's ensure_nl. If true, input that does not end with a newline gets one appended
	// before it is split. As in Chroma, the appended newline stays in the output only when it is included
	// in a token that started before it (e.g. "// c" becomes "// c\n", but no newline follows "x = 1").
	EnsureNewline bool
	// Package is the import path of the package that registers this language. Only languages provided by
	// fasthighlight have it.
	Package string
	// ParallelHint tells how much work highlighting this language takes, which decides how few code blocks
	// are enough to highlight a document in parallel (see the markdown package). It is HintHeavy,
	// HintLight, or HintDefault (not specified).
	//
	// A language you register yourself should be chosen by the same criterion as the languages
	// fasthighlight provides. Among those, the heavy ones are the programming languages (Bash, C, C#, C++,
	// CSS, Go, Java, JavaScript, Kotlin, PHP, PowerShell, Python, R, react, Ruby, Rust, SCSS, Swift and
	// TypeScript) and the languages mostly read as Bash (Bash Session, Docker and Makefile); the others are
	// not specified, which is
	// treated like HintLight. When in doubt, leave it unspecified or use HintLight (going parallel is less
	// likely to make things slower).
	ParallelHint ParallelHint

	// Lex splits text into tokens. It is called from Tokenise.
	// text is the input with a newline appended if EnsureNewline is set. end is the length before the
	// append, and Lex must not start a new token (one match of a Chroma rule) at or after end. A match that
	// started before end may include the appended newline past end (as in Chroma). The concatenated
	// tokens are the part of text from its start (of length end or len(text)).
	Lex func(text string, end int) []Token
	// Prepare builds things such as keyword tables (optional). It is called once, before the first
	// Tokenise. So that only the languages in use are prepared, the tables are built here, not at import time.
	Prepare func()

	prepareOnce sync.Once
}

// Tokenise splits code into tokens. As in Chroma, it first normalizes newlines to \n (\r\n and \r become \n).
// If EnsureNewline is true, a newline is appended before splitting. The concatenated tokens are the
// newline-normalized code, or that with a newline added.
func (l *Language) Tokenise(code string) []Token {
	l.prepare()
	code = NormalizeNewlines(code)
	end := len(code)
	if l.EnsureNewline && !strings.HasSuffix(code, "\n") {
		code += "\n"
	}
	return l.Lex(code, end)
}

// ParallelHint tells how much work highlighting a language takes. It decides how few code blocks are
// enough to highlight a document in parallel.
type ParallelHint int

const (
	// HintDefault means the hint is not specified (the zero value). Parallel highlighting treats it like
	// HintLight. It is kept apart from HintLight so that an unspecified hint can be told apart from a
	// chosen one.
	HintDefault ParallelHint = iota
	// HintHeavy means highlighting takes much work, so going parallel pays off with fewer code blocks.
	HintHeavy
	// HintLight means highlighting takes little work, so going parallel pays off only with more code blocks.
	HintLight
)

func (l *Language) prepare() {
	if l.Prepare != nil {
		l.prepareOnce.Do(l.Prepare)
	}
}

func (l *Language) keys() []string {
	keys := []string{strings.ToLower(l.Name)}
	for _, a := range l.Aliases {
		keys = append(keys, strings.ToLower(a))
	}
	return keys
}

// languageSet is a collection from which languages can be looked up by name and alias. It is used for the
// registered languages and for the built-in catalog.
type languageSet struct {
	byKey map[string]*Language
	all   []*Language // in name order
	// cache holds the results (including not-found) of looking up, by file name pattern, a language name that
	// is in neither the names nor the aliases. Documents often contain out-of-scope language names such as
	// "text", so this avoids matching the patterns every time.
	cache *lookupCache
}

// lookupCache remembers the results of a languageSet lookup. It is rebuilt when a language is added.
type lookupCache struct {
	m sync.Map // string → *Language
	n atomic.Int32
}

// maxCache is the upper limit on the number of language names put in cache. Documents usually contain few
// distinct language names.
const maxCache = 4096

// add adds l. If a name or alias collides with another language, it returns an error and changes nothing.
func (s *languageSet) add(l *Language) error {
	if s.byKey == nil {
		s.byKey = map[string]*Language{}
	}
	keys := l.keys()
	for _, k := range keys {
		if other, dup := s.byKey[k]; dup && other != l {
			return fmt.Errorf("%q of %s is already used by %s", k, l.Name, other.Name)
		}
	}
	for _, k := range keys {
		s.byKey[k] = l
	}
	s.cache = new(lookupCache)
	for _, other := range s.all {
		if other == l {
			return nil
		}
	}
	s.all = append(s.all, l)
	sort.Slice(s.all, func(i, j int) bool { return s.all[i].Name < s.all[j].Name })
	return nil
}

// lookup searches in the same order as Chroma's lexers.Get.
//
//  1. Name and aliases (case-insensitive)
//  2. As an extension (for "mts", whether "filename.mts" matches Filenames)
//  3. As a file name (whether "app.ts" matches Filenames; for a path such as "src/app.ts", the last part is used)
//
// In 2 and 3, as in Chroma, names with a backup suffix (".bak", "~", and so on) are matched too.
// When several languages match, it returns the first one in name order (Chroma chooses by priority, then
// by registration order; among the current target languages the result is the same).
func (s *languageSet) lookup(name string) *Language {
	if l := s.byKey[strings.ToLower(name)]; l != nil {
		return l
	}
	if name == "" {
		return nil
	}
	c := s.cache
	if c != nil {
		if l, ok := c.m.Load(name); ok {
			return l.(*Language)
		}
	}
	var found *Language
	for _, file := range [...]string{"filename." + name, name} {
		if found = s.matchFile(path.Base(file)); found != nil {
			break
		}
	}
	if c != nil && c.n.Add(1) <= maxCache {
		c.m.Store(name, found)
	}
	return found
}

// matchFile returns the first language (in name order) whose pattern matches the file name file. As in
// Chroma, it also matches the pattern with a backup suffix appended. That is the same as the file name
// minus the suffix matching the pattern, so the candidates with the suffix removed are built first.
func (s *languageSet) matchFile(file string) *Language {
	var buf [1 + len(ignoredSuffixes)]string
	files := append(buf[:0], file)
	for _, suf := range ignoredSuffixes {
		if f, ok := strings.CutSuffix(file, suf); ok {
			files = append(files, f)
		}
	}
	for _, l := range s.all {
		for _, pat := range l.Filenames {
			for _, f := range files {
				if globMatch(pat, f) {
					return l
				}
			}
		}
	}
	return nil
}

// ignoredSuffixes are the backup suffixes Chroma ignores when matching file names.
var ignoredSuffixes = [...]string{
	"~", ".bak", ".old", ".orig",
	".dpkg-dist", ".dpkg-old", ".ucf-dist", ".ucf-new", ".ucf-old",
	".rpmnew", ".rpmorig", ".rpmsave",
}

// globMatch is the same as path.Match(pat, file). The common forms ("*.ts" and names without special
// characters) are handled by string comparison (file has no /).
func globMatch(pat, file string) bool {
	if ext, ok := strings.CutPrefix(pat, "*"); ok && !strings.ContainsAny(ext, `*?[\`) {
		return strings.HasSuffix(file, ext)
	}
	if !strings.ContainsAny(pat, `*?[\`) {
		return pat == file
	}
	ok, _ := path.Match(pat, file)
	return ok
}

var registry struct {
	sync.RWMutex
	languageSet
}

// Register registers a language. It panics if a name or alias collides with a registered language (the
// registered languages are not changed). It is normally called from the init of a language package.
func Register(l *Language) {
	if l == nil || l.Name == "" || l.Lex == nil {
		panic("fasthighlight: Register: language must have a Name and a Lex function")
	}
	switch l.ParallelHint {
	case HintDefault, HintHeavy, HintLight:
	default:
		panic("fasthighlight: Register: ParallelHint of " + l.Name + " must be HintDefault, HintHeavy or HintLight")
	}
	registry.Lock()
	defer registry.Unlock()
	if err := registry.add(l); err != nil {
		panic("fasthighlight: Register: " + err.Error())
	}
}

// LookupRegistered looks up a code block's language name among the languages that can be used for
// highlighting now: the registered ones, that is, those of the imported language packages and those
// registered directly with Register. It searches in the same order as Chroma's lexers.Get (names and
// aliases, extension, file name). It returns nil if not found.
func LookupRegistered(name string) *Language {
	registry.RLock()
	defer registry.RUnlock()
	return registry.lookup(name)
}

// LookupSupported looks up a code block's language name among the languages fasthighlight provides (the
// built-in catalog, see Builtins), whether their packages are imported or not. Languages registered
// directly with Register are not included. It searches the same way as LookupRegistered and returns nil if
// not found.
//
// The returned language's Package is the package to import to highlight it. If that package is not
// imported, the language is not registered and cannot be used for highlighting (it has no Lex).
func LookupSupported(name string) *Language {
	return catalog.lookup(name)
}

// IsPlainText reports whether a code block's language name marks it as plain text on purpose: the name or
// an alias of Chroma's plaintext lexer (plaintext, text, plain, no-highlight), in any case. Such a block is
// not highlighted, like any language that is not registered, but it is not missing a language, so
// markdown.Scan and fhl do not report it.
func IsPlainText(name string) bool {
	switch strings.ToLower(name) {
	case "plaintext", "text", "plain", "no-highlight":
		return true
	}
	return false
}

// Languages returns the registered languages in name order.
func Languages() []*Language {
	registry.RLock()
	defer registry.RUnlock()
	return append([]*Language(nil), registry.all...)
}

// NormalizeNewlines turns \r\n and \r into \n (the same as Chroma's EnsureLF). If there is no \r, it returns s as is.
func NormalizeNewlines(s string) string {
	if strings.IndexByte(s, '\r') < 0 {
		return s
	}
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\r' {
			if i+1 < len(s) && s[i+1] == '\n' {
				continue
			}
			c = '\n'
		}
		b = append(b, c)
	}
	return string(b)
}
