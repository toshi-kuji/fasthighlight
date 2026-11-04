package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	fh "github.com/toshi-kuji/fasthighlight"
)

// styleName is the file name of the color theme CSS.
const styleName = "highlight.css"

// job is the conversion of one input.
type job struct {
	input  string // name of the input used in reports ("-" for standard input)
	path   string // file to read; empty for standard input
	stdin  bool
	lang   *fh.Language // language when read as code; nil for Markdown
	langID string       // for code, the name put in data-lang
	output string       // file to write; "-" for standard output, empty if nothing is written (--check)
	title  string       // <title> used when there is no heading
}

// planError is an error found before conversion starts. inputs are the inputs involved.
type planError struct {
	msg    string
	inputs []string
}

func (e *planError) Error() string { return e.msg }

// fatalError is an error that ends the run without converting any input (such as an invalid flag combination).
type fatalError struct{ msg string }

func (e *fatalError) Error() string { return e.msg }

func fatalf(format string, a ...any) error { return &fatalError{fmt.Sprintf(format, a...)} }

// plan collects all inputs, decides the output destination of each, and removes those that collide.
// The returned jobs are the conversions to run (in input order). errs are the errors of the inputs that are not converted.
// styles holds the path of highlight.css for each directory where it is written (empty if it is not written).
func plan(o *options) (jobs []*job, errs []*planError, err error) {
	mode := outNone
	switch {
	case o.check:
	case !o.outSet:
	case o.out == "-":
		mode = outStdout
	case isDirPath(o.out):
		mode = outDir
	default:
		mode = outFile
	}

	stdinSeen := false
	folders := 0
	for _, in := range o.inputs {
		if in == "-" {
			if stdinSeen {
				return nil, nil, fatalf("fhl: - (standard input) can be given only once")
			}
			stdinSeen = true
			j := &job{input: "-", stdin: true}
			if o.lang != "" {
				j.lang, j.langID = fh.LookupRegistered(o.lang), o.lang
			}
			jobs = append(jobs, j)
			continue
		}
		fi, serr := os.Stat(in)
		if serr != nil {
			errs = append(errs, &planError{msg: fmt.Sprintf("fhl: cannot read %s: %v", in, cause(serr)), inputs: []string{in}})
			continue
		}
		if !fi.IsDir() {
			j, perr := fileJob(o, in, filepath.Base(in))
			if perr != nil {
				errs = append(errs, perr)
				continue
			}
			jobs = append(jobs, j)
			continue
		}
		folders++
		skip := outputFolder(mode, o.out)
		werr := filepath.WalkDir(in, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				errs = append(errs, &planError{msg: fmt.Sprintf("fhl: cannot read %s: %v", p, cause(err)), inputs: []string{p}})
				return nil
			}
			if p != in && strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if p != in && d.IsDir() && skip(p) {
				return filepath.SkipDir // the output folder, inside the input folder: earlier output is not input
			}
			if d.IsDir() || !isMarkdown(p) {
				return nil
			}
			if !d.Type().IsRegular() {
				// Of symbolic links, only those pointing to files are read (directories are not followed).
				if fi, err := os.Stat(p); err != nil || !fi.Mode().IsRegular() {
					return nil
				}
			}
			rel, _ := filepath.Rel(in, p)
			j := &job{input: p, path: p, title: filepath.Base(p)}
			switch mode {
			case outNone:
				j.output = filepath.Join(filepath.Dir(p), htmlName(p))
			case outDir:
				j.output = filepath.Join(o.out, filepath.Dir(rel), htmlName(p))
			}
			jobs = append(jobs, j)
			return nil
		})
		if werr != nil {
			return nil, nil, werr
		}
	}

	switch mode {
	case outFile, outStdout:
		if len(o.inputs) != 1 || folders > 0 {
			if mode == outStdout {
				return nil, nil, fatalf("fhl: -o - needs exactly one input file")
			}
			return nil, nil, fatalf("fhl: -o %s needs exactly one input file; use -o %s/ for folders or several inputs", o.out, strings.TrimRight(o.out, "/"))
		}
	}
	if stdinSeen && mode == outNone && !o.check {
		return nil, nil, fatalf("fhl: reading standard input needs -o FILE, -o DIR/ or -o -")
	}

	for _, j := range jobs {
		switch {
		case o.check:
			j.output = ""
		case mode == outStdout:
			j.output = "-"
		case mode == outFile:
			j.output = o.out
		case j.output != "":
			// files inside the directory (decided above)
		case j.stdin && mode == outDir:
			j.output = filepath.Join(o.out, "stdin.html")
		case mode == outDir:
			j.output = filepath.Join(o.out, htmlName(j.path))
		default:
			j.output = filepath.Join(filepath.Dir(j.path), htmlName(j.path))
		}
		if j.stdin {
			switch j.output {
			case "", "-":
				j.title = "stdin"
			default:
				j.title = filepath.Base(j.output)
			}
		}
	}
	if o.check || mode == outStdout {
		return jobs, errs, nil
	}
	jobs, cerrs := collisions(jobs, !o.single && !o.staticSite)
	return jobs, append(errs, cerrs...), nil
}

// Kinds of output destination.
const (
	outNone   = iota // no -o: the same directory as the input
	outFile          // -o file
	outDir           // -o directory/
	outStdout        // -o -
)

// fileJob builds the conversion of the input in, which is not a directory.
func fileJob(o *options, in, base string) (*job, *planError) {
	j := &job{input: in, path: in, title: base}
	if isMarkdown(in) {
		return j, nil
	}
	if o.lang != "" {
		j.lang, j.langID = fh.LookupRegistered(o.lang), o.lang
		return j, nil
	}
	if j.lang = fh.LookupRegistered(base); j.lang == nil {
		return nil, &planError{msg: fmt.Sprintf("fhl: no language for %s; use --lang NAME (see fhl --list-langs)", in), inputs: []string{in}}
	}
	j.langID = strings.ToLower(j.lang.Name)
	return j, nil
}

// isMarkdown returns true if name is the name of a Markdown file.
func isMarkdown(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".md", ".markdown", ".mkd":
		return true
	}
	return false
}

// htmlName is the input name with its extension changed to .html.
func htmlName(p string) string {
	base := filepath.Base(p)
	return strings.TrimSuffix(base, filepath.Ext(base)) + ".html"
}

// isDirPath returns true if the value of -o is treated as a directory (it ends with / or is an existing directory).
func isDirPath(p string) bool {
	if strings.HasSuffix(p, "/") || strings.HasSuffix(p, string(filepath.Separator)) {
		return true
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// cause extracts the reason, without the path, from an os error (so the path is not written twice in the message).
func cause(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// fileKey is a string identifying the file that path p points to. Paths to the same underlying file give the same string.
//   - If the file exists, it is identified by the underlying file (device and inode numbers; the path below if the OS does not support that).
//   - Otherwise, it is identified by the lowercased absolute path with the symbolic links of the existing part resolved
//     (so paths that are the same file on a case-insensitive file system count as colliding).
func fileKey(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	if fi, err := os.Stat(abs); err == nil {
		if id, ok := fileID(fi); ok {
			return "id:" + id
		}
	}
	return "path:" + strings.ToLower(resolve(abs))
}

// resolve resolves the symbolic links in the existing part of the absolute path abs.
func resolve(abs string) string {
	dir, rest := abs, ""
	for {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(r, rest)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
}

// collisions removes the conversions whose output destinations collide. If css is true, it also compares the
// highlight.css of each directory where HTML is written.
func collisions(jobs []*job, css bool) ([]*job, []*planError) {
	var errs []*planError
	inputs := map[string]string{} // fileKey of the input -> name of the input
	for _, j := range jobs {
		if !j.stdin {
			inputs[fileKey(j.path)] = j.input
		}
	}

	// Conversions that write the same input to the same destination (such as fhl docs/ docs/a.md) are merged into one.
	type pair struct{ in, out string }
	seen := map[pair]bool{}
	outKey := map[*job]string{}
	var uniq []*job
	for _, j := range jobs {
		k := fileKey(j.output)
		outKey[j] = k
		if !j.stdin {
			p := pair{fileKey(j.path), k}
			if seen[p] {
				continue
			}
			seen[p] = true
		}
		uniq = append(uniq, j)
	}
	jobs = uniq

	// Those whose output destination collides with an input.
	var rest []*job
	for _, j := range jobs {
		in, ok := inputs[outKey[j]]
		switch {
		case !ok:
			rest = append(rest, j)
		case in == j.input:
			errs = append(errs, &planError{msg: fmt.Sprintf("fhl: %s would overwrite its own input; choose another -o", j.input), inputs: []string{j.input}})
		default:
			errs = append(errs, &planError{msg: fmt.Sprintf("fhl: %s would write %s, which is the input %s; choose another -o", j.input, j.output, in), inputs: []string{j.input}})
		}
	}

	// Those whose output destinations collide with each other.
	groups := map[string][]*job{}
	for _, j := range rest {
		groups[outKey[j]] = append(groups[outKey[j]], j)
	}
	jobs = rest[:0:0]
	reported := map[string]bool{}
	for _, j := range rest {
		g := groups[outKey[j]]
		if len(g) == 1 {
			jobs = append(jobs, j)
			continue
		}
		if reported[outKey[j]] {
			continue
		}
		reported[outKey[j]] = true
		names := make([]string, len(g))
		for i, x := range g {
			names[i] = x.input
		}
		both := "both"
		if len(g) > 2 {
			both = "all"
		}
		errs = append(errs, &planError{
			msg:    fmt.Sprintf("fhl: %s would %s write %s; rename one of them or convert them separately", joinAnd(names), both, j.output),
			inputs: names,
		})
	}
	if !css {
		return jobs, errs
	}

	// Those whose highlight.css collides.
	rest, jobs = jobs, nil
	byDir := map[string][]*job{}
	for _, j := range rest {
		d := filepath.Dir(j.output)
		byDir[d] = append(byDir[d], j)
	}
	bad := map[*job]bool{}
	for _, j := range rest {
		style := filepath.Join(filepath.Dir(j.output), styleName)
		k := fileKey(style)
		if k == outKey[j] {
			errs = append(errs, &planError{msg: fmt.Sprintf("fhl: %s would write %s, where fhl writes the style sheet; choose another -o", j.input, j.output), inputs: []string{j.input}})
			bad[j] = true
			continue
		}
		if in, ok := inputs[k]; ok && !bad[j] {
			g := byDir[filepath.Dir(j.output)]
			names := make([]string, 0, len(g))
			for _, x := range g {
				if !bad[x] {
					bad[x] = true
					names = append(names, x.input)
				}
			}
			errs = append(errs, &planError{msg: fmt.Sprintf("fhl: writing %s would overwrite the input %s; choose another -o", style, in), inputs: names})
		}
	}
	for _, j := range rest {
		if !bad[j] {
			jobs = append(jobs, j)
		}
	}
	return jobs, errs
}

// joinAnd joins names in the form "a, b and c".
func joinAnd(names []string) string {
	if len(names) <= 2 {
		return strings.Join(names, " and ")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// copyJob is a file that --static-site copies as it is.
// outputFolder returns a function that tells whether the folder p is the output folder (-o DIR/), so that a walk
// of an input folder that contains it can skip it. Without an output folder, it is always false.
func outputFolder(mode int, out string) func(p string) bool {
	if mode != outDir {
		return func(string) bool { return false }
	}
	key := fileKey(out)
	return func(p string) bool { return fileKey(p) == key }
}

type copyJob struct{ src, dst string }

// planCopies lists the files in the folder in that are not Markdown (images and so on), to be copied to the same
// place under the folder out. Names starting with . are skipped, as for Markdown. A file that would be written where
// fhl writes a page or the style sheet (outputs) is not copied, and is reported as an error. If out is the same
// folder as in, nothing is copied.
func planCopies(in, out string, outputs []string) ([]copyJob, []*planError) {
	if fileKey(in) == fileKey(out) {
		return nil, nil
	}
	taken := map[string]bool{}
	for _, p := range outputs {
		taken[fileKey(p)] = true
	}
	var copies []copyJob
	var errs []*planError
	skip := outputFolder(outDir, out)
	_ = filepath.WalkDir(in, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, &planError{msg: fmt.Sprintf("fhl: cannot read %s: %v", p, cause(err)), inputs: []string{p}})
			return nil
		}
		if p != in && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if p != in && d.IsDir() && skip(p) {
			return filepath.SkipDir // the output folder, inside the input folder: the pages and copies just written
		}
		if d.IsDir() || isMarkdown(p) {
			return nil
		}
		if !d.Type().IsRegular() {
			// Of symbolic links, only those pointing to files are copied (directories are not followed).
			if fi, err := os.Stat(p); err != nil || !fi.Mode().IsRegular() {
				return nil
			}
		}
		rel, _ := filepath.Rel(in, p)
		dst := filepath.Join(out, rel)
		if taken[fileKey(dst)] {
			errs = append(errs, &planError{msg: fmt.Sprintf("fhl: %s would be copied to %s, where fhl writes a page or the style sheet; rename it", p, dst), inputs: []string{p}})
			return nil
		}
		copies = append(copies, copyJob{p, dst})
		return nil
	})
	return copies, errs
}
