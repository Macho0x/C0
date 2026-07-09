package deadlock

import (
	"goop.dev/compiler/internal/ast"
	"goop.dev/compiler/internal/token"
)

func parseChanSend(app *ast.AppExpr) (bool, string, token.SourceLoc) {
	for cur := app; cur != nil; {
		if inner, ok := cur.Func.(*ast.AppExpr); ok {
			if ch, loc, found := chanSendPartial(inner); found {
				return true, ch, loc
			}
		}
		if ch, loc, found := chanSendPartial(cur); found {
			return true, ch, loc
		}
		if next, ok := cur.Func.(*ast.AppExpr); ok {
			cur = next
			continue
		}
		break
	}
	return false, "", token.SourceLoc{}
}

func moduleName(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.IdentExpr:
		return e.Name, true
	case *ast.ConstructorExpr:
		return e.Name, true
	default:
		return "", false
	}
}

func chanSendPartial(app *ast.AppExpr) (ch string, loc token.SourceLoc, ok bool) {
	sel, isSel := app.Func.(*ast.FieldAccessExpr)
	if !isSel {
		return "", token.SourceLoc{}, false
	}
	mod, okMod := moduleName(sel.Left)
	if !okMod || mod != "Chan" || sel.Field != "send" {
		return "", token.SourceLoc{}, false
	}
	if id, okID := app.Arg.(*ast.IdentExpr); okID {
		return id.Name, app.Loc, true
	}
	return "", token.SourceLoc{}, false
}

func parseChanRecv(app *ast.AppExpr) (bool, string, token.SourceLoc) {
	for cur := app; cur != nil; {
		if ch, loc, found := chanRecvPartial(cur); found {
			return true, ch, loc
		}
		if next, ok := cur.Func.(*ast.AppExpr); ok {
			cur = next
			continue
		}
		break
	}
	return false, "", token.SourceLoc{}
}

func chanRecvPartial(app *ast.AppExpr) (ch string, loc token.SourceLoc, ok bool) {
	sel, isSel := app.Func.(*ast.FieldAccessExpr)
	if !isSel {
		return "", token.SourceLoc{}, false
	}
	mod, okMod := moduleName(sel.Left)
	if !okMod || mod != "Chan" || sel.Field != "recv" {
		return "", token.SourceLoc{}, false
	}
	if id, okID := app.Arg.(*ast.IdentExpr); okID {
		return id.Name, app.Loc, true
	}
	return "", token.SourceLoc{}, false
}
