// Package main provides a code generation tool for automatically generating Reset() methods
// for Go structs. It scans Go packages for structs annotated with the "generate:reset"
// comment and generates reset.gen.go files with Reset() implementations.
//
// The generator handles various types including:
// - Basic types (int, string, bool, etc.)
// - Pointers to structs (with or without Reset() method)
// - Slices and maps
// - Nested structs
// - Custom types implementing the Resetter interface
//
// Usage:
//
//	go run main.go [flags] [directory]
//
// Flags:
//
//	--force          Force regeneration even if file exists
//	-v               Verbose output
//	--skip-generated Skip generated files (_test.go, reset.gen.go)
//
// Example:
//
//	// Code generation is triggered by adding this comment to a struct:
//	// generate:reset
//	type User struct {
//	    ID   int
//	    Name string
//	    Tags []string
//	}
//
// Generated output:
//
//	func (u *User) Reset() {
//	    if u == nil {
//	        return
//	    }
//	    u.ID = 0
//	    u.Name = ""
//	    u.Tags = u.Tags[:0]
//	}
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Config holds the generator configuration.
type Config struct {
	Force         bool
	Verbose       bool
	SkipGenerated bool
}

// StructInfo represents information about a struct that needs a Reset() method.
type StructInfo struct {
	Name   string
	Fields []FieldInfo
}

// FieldInfo represents information about a struct field.
type FieldInfo struct {
	Name     string
	Type     string
	IsPtr    bool
	IsSlice  bool
	IsMap    bool
	IsStruct bool
	HasReset bool
	BaseType string
}

// TypeRegistry maintains information about types in a package.
type TypeRegistry struct {
	structTypes  map[string]bool   // type name -> is struct
	hasResetFunc map[string]bool   // type name -> has Reset() method
	baseTypes    map[string]string // full type -> base type name
}

var config Config

func main() {
	var verbose bool
	var skipGenerated bool

	flag.BoolVar(&config.Force, "force", false, "Force regeneration even if file exists")
	flag.BoolVar(&verbose, "v", false, "Verbose output")
	flag.BoolVar(&skipGenerated, "skip-generated", true, "Skip generated files (_test.go, reset.gen.go)")
	flag.Parse()

	config.Verbose = verbose
	config.SkipGenerated = skipGenerated

	rootDir := "."
	if len(flag.Args()) > 0 {
		rootDir = flag.Args()[0]
	}

	fmt.Printf("Starting reset generator in: %s\n", rootDir)
	if config.Force {
		fmt.Println("Force mode: ON - will regenerate existing files")
	}
	if config.Verbose {
		fmt.Println("Verbose mode: ON")
	}

	if err := walkPackages(rootDir); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Done!")
}

// walkPackages recursively walks through directories and processes Go packages.
func walkPackages(rootDir string) error {
	var processedCount int
	var generatedCount int

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Warning: error accessing %s: %v\n", path, err)
			return nil
		}

		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
			if config.Verbose {
				fmt.Printf("Skipping hidden directory: %s\n", path)
			}
			return filepath.SkipDir
		}

		if !info.IsDir() {
			return nil
		}

		files, err := os.ReadDir(path)
		if err != nil {
			fmt.Printf("Warning: error reading directory %s: %v\n", path, err)
			return nil
		}

		hasGoFiles := false
		for _, file := range files {
			name := file.Name()
			if strings.HasSuffix(name, ".go") {
				if config.SkipGenerated {
					if strings.HasSuffix(name, "_test.go") || name == "reset.gen.go" {
						continue
					}
				}
				hasGoFiles = true
				break
			}
		}

		if !hasGoFiles {
			if config.Verbose {
				fmt.Printf("Skipping directory without .go files: %s\n", path)
			}
			return nil
		}

		processedCount++
		if config.Verbose {
			fmt.Printf("Processing package: %s\n", path)
		}

		generated, err := processPackage(path)
		if err != nil {
			fmt.Printf("Error processing package %s: %v\n", path, err)
			return nil
		}

		if generated {
			generatedCount++
		}

		return nil
	})

	fmt.Printf("\nStatistics:\n")
	fmt.Printf("  Packages processed: %d\n", processedCount)
	fmt.Printf("  Files generated: %d\n", generatedCount)

	return err
}

// processPackage processes a single Go package directory.
// Returns true if a reset.gen.go file was generated.
func processPackage(pkgPath string) (bool, error) {
	fset := token.NewFileSet()

	// Read all Go files in the directory
	files, err := os.ReadDir(pkgPath)
	if err != nil {
		return false, fmt.Errorf("error reading directory %s: %w", pkgPath, err)
	}

	// Parse each Go file individually
	var parsedFiles []*ast.File
	var pkgName string

	for _, file := range files {
		name := file.Name()

		// Skip files based on configuration
		if config.SkipGenerated {
			if strings.HasSuffix(name, "_test.go") || name == "reset.gen.go" {
				continue
			}
		}

		if !strings.HasSuffix(name, ".go") {
			continue
		}

		filename := filepath.Join(pkgPath, name)
		src, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if err != nil {
			if config.Verbose {
				fmt.Printf("  Warning: could not parse %s: %v\n", filename, err)
			}
			continue
		}

		if pkgName == "" {
			pkgName = src.Name.Name
		} else if src.Name.Name != pkgName {
			if config.Verbose {
				fmt.Printf("  Warning: file %s has package %s, expected %s\n",
					filename, src.Name.Name, pkgName)
			}
			continue
		}

		parsedFiles = append(parsedFiles, src)
	}

	if len(parsedFiles) == 0 {
		if config.Verbose {
			fmt.Printf("  No valid Go files found in %s\n", pkgPath)
		}
		return false, nil
	}

	typeRegistry := &TypeRegistry{
		structTypes:  make(map[string]bool),
		hasResetFunc: make(map[string]bool),
		baseTypes:    make(map[string]string),
	}

	// Collect type information from all files
	for _, file := range parsedFiles {
		collectTypeInfoFromFile(file, typeRegistry, fset)
	}

	var structs []StructInfo

	// Look for structs with // generate:reset comment
	for _, file := range parsedFiles {
		ast.Inspect(file, func(n ast.Node) bool {
			genDecl, ok := n.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				return true
			}

			if genDecl.Doc == nil {
				return true
			}

			hasResetComment := false
			for _, comment := range genDecl.Doc.List {
				if strings.Contains(comment.Text, "generate:reset") {
					hasResetComment = true
					break
				}
			}

			if !hasResetComment {
				return true
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				structInfo := StructInfo{
					Name: typeSpec.Name.Name,
				}

				for _, field := range structType.Fields.List {
					if len(field.Names) == 0 {
						continue
					}

					typeStr := getTypeString(field.Type)
					baseType := resolveBaseType(field.Type, typeRegistry)
					isStruct := typeRegistry.structTypes[baseType]
					hasReset := typeRegistry.hasResetFunc[baseType]

					for _, name := range field.Names {
						typeInfo := analyzeType(field.Type)
						fieldInfo := FieldInfo{
							Name:     name.Name,
							Type:     typeStr,
							IsPtr:    typeInfo.isPtr,
							IsSlice:  typeInfo.isSlice,
							IsMap:    typeInfo.isMap,
							IsStruct: isStruct,
							HasReset: hasReset,
							BaseType: baseType,
						}
						structInfo.Fields = append(structInfo.Fields, fieldInfo)
					}
				}

				structs = append(structs, structInfo)
				if config.Verbose {
					fmt.Printf("  Found struct with generate:reset: %s\n", structInfo.Name)
				}
			}

			return true
		})
	}

	if len(structs) > 0 {
		if err = generateResetFile(pkgPath, pkgName, structs, typeRegistry); err != nil {
			return false, err
		}
		return true, nil
	}

	if config.Verbose && len(structs) == 0 {
		fmt.Printf("  No structs with generate:reset found in %s\n", pkgPath)
	}

	return false, nil
}

// collectTypeInfoFromFile collects information about types and methods from a single file.
func collectTypeInfoFromFile(file *ast.File, registry *TypeRegistry, fset *token.FileSet) {
	// Collect struct types
	ast.Inspect(file, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
					registry.structTypes[typeSpec.Name.Name] = true
					registry.baseTypes[typeSpec.Name.Name] = typeSpec.Name.Name
				}
			}
		}
		return true
	})

	// Collect Reset methods
	ast.Inspect(file, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || funcDecl.Name == nil {
			return true
		}

		if funcDecl.Name.Name != "Reset" {
			return true
		}

		for _, recv := range funcDecl.Recv.List {
			receiverType := extractReceiverType(recv.Type)
			if receiverType != "" {
				registry.hasResetFunc[receiverType] = true
			}
		}
		return true
	})
}

// typeAnalysis holds information about a Go type.
type typeAnalysis struct {
	isPtr    bool
	isSlice  bool
	isMap    bool
	isArray  bool
	elemType ast.Expr
}

// analyzeType analyzes a Go type expression.
func analyzeType(expr ast.Expr) typeAnalysis {
	var result typeAnalysis

	switch t := expr.(type) {
	case *ast.StarExpr:
		result.isPtr = true
		result.elemType = t.X
	case *ast.ArrayType:
		if t.Len == nil {
			result.isSlice = true
		} else {
			result.isArray = true
		}
		result.elemType = t.Elt
	case *ast.MapType:
		result.isMap = true
		result.elemType = t.Value
	default:
		result.elemType = expr
	}

	return result
}

// extractReceiverType extracts the receiver type from a method receiver.
func extractReceiverType(expr ast.Expr) string {
	return resolveBaseType(expr, &TypeRegistry{})
}

// resolveBaseType resolves a type expression to its base type name.
func resolveBaseType(expr ast.Expr, registry *TypeRegistry) string {
	if registry != nil {
		typeStr := getTypeString(expr)
		if cached, ok := registry.baseTypes[typeStr]; ok {
			return cached
		}
	}

	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return resolveBaseType(t.X, registry)
	case *ast.ArrayType:
		return resolveBaseType(t.Elt, registry)
	case *ast.MapType:
		return resolveBaseType(t.Value, registry)
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.IndexExpr:
		return resolveBaseType(t.X, registry)
	case *ast.IndexListExpr:
		return resolveBaseType(t.X, registry)
	default:
		return ""
	}
}

// getTypeString converts a type expression to its string representation.
func getTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + getTypeString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + getTypeString(t.Elt)
		}
		return "[" + getTypeString(t.Len) + "]" + getTypeString(t.Elt)
	case *ast.MapType:
		return "map[" + getTypeString(t.Key) + "]" + getTypeString(t.Value)
	case *ast.SelectorExpr:
		return getTypeString(t.X) + "." + t.Sel.Name
	case *ast.BasicLit:
		return t.Value
	case *ast.IndexExpr:
		return getTypeString(t.X) + "[" + getTypeString(t.Index) + "]"
	default:
		return fmt.Sprintf("%T", t)
	}
}

// generateResetFile generates a reset.gen.go file for a package.
func generateResetFile(pkgPath, pkgName string, structs []StructInfo, registry *TypeRegistry) error {
	outputPath := filepath.Join(pkgPath, "reset.gen.go")

	if _, err := os.Stat(outputPath); err == nil && !config.Force {
		fmt.Printf("  File already exists, skipping (use --force to regenerate): %s\n", outputPath)
		return nil
	}

	tmpl := template.Must(template.New("reset").Parse(`// Code generated by reset generator. DO NOT EDIT.
package {{.PackageName}}

// Resetter is an interface for types that can be reset to their zero state.
type Resetter interface {
	Reset()
}

{{range .Structs}}
func ({{.Receiver}} *{{.Name}}) Reset() {
	if {{.Receiver}} == nil {
		return
	}
	{{range .Fields}}
	{{.ResetCode}}
	{{end}}
}
{{end}}
`))

	type TemplateField struct {
		ResetCode string
	}

	type TemplateStruct struct {
		Name     string
		Receiver string
		Fields   []TemplateField
	}

	type TemplateData struct {
		PackageName string
		Structs     []TemplateStruct
	}

	data := TemplateData{
		PackageName: pkgName,
	}

	for _, s := range structs {
		tmplStruct := TemplateStruct{
			Name:     s.Name,
			Receiver: getReceiverName(s.Name),
		}

		for _, f := range s.Fields {
			field := TemplateField{}
			field.ResetCode = generateResetCode(tmplStruct.Receiver, f, registry)
			tmplStruct.Fields = append(tmplStruct.Fields, field)
		}

		data.Structs = append(data.Structs, tmplStruct)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	formatted, err := format.Source([]byte(buf.String()))
	if err != nil {
		fmt.Printf("Warning: could not format code for %s: %v\n", outputPath, err)
		formatted = []byte(buf.String())
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", outputPath, err)
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(file)

	if _, err = file.Write(formatted); err != nil {
		return fmt.Errorf("error writing to file %s: %w", outputPath, err)
	}

	fmt.Printf("  Generated: %s\n", outputPath)
	return nil
}

// getReceiverName generates a receiver name for a type.
func getReceiverName(typeName string) string {
	if len(typeName) == 0 {
		return "x"
	}

	firstChar := strings.ToLower(string(typeName[0]))
	if len(typeName) == 1 || !isValidGoIdentifier(firstChar) {
		return "x"
	}
	return firstChar
}

// isValidGoIdentifier checks if a string is a valid Go identifier.
func isValidGoIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}

	firstChar := s[0]
	if !((firstChar >= 'a' && firstChar <= 'z') || (firstChar >= 'A' && firstChar <= 'Z') || firstChar == '_') {
		return false
	}

	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// generateResetCode generates reset code for a field.
func generateResetCode(receiver string, field FieldInfo, registry *TypeRegistry) string {
	if field.Name == "" {
		return ""
	}

	fieldAccess := receiver + "." + field.Name

	switch {
	case field.IsSlice:
		return fieldAccess + " = " + fieldAccess + "[:0]"

	case field.IsMap:
		return "clear(" + fieldAccess + ")"

	case field.IsPtr:
		if field.HasReset {
			return `if ` + fieldAccess + ` != nil {
		` + fieldAccess + `.Reset()
	}`
		} else if field.IsStruct {
			return `if ` + fieldAccess + ` != nil {
		*` + fieldAccess + ` = ` + field.BaseType + `{}
	}`
		} else {
			return fieldAccess + ` = nil`
		}

	default:
		if field.HasReset {
			return `if r, ok := interface{}(` + fieldAccess + `).(Resetter); ok {
		r.Reset()
	}`
		} else if field.IsStruct {
			return `).(Resetter); ok {
		r.Reset()
	} else {
		` + `if r, ok := interface{}(` + fieldAccess + fieldAccess + ` = ` + field.BaseType + `{}
	}`
		} else {
			return fieldAccess + ` = ` + getZeroValue(field.Type)
		}
	}
}

// getZeroValue returns the zero value for a Go type.
func getZeroValue(typeName string) string {
	baseType := typeName
	for strings.HasPrefix(baseType, "*") || strings.HasPrefix(baseType, "[]") ||
		strings.HasPrefix(baseType, "map[") || strings.HasPrefix(baseType, "[") {
		if strings.HasPrefix(baseType, "*") {
			baseType = baseType[1:]
		} else if strings.HasPrefix(baseType, "[]") {
			baseType = baseType[2:]
		} else if strings.HasPrefix(baseType, "map[") {
			bracketCount := 1
			for i := 4; i < len(baseType); i++ {
				if baseType[i] == '[' {
					bracketCount++
				} else if baseType[i] == ']' {
					bracketCount--
					if bracketCount == 0 {
						baseType = baseType[i+1:]
						break
					}
				}
			}
		} else if strings.HasPrefix(baseType, "[") {
			for i := 1; i < len(baseType); i++ {
				if baseType[i] == ']' {
					baseType = baseType[i+1:]
					break
				}
			}
		}
	}

	switch baseType {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"uintptr", "float32", "float64", "complex64", "complex128":
		return "0"
	case "string":
		return `""`
	case "bool":
		return "false"
	case "byte", "rune":
		return "0"
	case "error":
		return "nil"
	default:
		if strings.HasPrefix(typeName, "*") ||
			strings.HasPrefix(typeName, "[]") ||
			strings.HasPrefix(typeName, "map[") ||
			strings.HasPrefix(typeName, "chan") ||
			strings.HasPrefix(typeName, "func(") ||
			strings.HasPrefix(typeName, "interface") {
			return "nil"
		}
		return typeName + "{}"
	}
}
