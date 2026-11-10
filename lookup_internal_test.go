package fasthighlight

import (
	"fmt"
	"strings"
	"testing"
)

// withFakeLanguages adds test languages to the built-in catalog and the registered languages, runs f, and restores them.
func withFakeLanguages(t *testing.T, builtin, registered []*Language, f func()) {
	t.Helper()
	saveSet := func(s *languageSet) languageSet {
		c := languageSet{byKey: map[string]*Language{}, all: append([]*Language(nil), s.all...), cache: new(lookupCache)}
		for k, v := range s.byKey {
			c.byKey[k] = v
		}
		return c
	}
	savedCatalog := saveSet(&catalog)
	registry.Lock()
	savedRegistry := saveSet(&registry.languageSet)
	registry.Unlock()
	defer func() {
		catalog = savedCatalog
		registry.Lock()
		registry.languageSet = savedRegistry
		registry.Unlock()
	}()
	for _, l := range builtin {
		if err := catalog.add(l); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range registered {
		Register(l)
	}
	f()
}

// The results of lookup by file name pattern are remembered, but remembered anew when a language is registered.
func TestLookupCacheIsResetOnRegister(t *testing.T) {
	withFakeLanguages(t, nil, nil, func() {
		if LookupRegistered("x.fakeext") != nil {
			t.Fatal("unexpected language for x.fakeext")
		}
		fake := &Language{Name: "Fakeext", Filenames: []string{"*.fakeext"}, Lex: func(c string, _ int) []Token { return nil }}
		Register(fake)
		if got := LookupRegistered("x.fakeext"); got != fake {
			t.Errorf("after Register, LookupRegistered(x.fakeext) = %v, want the new language", got)
		}
	})
}

// The cache does not stay full: after maxCache names it is emptied, so later names are cached again, and long
// names are not stored.
func TestLookupCacheBounded(t *testing.T) {
	s := languageSet{byKey: map[string]*Language{}, cache: new(lookupCache)}
	count := func() int {
		n := 0
		s.cache.m.Range(func(_, _ any) bool { n++; return true })
		return n
	}
	for i := range 3*maxCache + 10 {
		s.lookup(fmt.Sprintf("bogus%d", i))
		if n := count(); n > maxCache {
			t.Fatalf("after %d names, the cache holds %d (limit %d)", i+1, n, maxCache)
		}
	}
	s.lookup("late")
	if _, ok := s.cache.m.Load("late"); !ok {
		t.Error("a name looked up after the cache filled is not cached")
	}
	long := strings.Repeat("x", maxCachedName+1)
	s.lookup(long)
	if _, ok := s.cache.m.Load(long); ok {
		t.Error("a name longer than maxCachedName is cached")
	}
}
