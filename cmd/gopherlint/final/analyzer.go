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

var flags struct {
	verbose int
}

func init() {
	Analyzer.Flags.IntVar(&flags.verbose, "verbose", 0, "final analyzer verbose level")
}

var Analyzer = &analysis.Analyzer{
	Name:      "final",
	Doc:       `check for final variables that reassigned or referenced.`,
	Requires:  []*analysis.Analyzer{inspect.Analyzer},
	FactTypes: []analysis.Fact{new(finalDeclFact)},
	Run:       run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	localFinals := make(map[types.Object]*finalDeclFact)

	inspect.Preorder([]ast.Node{
		(*ast.GenDecl)(nil),
	}, func(n ast.Node) {
		switch x := n.(type) {
		case *ast.GenDecl:
			if x.Tok != token.VAR {
				return
			}
			var groupFinalPos = getFinalDirectivePos(pass, x.Doc)
			var groupFinalPosition token.Position
			if groupFinalPos.IsValid() {
				groupFinalPosition = getFileAndLine(pass, groupFinalPos)
			}
			for _, spec := range x.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				var finalPos = groupFinalPos
				var position = groupFinalPosition
				if !finalPos.IsValid() {
					finalPos = getFinalDirectivePos(pass, valueSpec.Doc)
					position = getFileAndLine(pass, finalPos)
				}
				if finalPos.IsValid() {
					exportFinalObjects(pass, localFinals, valueSpec.Names, position)
				}
			}
		}
	})

	inspect.Preorder([]ast.Node{
		(*ast.AssignStmt)(nil),
		(*ast.UnaryExpr)(nil),
		(*ast.ExprStmt)(nil),
	}, func(n ast.Node) {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, expr := range x.Lhs {
				checkFinalObject(pass, localFinals, expr, token.ASSIGN, false)
			}
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				checkFinalObject(pass, localFinals, x.X, x.Op, false)
			}
		case *ast.ExprStmt:
			call, ok := util.Unparen(n.(*ast.ExprStmt).X).(*ast.CallExpr)
			if !ok {
				return // not a call statement
			}
			fun := util.Unparen(call.Fun)
			if pass.TypesInfo.Types[fun].IsType() {
				return // a conversion, not a call
			}
			fn, signature, isMethod := util.GetFunc(pass, fun)
			if fn == nil || signature == nil || !isMethod || signature.Recv().Anonymous() {
				return
			}
			recvName := signature.Recv().Name()
			if recvName == "" || recvName == "_" {
				return
			}
			if _, isPointer := signature.Recv().Type().(*types.Pointer); !isPointer {
				return
			}
			selector := fun.(*ast.SelectorExpr)
			checkFinalObject(pass, localFinals, selector.X, token.AND, true)
		}
	})
	return nil, nil
}

func getFileAndLine(pass *analysis.Pass, pos token.Pos) token.Position {
	position := pass.Fset.Position(pos)
	position.Column = 0
	return position
}

func checkFinalObject(pass *analysis.Pass, finals map[types.Object]*finalDeclFact, expr ast.Expr, op token.Token, ignorePointer bool) {
	expr = util.Unparen(expr)
	var pos = expr.Pos()
	var ident *ast.Ident
	var position token.Position
	var ok bool
	var field bool
	for expr != nil {
		switch x := expr.(type) {
		case *ast.Ident:
			ident = x
			expr = nil
		case *ast.SelectorExpr:
			ident = x.Sel
			expr = x.X
		}
		if ident == nil {
			return
		}
		obj := pass.TypesInfo.ObjectOf(ident)
		if obj == nil {
			return
		}
		if ignorePointer {
			if _, isPointer := obj.Type().(*types.Pointer); isPointer {
				return
			}
		}
		position, ok = lookupFinalObject(pass, finals, obj)
		if ok {
			break
		}
		field = true
	}
	if !ok {
		return
	}
	var fieldprefix string
	if field {
		fieldprefix = "field of "
	}
	switch op {
	case token.ASSIGN:
		if flags.verbose > 0 {
			pass.Reportf(
				pos,
				"cannot assign a value to %sfinal variable %s (directive %q declared here %s)",
				fieldprefix, ident.Name, directive, position.String(),
			)
		} else {
			pass.Reportf(
				pos,
				"cannot assign a value to %sfinal variable %s",
				fieldprefix, ident.Name,
			)
		}
	case token.AND:
		if flags.verbose > 0 {
			pass.Reportf(
				pos,
				"cannot reference %sfinal variable %s (directive %q declared here %s)",
				fieldprefix, ident.Name, directive, position.String(),
			)
		} else {
			pass.Reportf(
				pos,
				"cannot reference %sfinal variable %s",
				fieldprefix, ident.Name,
			)
		}
	}
}

func exportFinalObjects(pass *analysis.Pass, finals map[types.Object]*finalDeclFact, names []*ast.Ident, position token.Position) {
	position.Column = 0 // ignore column
	for _, v := range names {
		if v.Name == "_" || v.Name == "" {
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

func lookupFinalObject(pass *analysis.Pass, finals map[types.Object]*finalDeclFact, obj types.Object) (pos token.Position, ok bool) {
	if obj == nil {
		return
	}
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

func getFinalDirectivePos(pass *analysis.Pass, doc *ast.CommentGroup) token.Pos {
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
