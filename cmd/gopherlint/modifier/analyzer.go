package modifier

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const modifierDirective = "//@mod:"

type Modifier struct {
	Directive string         // prefix "@mod:" has been removed
	Position  token.Position // position of directive
}

type ModifierFact struct {
	Modifiers []Modifier
}

func (ModifierFact) AFact()         {}
func (ModifierFact) String() string { return "ModifierFact" }

var Analyzer = &analysis.Analyzer{
	Name:      "modifier",
	Doc:       `lookup modifiers`,
	Requires:  []*analysis.Analyzer{inspect.Analyzer},
	FactTypes: []analysis.Fact{new(ModifierFact)},
	Run:       run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	inspect.Preorder([]ast.Node{
		(*ast.File)(nil),
		(*ast.Field)(nil),
		(*ast.ImportSpec)(nil),
		(*ast.ValueSpec)(nil),
		(*ast.TypeSpec)(nil),
		(*ast.GenDecl)(nil),
		(*ast.FuncDecl)(nil),
	}, func(n ast.Node) {
		switch x := n.(type) {
		case *ast.File:
			lookupAndExportModifiers(pass, x.Doc, x.Name)
		case *ast.Field:
			lookupAndExportModifiers(pass, x.Doc, x.Names...)
		case *ast.ImportSpec:
			lookupAndExportModifiers(pass, x.Doc, x.Name)
		case *ast.ValueSpec:
			lookupAndExportModifiers(pass, x.Doc, x.Names...)
		case *ast.TypeSpec:
			lookupAndExportModifiers(pass, x.Doc, x.Name)
		case *ast.GenDecl:
			var modifiers = lookupModifiers(pass, x.Doc)
			for _, spec := range x.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				exportModifiers(
					pass,
					mergeModifiers(lookupModifiers(pass, valueSpec.Doc), modifiers),
					valueSpec.Names...,
				)
			}
		case *ast.FuncDecl:
			lookupAndExportModifiers(pass, x.Doc, x.Name)
		}
	})
	return nil, nil
}

func getPosition(pass *analysis.Pass, pos token.Pos) token.Position {
	position := pass.Fset.Position(pos)
	position.Column = 0
	return position
}

func lookupModifiers(pass *analysis.Pass, doc *ast.CommentGroup) []Modifier {
	if doc == nil {
		return nil
	}
	var modifiers []Modifier
	for _, comment := range doc.List {
		if strings.HasPrefix(comment.Text, modifierDirective) {
			modifiers = append(modifiers, Modifier{
				Directive: strings.TrimPrefix(comment.Text, modifierDirective),
				Position:  getPosition(pass, comment.Pos()),
			})
		}
	}
	return modifiers
}

func mergeModifiers(dst, src []Modifier) []Modifier {
	return append(dst, src...)
}

func exportModifiers(pass *analysis.Pass, modifiers []Modifier, names ...*ast.Ident) {
	if len(modifiers) == 0 || len(names) == 0 {
		return
	}
	for _, v := range names {
		if v.Name == "_" || v.Name == "" {
			continue
		}
		obj := pass.TypesInfo.ObjectOf(v)
		if obj == nil {
			continue
		}
		fact := &ModifierFact{
			Modifiers: modifiers,
		}
		println("export modifier", len(modifiers))
		pass.ExportObjectFact(obj, fact)
	}
}

func lookupAndExportModifiers(pass *analysis.Pass, doc *ast.CommentGroup, names ...*ast.Ident) {
	modifiers := lookupModifiers(pass, doc)
	if len(modifiers) == 0 {
		return
	}
	exportModifiers(pass, modifiers, names...)
}
