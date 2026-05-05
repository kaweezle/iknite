// cSpell: words Stmts fset forbidigo gosec gocyclo wrapcheck
//
//nolint:gosec,gocyclo // Vibe coded tool
package main

import (
	"bufio"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type lineRange struct {
	expandedFrom *lineRange
	start        int
	end          int
	col          int
}

type fileAnalysis struct {
	nocovRanges []lineRange
}

type coverageEntry struct {
	filePath   string
	original   string
	startLine  int
	startCol   int
	endLine    int
	endCol     int
	numStmts   int
	execCount  int
	lineNumber int
}

func (e *coverageEntry) Len() int {
	return e.endLine - e.startLine + 1
}

var coverageLineRE = regexp.MustCompile(`^(.+):(\d+)\.(\d+),(\d+)\.(\d+)\s+(\d+)\s+(\d+)$`)

func main() {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <coverage-file> [output-file]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "example: %s coverage.out coverage.nocov\n", os.Args[0])
		fmt.Fprintf(
			os.Stderr,
			"If output-file is not provided, the filtered coverage will be written to <coverage-file>.nocov\n",
		)
		os.Exit(2)
	}

	inputPath := os.Args[1]
	outputPath := inputPath + ".nocov"
	if len(os.Args) == 3 {
		outputPath = os.Args[2]
	}

	moduleName, err := readModuleName("go.mod")
	if err != nil {
		die("read module name from go.mod", err)
	}

	lines, err := readLines(inputPath)
	if err != nil {
		die("read coverage profile", err)
	}
	if len(lines) == 0 {
		die("read coverage profile", errors.New("empty coverage file"))
	}

	mode := lines[0]
	if !strings.HasPrefix(mode, "mode:") {
		die("parse coverage profile", fmt.Errorf("first line must start with mode:, got %q", mode))
	}

	analyses := make(map[string]*fileAnalysis)
	ignoredByFile := make(map[string]int)
	totalIgnored := 0

	outLines := make([]string, 0, len(lines))
	outLines = append(outLines, mode)

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var entry *coverageEntry
		entry, err = parseCoverageEntry(lines[i], i+1)
		if err != nil {
			die("parse coverage profile", err)
		}

		if entry.execCount != 0 {
			outLines = append(outLines, lines[i])
			continue
		}

		var sourcePath, relPath string
		sourcePath, relPath, err = resolveSourcePath(moduleName, entry.filePath)
		if err != nil {
			die("resolve source path", fmt.Errorf("line %d: %w", entry.lineNumber, err))
		}

		analysis, found := analyses[relPath]
		if !found {
			analysis, err = buildAnalysis(sourcePath)
			if err != nil {
				die("analyze source file", fmt.Errorf("%s: %w", relPath, err))
			}
			analyses[relPath] = analysis
		}

		if shouldIgnore(entry, analysis) {
			ignoredByFile[relPath] += entry.Len()
			totalIgnored += entry.Len()
			continue
		}

		outLines = append(outLines, lines[i])
	}

	if err = writeLines(outputPath, outLines); err != nil {
		die("write filtered coverage profile", err)
	}

	_, err = fmt.Fprintf(os.Stdout, "wrote filtered coverage profile: %s\n", outputPath)
	check(err)
	printSummary(os.Stdout, ignoredByFile, totalIgnored)
}

func readModuleName(goModPath string) (string, error) {
	lines, err := readLines(goModPath)
	if err != nil {
		return "", err
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			module := strings.TrimSpace(strings.TrimPrefix(trimmed, "module "))
			if module == "" {
				return "", errors.New("empty module declaration")
			}
			return module, nil
		}
	}

	return "", errors.New("module declaration not found")
}

func parseCoverageEntry(line string, lineNumber int) (*coverageEntry, error) {
	matches := coverageLineRE.FindStringSubmatch(strings.TrimSpace(line))
	if matches == nil {
		return nil, fmt.Errorf("line %d: invalid coverage entry %q", lineNumber, line)
	}

	toInt := func(idx int) (int, error) {
		v, err := strconv.Atoi(matches[idx])
		if err != nil {
			return 0, fmt.Errorf("line %d: parse integer %q: %w", lineNumber, matches[idx], err)
		}
		return v, nil
	}

	startLine, err := toInt(2)
	if err != nil {
		return nil, err
	}
	startCol, err := toInt(3)
	if err != nil {
		return nil, err
	}
	endLine, err := toInt(4)
	if err != nil {
		return nil, err
	}
	endCol, err := toInt(5)
	if err != nil {
		return nil, err
	}
	numStmts, err := toInt(6)
	if err != nil {
		return nil, err
	}
	execCount, err := toInt(7)
	if err != nil {
		return nil, err
	}

	return &coverageEntry{
		filePath:   matches[1],
		startLine:  startLine,
		startCol:   startCol,
		endLine:    endLine,
		endCol:     endCol,
		numStmts:   numStmts,
		execCount:  execCount,
		original:   line,
		lineNumber: lineNumber,
	}, nil
}

func resolveSourcePath(moduleName, coveredPath string) (string, string, error) {
	relPath := coveredPath
	prefix := moduleName + "/"
	if after, ok := strings.CutPrefix(coveredPath, prefix); ok {
		relPath = after
	}

	relPath = filepath.Clean(relPath)

	if _, err := os.Stat(relPath); err == nil {
		return relPath, relPath, nil
	}

	return "", "", fmt.Errorf("cannot locate source file for %q (resolved as %q)", coveredPath, relPath)
}

func buildAnalysis(sourcePath string) (*fileAnalysis, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, sourcePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("while parsing source file %q: %w", sourcePath, err)
	}

	nocovRanges := []lineRange{}
	for _, group := range file.Comments {
		for _, c := range group.List {
			if !strings.HasPrefix(c.Text, "//") {
				continue
			}
			commentText := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			if !strings.HasPrefix(commentText, "nocov") {
				continue
			}
			pos := fset.Position(group.Pos())
			nocovRange := lineRange{
				start: pos.Line,
				end:   fset.Position(group.End()).Line,
				col:   pos.Column,
			}
			nocovRanges = append(nocovRanges, nocovRange)
		}
	}

	slog.Debug("found nocov ranges", "count", len(nocovRanges), "sourcePath", sourcePath, "nocovRanges", nocovRanges)

	e := &rangeExpander{
		fset:         fset,
		inlineRanges: &nocovRanges,
	}
	ast.Walk(e, file)

	slog.Debug(
		"expanded ranges",
		"count",
		len(e.expandedRanges),
		"sourcePath",
		sourcePath,
		"expandedRanges",
		e.expandedRanges,
	)

	allRanges := make([]lineRange, 0, len(nocovRanges)+len(e.expandedRanges))
	for i := 0; i < len(nocovRanges); i++ {
		r := &nocovRanges[i]
		hasBeenExpanded := false
		for _, e := range e.expandedRanges {
			if e.expandedFrom == r {
				allRanges = append(allRanges, e)
				hasBeenExpanded = true
				// Not sure if there can be multiple expansions from the same original range, but if so we want to
				// include them all, so we don't break here
				// break
			}
		}
		if !hasBeenExpanded {
			allRanges = append(allRanges, *r)
		}
	}

	slog.Debug("all ranges", "count", len(allRanges), "sourcePath", sourcePath, "allRanges", allRanges)

	return &fileAnalysis{
		nocovRanges: allRanges,
	}, nil
}

type rangeExpander struct {
	fset           *token.FileSet
	inlineRanges   *[]lineRange
	expandedRanges []lineRange
}

func (e *rangeExpander) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return e
	}

	nodeStartPos := e.fset.Position(node.Pos())
	nodeStartLine := nodeStartPos.Line
	nodeEndLine := e.fset.Position(node.End()).Line
	block, isBlock := node.(*ast.BlockStmt)

	var foundRange *lineRange
	for i := 0; i < len(*e.inlineRanges); i++ {
		r := &(*e.inlineRanges)[i]
		if r.end == nodeStartLine-1 && nodeStartPos.Column == r.col {
			slog.Debug("Nocov comment is before statement", "range", *r, "nodeStart", nodeStartPos)
			foundRange = r
			break
		}
		if isBlock && r.start == nodeStartLine && r.col > e.fset.Position(block.Lbrace).Column {
			slog.Debug("Nocov comment is after block start", "range", *r, "blockStart", e.fset.Position(block.Lbrace))
			foundRange = r
			break
		}
	}
	if foundRange == nil {
		return e
	}

	expandedRange := *foundRange
	expandedRange.expandedFrom = foundRange
	if expandedRange.end < nodeEndLine {
		expandedRange.end = nodeEndLine
	}

	slog.Debug(fmt.Sprintf("found range is %v for node %#v [%d;%d], expanded range is %v",
		*foundRange, node, nodeStartLine, nodeEndLine, expandedRange))
	e.expandedRanges = append(e.expandedRanges, expandedRange)

	return e
}

func shouldIgnore(entry *coverageEntry, analysis *fileAnalysis) bool {
	for _, r := range analysis.nocovRanges {
		if entry.startLine >= r.start && entry.endLine <= r.end {
			return true
		}
	}

	return false
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("while opening file %q: %w", path, err)
	}
	//nolint:errcheck // Intentionally ignore error on close since we're exiting immediately after
	defer f.Close()

	lines := make([]string, 0)
	s := bufio.NewScanner(f)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("while reading file %q: %w", path, err)
	}
	return lines, nil
}

func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o644) //nolint:wrapcheck // No Added Value
}

func printSummary(w io.Writer, ignoredByFile map[string]int, totalIgnored int) {
	if len(ignoredByFile) == 0 {
		_, err := fmt.Fprintln(w, "ignored lines: 0")
		check(err)
		return
	}

	keys := make([]string, 0, len(ignoredByFile))
	for k := range ignoredByFile {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	_, err := fmt.Fprintln(w, "ignored lines by file:")
	check(err)
	for _, k := range keys {
		_, err = fmt.Fprintf(w, "  %s: %d\n", k, ignoredByFile[k])
		check(err)
	}
	_, err = fmt.Fprintf(w, "total ignored lines: %d\n", totalIgnored)
	check(err)
}

func die(action string, err error) {
	fmt.Fprintf(os.Stderr, "error: %s: %v\n", action, err)
	os.Exit(1)
}

func check(err error) {
	if err != nil {
		die("fatal error", err)
	}
}
