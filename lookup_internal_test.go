package fasthighlight

import "testing"

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
