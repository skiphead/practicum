// Package exitcheck contains an analyzer that checks for direct calls to os.Exit
// in the main function of the main package.
package exitcheck

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// Analyzer defines the static analysis tool that checks for prohibited os.Exit calls.
// It scans Go source files and reports violations when os.Exit is called directly
// within the main function of the main package.
var Analyzer = &analysis.Analyzer{
	Name: "exitcheck",
	Doc:  "prohibits direct calls to os.Exit in the main function of the main package",
	Run:  run,
}

// run is the main analysis function that examines Go source files.
// It traverses the AST of each file and identifies violations of the os.Exit rule.
//
// Parameters:
//   - pass: The analysis pass containing file information and reporting capabilities
//
// Returns:
//   - interface{}: Analysis result (nil in this case)
//   - error: Any error encountered during analysis
//
// Algorithm:
// 1. Filters files to only those in the "main" package
// 2. Traverses AST to find function declarations named "main"
// 3. Inspects the body of main functions for call expressions
// 4. Identifies calls to os.Exit and reports them as violations
func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		// Check that the file belongs to the main package
		if file.Name.Name != "main" {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			// Look for the main function
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "main" {
				return true
			}

			// Check the body of the main function
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}

				// Check if the call is to os.Exit
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}

				// If it's os.Exit, report an error
				if ident.Name == "os" && sel.Sel.Name == "Exit" {
					pass.Reportf(call.Pos(), "direct call to os.Exit in the main function is prohibited")
				}

				return true
			})

			return false
		})
	}
	return nil, nil
}
