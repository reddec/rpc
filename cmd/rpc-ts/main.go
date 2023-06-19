package main

import (
	_ "embed"
	"flag"
	"github.com/reddec/rpc/internal/compile"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

//go:embed ts.gotemplate
var templateText string

func main() {
	lineNum, err := strconv.Atoi(os.Getenv("GOLINE"))
	if err != nil {
		panic("GOLINE env incorrect")
	}
	fileName, err := filepath.Abs(os.Getenv("GOFILE"))
	if err != nil {
		panic(err)
	}
	packageName := os.Getenv("GOPACKAGE")

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedTypes | packages.NeedImports | packages.NeedName | packages.NeedSyntax,
	})
	if err != nil {
		panic(err)
	}
	// find correct package
	var pkg *packages.Package
	for _, p := range pkgs {
		if p.Name == packageName {
			pkg = p
			break
		}
	}
	if pkg == nil {
		panic("unknown package " + packageName)
	}

	scope := pkg.Types.Scope()
	var typeName string
	for _, name := range scope.Names() {
		tp := scope.Lookup(name)
		pos := pkg.Fset.Position(tp.Pos())
		if pos.Filename == fileName && pos.Line == lineNum+1 {
			typeName = name
			break
		}
	}
	if typeName == "" {
		panic("directive should be on top of struct declaration")
	}

	output := flag.String("out", strings.ToLower(typeName)+".ts", "Output file")
	shim := flag.String("shim", "", "Comma-separated list of TS types shim (ex: github.com/jackc/pgtype.JSONB:any")
	flag.Parse()

	obj := scope.Lookup(typeName)
	if obj == nil {
		panic("typename not found")
	}
	base := obj.Type().(*types.Named)

	tpl := getTemplate()
	var tl = compile.New()

	for _, opt := range strings.Split(*shim, ",") {
		sourceType, tsType, ok := strings.Cut(opt, ":")
		if !ok {
			continue
		}
		tl.Custom(sourceType, compile.TSVar{Type: tsType})
	}

	tl.CommentLookup(func(pos token.Pos) string {
		rp := pkg.Fset.Position(pos)
		prevLine := pkg.Fset.File(pos).Pos(rp.Offset - rp.Column - 1)
		for _, s := range pkg.Syntax {
			for _, g := range s.Comments {
				if prevLine >= g.Pos() && prevLine <= g.End() {
					return strings.TrimSpace(g.Text())
				}
			}
		}
		return ""
	})
	api := tl.ScanAPI(base)

	vc := viewContext{
		API:     api,
		Objects: tl.Objects(),
		Aliases: tl.Aliases(),
	}

	// save
	if err := os.MkdirAll(filepath.Dir(*output), 0755); err != nil {
		panic(err)
	}
	f, err := os.Create(*output)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err := tpl.Execute(f, &vc); err != nil {
		panic(err)
	}
}

type viewContext struct {
	API     compile.API
	Objects map[string][]compile.Param
	Aliases map[string]compile.Type
}

func getTemplate() *template.Template {
	return template.Must(template.New("").Funcs(map[string]any{
		"join": func(sep string, list []string) string { return strings.Join(list, sep) },
		"comment": func(ident int, text string) string {
			if text == "" {
				return ""
			}
			var ans []string
			for _, line := range strings.Split(text, "\n") {
				ans = append(ans, "// "+line)
			}
			if len(ans) == 0 {
				return ""
			}
			return strings.Join(ans, "\n"+strings.Repeat(" ", ident))
		},
		"lower": strings.ToLower,
	}).Delims("[[", "]]").Parse(templateText))
}
