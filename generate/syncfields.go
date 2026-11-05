//go:build ignore

// syncfields adds the flat fields selected by core's query files to matching operations in query/
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
)

const (
	coreModule    = "github.com/theopenlane/core/v2"
	coreQueryPath = "internal/graphapi/query"
	queryDir      = "query"
	schemaFile    = "schema/schema.graphql"
	excludeFile   = "generate/syncfields.exclude"
)

// errNoCoreDir is returned when the module download does not report a directory
var errNoCoreDir = errors.New("go mod download returned no directory for core")

// syncer merges core's flat fields into client selections
type syncer struct {
	schema   *ast.Schema
	excluded []string
	added    int
	removed  int
}

func main() {
	version := flag.String("version", "latest", "version of the core module to read queries from")
	dryRun := flag.Bool("dry-run", false, "report the fields without changing any files")

	flag.Parse()

	if err := run(*version, *dryRun); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(version string, dryRun bool) error {
	coreDir, resolved, err := downloadCore(version)
	if err != nil {
		return err
	}

	excluded, err := loadExcludes()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(schemaFile)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}

	schema, err := gqlparser.LoadSchema(&ast.Source{Name: schemaFile, Input: string(data)})
	if err != nil {
		return fmt.Errorf("load schema: %w", err)
	}

	coreDocs, err := loadDocuments(schema, filepath.Join(coreDir, coreQueryPath))
	if err != nil {
		return err
	}

	localDocs, err := loadDocuments(schema, queryDir)
	if err != nil {
		return err
	}

	coreOps := operationsByName(coreDocs)
	s := &syncer{schema: schema, excluded: excluded}

	for _, file := range slices.Sorted(maps.Keys(localDocs)) {
		doc := localDocs[file]
		before := s.added + s.removed

		for _, op := range doc.Operations {
			s.pruneOperation(op)

			if core, ok := coreOps[op.Name]; ok {
				op.SelectionSet = s.merge(core.SelectionSet, op.SelectionSet, true)
			}
		}

		if s.added+s.removed > before && !dryRun {
			if err := writeDocument(file, doc); err != nil {
				return err
			}
		}
	}

	fmt.Printf("core %s: %d fields to add, %d to remove\n", resolved, s.added, s.removed)

	return nil
}

// downloadCore fetches the core module through the go proxy and returns its directory and version
func downloadCore(version string) (string, string, error) {
	// go mod download -json writes its error to stdout, so it has to be parsed on failure too
	out, downloadErr := exec.Command("go", "mod", "download", "-json", coreModule+"@"+version).Output()

	var module struct {
		Dir     string
		Version string
		Error   string
	}

	if err := json.Unmarshal(out, &module); err != nil {
		return "", "", fmt.Errorf("parse core download: %w", err)
	}

	if module.Error != "" {
		return "", "", fmt.Errorf("download core: %s", module.Error)
	}

	if downloadErr != nil {
		return "", "", fmt.Errorf("download core: %w", downloadErr)
	}

	if module.Dir == "" {
		return "", "", errNoCoreDir
	}

	return module.Dir, module.Version, nil
}

// loadExcludes reads field name patterns, one per line, ignoring blanks and # comments
func loadExcludes() ([]string, error) {
	data, err := os.ReadFile(excludeFile)
	if err != nil {
		return nil, fmt.Errorf("read excludes: %w", err)
	}

	patterns := []string{}

	for line := range strings.Lines(string(data)) {
		if pattern := strings.TrimSpace(strings.SplitN(line, "#", 2)[0]); pattern != "" {
			patterns = append(patterns, pattern)
		}
	}

	return patterns, nil
}

// loadDocuments parses the query files in dir, the walk resolves each field against the schema
func loadDocuments(schema *ast.Schema, dir string) (map[string]*ast.QueryDocument, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.graphql"))
	if err != nil {
		return nil, err
	}

	docs := map[string]*ast.QueryDocument{}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}

		doc, err := parser.ParseQuery(&ast.Source{Name: file, Input: string(data)})
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", file, err)
		}

		validator.Walk(schema, doc, &validator.Events{})

		docs[file] = doc
	}

	return docs, nil
}

func operationsByName(docs map[string]*ast.QueryDocument) map[string]*ast.OperationDefinition {
	ops := map[string]*ast.OperationDefinition{}

	for _, doc := range docs {
		for _, op := range doc.Operations {
			ops[op.Name] = op
		}
	}

	return ops
}

// pruneOperation removes fields the schema no longer has, top level fields are reported and left in place
func (s *syncer) pruneOperation(op *ast.OperationDefinition) {
	for _, selection := range op.SelectionSet {
		field, ok := selection.(*ast.Field)
		if !ok {
			continue
		}

		if field.Definition == nil {
			fmt.Printf("%s: top level field %s is not in the schema, remove it by hand\n", op.Name, field.Name)

			continue
		}

		kept, gone := prune(op.Name+": "+field.Name, field.SelectionSet)
		if len(gone) > 0 && len(kept) == 0 {
			fmt.Printf("%s: no fields of %s are left in the schema, remove it by hand\n", op.Name, field.Name)

			continue
		}

		field.SelectionSet = kept
		s.removed += len(gone)

		for _, name := range gone {
			fmt.Println("remove", name)
		}
	}
}

// prune returns the selections that exist in the schema and the names of those that do not
// a field left with nothing selected is dropped as well, since an empty selection is invalid
func prune(parent string, selections ast.SelectionSet) (ast.SelectionSet, []string) {
	kept := ast.SelectionSet{}
	gone := []string{}

	for _, selection := range selections {
		field, ok := selection.(*ast.Field)
		if !ok {
			kept = append(kept, selection)

			continue
		}

		name := parent + "." + field.Name

		if field.Definition == nil {
			gone = append(gone, name)

			continue
		}

		if len(field.SelectionSet) == 0 {
			kept = append(kept, field)

			continue
		}

		children, removed := prune(name, field.SelectionSet)
		gone = append(gone, removed...)

		if len(children) > 0 {
			field.SelectionSet = children
			kept = append(kept, field)
		}
	}

	return kept, gone
}

// merge adds core's missing flat fields, descending through wrappers to the entity and never into edges
func (s *syncer) merge(core, client ast.SelectionSet, wrapper bool) ast.SelectionSet {
	existing := fieldsByName(client)
	merged := client
	insertAt := 0

	for _, selection := range core {
		field, ok := selection.(*ast.Field)
		if !ok || field.Definition == nil {
			continue
		}

		typeName := field.Definition.Type.Name()
		leaf := s.schema.Types[typeName].IsLeafType()

		if match, ok := existing[field.Name]; ok {
			if leaf {
				insertAt = slices.Index(merged, ast.Selection(match)) + 1
			} else if wrapper {
				match.SelectionSet = s.merge(field.SelectionSet, match.SelectionSet, isWrapper(typeName))
			}

			continue
		}

		if !leaf || wrapper || s.isExcluded(field.Name) {
			continue
		}

		merged = slices.Insert(merged, insertAt, ast.Selection(&ast.Field{Alias: field.Name, Name: field.Name}))
		insertAt++
		s.added++
	}

	return merged
}

func (s *syncer) isExcluded(name string) bool {
	for _, pattern := range s.excluded {
		if matched, _ := path.Match(pattern, name); matched {
			return true
		}
	}

	return false
}

func fieldsByName(selections ast.SelectionSet) map[string]*ast.Field {
	fields := map[string]*ast.Field{}

	for _, selection := range selections {
		if field, ok := selection.(*ast.Field); ok {
			fields[field.Name] = field
		}
	}

	return fields
}

func isWrapper(typeName string) bool {
	return strings.HasSuffix(typeName, "Payload") || strings.HasSuffix(typeName, "Connection") || strings.HasSuffix(typeName, "Edge")
}

func writeDocument(file string, doc *ast.QueryDocument) error {
	var out bytes.Buffer

	formatter.NewFormatter(&out).FormatQueryDocument(doc)

	return os.WriteFile(file, out.Bytes(), 0o644)
}
