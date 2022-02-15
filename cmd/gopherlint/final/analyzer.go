package final

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/gopherd/tools/cmd/gopherlint/util"
)

const directive = "@mod:final"

const Doc = `check for final variables that reassigned.`

var Analyzer = &analysis.Analyzer{
	Name:      "final",
	Doc:       Doc,
	Requires:  []*analysis.Analyzer{inspect.Analyzer},
	FactTypes: []analysis.Fact{new(finalDeclFact)},
	Run:       run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	localFinals := make(map[types.Object]*finalDeclFact)

	inspect.Preorder([]ast.Node{
		(*ast.GenDecl)(nil),
		(*ast.ValueSpec)(nil),
	}, func(n ast.Node) {
		switch x := n.(type) {
		case *ast.GenDecl:
			if x.Tok != token.VAR {
				return
			}
			var groupFinalPos = getFinalDirectivePos(pass, x, x.Doc)
			var groupFinalPosition token.Position
			if groupFinalPos.IsValid() {
				groupFinalPosition = pass.Fset.Position(groupFinalPos)
			}
			for _, spec := range x.Specs {
				var finalPos = groupFinalPos
				var position = groupFinalPosition
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				} else if !finalPos.IsValid() {
					finalPos = getFinalDirectivePos(pass, valueSpec, valueSpec.Doc)
					position = pass.Fset.Position(finalPos)
				}
				if finalPos.IsValid() {
					exportFinals(pass, localFinals, valueSpec.Names, position)
				}
			}
		case *ast.ValueSpec:
			if finalPos := getFinalDirectivePos(pass, x, x.Doc); finalPos.IsValid() {
				exportFinals(pass, localFinals, x.Names, pass.Fset.Position(finalPos))
			}
		}
	})

	inspect.Preorder([]ast.Node{(*ast.AssignStmt)(nil)}, func(n ast.Node) {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return
		}
		for _, expr := range assign.Lhs {
			checkFinalVar(pass, localFinals, expr)
		}
	})
	return nil, nil
}

func checkFinalVar(pass *analysis.Pass, finals map[types.Object]*finalDeclFact, expr ast.Expr) {
	expr = util.Unparen(expr)
	var ident *ast.Ident
	switch x := expr.(type) {
	case *ast.Ident:
		ident = x
	case *ast.SelectorExpr:
		ident = x.Sel
	}
	if ident == nil {
		return
	}
	obj := pass.TypesInfo.ObjectOf(ident)
	if obj != nil {
		pos, ok := isFinalObj(pass, finals, obj)
		if ok {
			pass.Reportf(expr.Pos(), "assign value to a final variable %s (directive %q declared here %s)", ident.Name, directive, pos.String())
		}
	}
}

func exportFinals(pass *analysis.Pass, finals map[types.Object]*finalDeclFact, names []*ast.Ident, position token.Position) {
	position.Column = 0 // ignore column
	for _, v := range names {
		if v.Name == "_" {
			continue
		}
		obj := pass.TypesInfo.ObjectOf(v)
		if obj == nil {
			continue
		}
		fact := &finalDeclFact{position: position}
		finals[obj] = fact
		pass.ExportObjectFact(obj, fact)
	}
}

func isFinalObj(pass *analysis.Pass, finals map[types.Object]*finalDeclFact, obj types.Object) (pos token.Position, ok bool) {
	if fact, ok := finals[obj]; ok {
		return fact.position, true
	}
	var fact = new(finalDeclFact)
	ok = pass.ImportObjectFact(obj, fact)
	if ok {
		pos = fact.position
	}
	return
}

type finalDeclFact struct {
	position token.Position
}

func (finalDeclFact) AFact()         {}
func (finalDeclFact) String() string { return "finalDeclFact" }

func getFinalDirectivePos(pass *analysis.Pass, node ast.Node, doc *ast.CommentGroup) token.Pos {
	if doc == nil {
		return token.NoPos
	}
	for _, comment := range doc.List {
		if comment.Text == "//"+directive || strings.HasPrefix(comment.Text, "//"+directive+" ") {
			return comment.Pos()
		}
	}
	return token.NoPos
}
