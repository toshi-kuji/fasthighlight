package fasthighlight

// The list of languages fasthighlight provides. It has no tokenizers, only names, aliases, file name
// patterns, and so on. Because the core package holds this list, "languages that are supported but not
// built in" can be told apart even when the language package is not imported (see LookupSupported).
//
// The names, aliases, file name patterns, and EnsureNewline are the same as in the Chroma 2.27.0 language
// definitions (checked by TestLanguagesMatchChroma in compat/). When adding a language, add it both here and in lang/.

const langPackage = "github.com/toshi-kuji/fasthighlight/lang/"

var catalog = func() languageSet {
	var s languageSet
	for _, l := range []*Language{
		{
			Name:         "Bash",
			ParallelHint: HintHeavy,
			Aliases:      []string{"bash", "sh", "ksh", "zsh", "shell"},
			Filenames: []string{
				"*.sh", "*.ksh", "*.bash", "*.ebuild", "*.eclass", ".env", ".env.*", "*.env", "*.exheres-0",
				"*.exlib", "*.zsh", "*.zshrc", ".bashrc", "bashrc", ".bash_*", "bash_*", "zshrc", ".zshrc",
				"APKBUILD", "PKGBUILD",
			},
			Package: langPackage + "bash",
		},
		{
			Name:          "C",
			ParallelHint:  HintHeavy,
			Aliases:       []string{"c"},
			Filenames:     []string{"*.c", "*.h", "*.idc", "*.x[bp]m"},
			EnsureNewline: true,
			Package:       langPackage + "c",
		},
		{
			Name:          "C#",
			ParallelHint:  HintHeavy,
			Aliases:       []string{"csharp", "c#"},
			Filenames:     []string{"*.cs"},
			EnsureNewline: true,
			Package:       langPackage + "csharp",
		},
		{
			Name:         "C++",
			ParallelHint: HintHeavy,
			Aliases:      []string{"cpp", "c++"},
			Filenames: []string{
				"*.cpp", "*.hpp", "*.c++", "*.h++", "*.cc", "*.hh", "*.cxx", "*.hxx", "*.C", "*.H", "*.cp", "*.CPP", "*.tpp",
			},
			EnsureNewline: true,
			Package:       langPackage + "cpp",
		},
		{
			Name:         "CSS",
			ParallelHint: HintHeavy,
			Aliases:      []string{"css"},
			Filenames:    []string{"*.css"},
			Package:      langPackage + "css",
		},
		{
			Name:          "Diff",
			Aliases:       []string{"diff", "udiff"},
			Filenames:     []string{"*.diff", "*.patch"},
			EnsureNewline: true,
			Package:       langPackage + "diff",
		},
		{
			Name:         "Go",
			ParallelHint: HintHeavy,
			Aliases:      []string{"go", "golang"},
			Filenames:    []string{"*.go"},
			Package:      langPackage + "golang",
		},
		{
			Name:      "HTML",
			Aliases:   []string{"html"},
			Filenames: []string{"*.html", "*.htm", "*.xhtml", "*.xslt"},
			Package:   langPackage + "html",
		},
		{
			Name:    "HTTP",
			Aliases: []string{"http"},
			Package: langPackage + "http",
		},
		{
			Name:    "INI",
			Aliases: []string{"ini", "cfg", "dosini"},
			Filenames: []string{
				"*.ini", "*.cfg", "*.inf", "*.service", "*.socket", "*.container", "*.network", "*.build",
				"*.pod", "*.kube", "*.volume", "*.image", ".gitconfig", ".editorconfig", "pylintrc", ".pylintrc",
			},
			Package: langPackage + "ini",
		},
		{
			Name:      "JSON",
			Aliases:   []string{"json", "jsonl"},
			Filenames: []string{"*.json", "*.jsonl", "*.jsonc", "*.json5", "*.avsc", ".luaurc"},
			Package:   langPackage + "json",
		},
		{
			Name:         "Java",
			ParallelHint: HintHeavy,
			Aliases:      []string{"java"},
			Filenames:    []string{"*.java"},
			Package:      langPackage + "java",
		},
		{
			Name:          "JavaScript",
			ParallelHint:  HintHeavy,
			Aliases:       []string{"js", "javascript"},
			Filenames:     []string{"*.js", "*.jsm", "*.mjs", "*.cjs"},
			EnsureNewline: true,
			Package:       langPackage + "javascript",
		},
		{
			Name:         "Kotlin",
			ParallelHint: HintHeavy,
			Aliases:      []string{"kotlin"},
			Filenames:    []string{"*.kt", "*.kts"},
			Package:      langPackage + "kotlin",
		},
		{
			Name:          "PHP",
			ParallelHint:  HintHeavy,
			Aliases:       []string{"php", "php3", "php4", "php5"},
			Filenames:     []string{"*.php", "*.php[345]", "*.inc"},
			EnsureNewline: true,
			Package:       langPackage + "php",
		},
		{
			Name:         "Python",
			ParallelHint: HintHeavy,
			Aliases:      []string{"python", "py", "sage", "python3", "py3", "starlark"},
			Filenames: []string{
				"*.py", "*.pyi", "*.pyw", "*.jy", "*.sage", "*.sc", "SConstruct", "SConscript", "*.bzl",
				"BUCK", "BUILD", "BUILD.bazel", "WORKSPACE", "WORKSPACE.bzlmod", "WORKSPACE.bazel",
				"MODULE.bazel", "REPO.bazel", "*.star", "*.tac",
			},
			Package: langPackage + "python",
		},
		{
			Name:         "R",
			ParallelHint: HintHeavy,
			Aliases:      []string{"splus", "s", "r"},
			Filenames:    []string{"*.S", "*.R", "*.r", ".Rhistory", ".Rprofile", ".Renviron"},
			Package:      langPackage + "r",
		},
		{
			Name:         "Ruby",
			ParallelHint: HintHeavy,
			Aliases:      []string{"rb", "ruby", "duby"},
			Filenames: []string{
				"*.rb", "*.rbw", "Rakefile", "*.rake", "*.gemspec", "*.rbx", "*.duby", "Gemfile", "*.gemfile",
				"Vagrantfile", "Appraisals", ".pryrc", "*.json.jbuilder",
			},
			Package: langPackage + "ruby",
		},
		{
			Name:          "Rust",
			ParallelHint:  HintHeavy,
			Aliases:       []string{"rust", "rs"},
			Filenames:     []string{"*.rs", "*.rs.in"},
			EnsureNewline: true,
			Package:       langPackage + "rust",
		},
		{
			Name:      "SQL",
			Aliases:   []string{"sql"},
			Filenames: []string{"*.sql"},
			Package:   langPackage + "sql",
		},
		{
			Name:         "Swift",
			ParallelHint: HintHeavy,
			Aliases:      []string{"swift"},
			Filenames:    []string{"*.swift"},
			Package:      langPackage + "swift",
		},
		{
			Name:      "TOML",
			Aliases:   []string{"toml"},
			Filenames: []string{"*.toml", "Pipfile", "poetry.lock", "uv.lock"},
			Package:   langPackage + "toml",
		},
		{
			Name:          "TypeScript",
			ParallelHint:  HintHeavy,
			Aliases:       []string{"ts", "tsx", "typescript"},
			Filenames:     []string{"*.ts", "*.tsx", "*.mts", "*.cts"},
			EnsureNewline: true,
			Package:       langPackage + "typescript",
		},
		{
			Name:    "XML",
			Aliases: []string{"xml"},
			Filenames: []string{
				"*.xml", "*.xsl", "*.rss", "*.xslt", "*.xsd", "*.wsdl", "*.wsf", "*.svg", "*.qrc",
				"*.csproj", "*.vcxproj", "*.fsproj",
			},
			Package: langPackage + "xml",
		},
		{
			Name:      "YAML",
			Aliases:   []string{"yaml"},
			Filenames: []string{"*.yaml", "*.yml"},
			Package:   langPackage + "yaml",
		},
		{
			Name:      "markdown",
			Aliases:   []string{"md", "mkd"},
			Filenames: []string{"*.md", "*.mkd", "*.markdown"},
			Package:   langPackage + "markdown",
		},
	} {
		if err := s.add(l); err != nil {
			panic("fasthighlight: catalog: " + err.Error())
		}
	}
	return s
}()

// Builtin returns the information of a language fasthighlight provides. A language package uses it to
// Register with Lex and Prepare added. It panics if the name is not in the list.
func Builtin(name string) *Language {
	for _, l := range catalog.all {
		if l.Name == name {
			return l
		}
	}
	panic("fasthighlight: Builtin: unknown language " + name)
}

// Builtins returns the information of all the languages fasthighlight provides, in name order. It includes languages that are not imported.
func Builtins() []*Language {
	return append([]*Language(nil), catalog.all...)
}
