package main

import (
	"bytes"
	"context"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/gqlgo/gqlgenc/config"
	"github.com/gqlgo/gqlgenc/generator"

	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
	"github.com/vektah/gqlparser/v2/parser"

	"github.com/theopenlane/entx/genhooks"
)

const (
	graphapiGenDir = "./"
	schemaDir      = "../schema/schema.graphql"
	queryDir       = "../query/"
)

// query data for template
type query struct {
	// Name of the type
	Name string
	// Fields to include in the query
	Fields []string
	// IncludeMutations to include mutation (create, update, delete) queries
	IncludeMutations bool
	// IsHistory to indicate if the type is a history type
	IsHistory bool
}

type mutation struct {
	Name          string
	OperationName string
	InputArgs     []arg
	PayloadField  string
	IsDelete      bool
}

type arg struct {
	Name string
	Type string
}

var (
	softDeleteFields = []string{"deleted_at", "deleted_by"}
)

func main() {

	if err := GenQuery(); err != nil {
		log.Fatal().Err(err).Msg("Failed to generate queries")
	}

	cfg, err := config.LoadConfig(graphapiGenDir + ".gqlgenc.yml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
		os.Exit(2)
	}

	if err := generator.Generate(context.Background(), cfg); err != nil {
		log.Error().Err(err).Msg("Failed to generate gqlgenc client")
	}
}

// GenQuery generates query files for each node in the schema
func GenQuery() error {

	content, err := os.ReadFile(schemaDir)
	if err != nil {
		return err
	}

	schema, err := gqlparser.LoadSchema(
		&ast.Source{Name: "schema.graphql", Input: string(content)},
	)
	if err != nil {
		return err
	}

	tmpl := genhooks.CreateQuery()

	for _, node := range getClientNodes(schema) {
		generateQuery(node, tmpl, schema)
	}

	return nil
}

// generateQuery generates or updates a query file for the given node
func generateQuery(node *ast.Definition, tmpl *template.Template, schema *ast.Schema) {
	queryFilePath := getFileName(queryDir, node.Name)

	if _, err := os.Stat(queryFilePath); err == nil {
		// file exists -> update!!
		return
	}
	print("\nGenerating query for ", node.Name, "\nFields: ")

	makeQueryFile(schema, node, tmpl, queryFilePath)
}

// allowedMutationNames returns a map of the allowed mutation names
func allowedMutationNames(schema *ast.Schema, nodeName string) map[string]bool {
	allowed := map[string]bool{}

	for _, mutation := range schema.Mutation.Fields {
		if isAMutationFromNode(mutation.Name, nodeName) {
			allowed[strings.ToLower(mutation.Name)] = true
		}
	}
	return allowed
}

// makeQueryFile generates a query file for the given node
func makeQueryFile(schema *ast.Schema, node *ast.Definition, tmpl *template.Template, filePath string) error {
	var buf bytes.Buffer

	s := query{
		Name:             node.Name,
		Fields:           getFieldNames(schema, node.Fields),
		IncludeMutations: true, // let template generate all mutations
	}

	if err := tmpl.Execute(&buf, s); err != nil {
		return err
	}

	doc, err := parser.ParseQuery(&ast.Source{
		Name:  filePath,
		Input: buf.String(),
	})
	if err != nil {
		return err
	}

	allowedMutations := allowedMutationNames(schema, node.Name)

	print("\nAllowed mutations for ", node.Name, ": ")
	for name := range allowedMutations {
		println(name, " ")
	}

	filtered := &ast.QueryDocument{}

	for _, op := range doc.Operations {
		if op.Operation != ast.Mutation {
			filtered.Operations = append(filtered.Operations, op)
			continue
		}

		mutationName := op.Name
		println("Checking mutation:", mutationName, "for root node:", node.Name)
		if allowedMutations[strings.ToLower(mutationName)] {
			println("mutation accepted:", mutationName)
			filtered.Operations = append(filtered.Operations, op)
		}
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	println("filtered operation count:", len(filtered.Operations))

	for _, op := range filtered.Operations {
		println("kept:", op.Operation, op.Name)
	}

	formatter.NewFormatter(file).FormatQueryDocument(filtered)

	return nil
}

// isAMutationFromNode checks if the mutation name is related to the given node name
func isAMutationFromNode(mutationName, nodeName string) bool {
	stringsToExclude := []string{
		"bulk",
		"csv",
		"create",
		"update",
		"delete",
	}

	nodeName = strings.ToLower(nodeName)
	mutationName = strings.ToLower(mutationName)

	transformedName := mutationName

	for _, prefix := range stringsToExclude {
		transformedName = strings.ReplaceAll(transformedName, prefix, "")
	}

	return transformedName == nodeName
}

// getFieldNames returns a list of field names from a list of fields in alphabetical order
func getFieldNames(schema *ast.Schema, fields ast.FieldList) []string {
	fieldNames := []string{"id"}

	for _, f := range fields {
		if f.Name == "id" {
			continue
		}

		if isSoftDeleteField(f.Name) {
			continue
		}

		print(f.Name, ", ")

		fieldNames = append(fieldNames, f.Name)
	}

	sort.Strings(fieldNames)
	return fieldNames
}

// checkASTMutation checks if the node has any mutations
func checkASTMutation(schema *ast.Schema, node *ast.Definition) bool {
	// for _, mutation := range schema.Mutation.Fields {
	// To do.....
	// }

	return false
}

// unwrapType returns the lowest level named type
func unwrapType(t *ast.Type) string {
	if t.NamedType != "" {
		return t.NamedType
	}

	/*
		[name!]!

		Type
		    Nonnull
		    Elem
				list
				elem
					nonnull
					elem
						the actual type name
	*/

	return unwrapType(t.Elem)
}

// getClientNodes returns all object definitions that implement node
func getClientNodes(schema *ast.Schema) []*ast.Definition {
	var nodes []*ast.Definition

	for _, def := range schema.Types {
		if def.Kind != ast.Object {
			continue
		}

		// Check if definition implements node
		if implementsInterface(def, "Node") {
			nodes = append(nodes, def)
		}
	}

	return nodes
}

func implementsInterface(def *ast.Definition, name string) bool {
	for _, iface := range def.Interfaces {
		if iface == name {
			return true
		}
	}

	return false
}

func getFileName(dir, name string) string {
	return filepath.Clean(filepath.Join(dir, strings.ToLower(name)+".graphql"))
}

func isSoftDeleteField(fieldName string) bool {
	return lo.Contains(softDeleteFields, fieldName)
}

// getFirstWord returns the first work of the string before the separator
func GetFirstWord(name string) string {
	words := strings.FieldsFunc(name, isSeparator)
	return words[0]
}

// isSeparator checks if character is a underscore, hyphen or a space
func isSeparator(r rune) bool {
	return r == '_' || r == '-' || unicode.IsSpace(r)
}

// lowerSubstring lowers the a substring from s, where the portion lowored starts from the first character
// and lowers wordLen characters in s
func lowerSubstring(s string, wordLen int) string {
	return strings.ToLower(s[:wordLen]) + s[wordLen:]
}
