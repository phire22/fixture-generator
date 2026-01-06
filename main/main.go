package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"

	"fixture-generator/pkg/generator"

	"golang.org/x/tools/go/packages"
)

func main() {
	pkgPath := flag.String("pkg", "", "path to the Go package to generate fixtures for")
	pkgName := flag.String("outpkg", "fixtures", "package name for the generated file")
	outFile := flag.String("out", "", "output file path (prints to stdout if not specified)")
	flag.Parse()

	if *pkgPath == "" {
		fmt.Fprintln(os.Stderr, "error: -pkg flag is required")
		os.Exit(1)
	}

	pkgs := load(*pkgPath)
	model := extract(pkgs)
	out, _ := generator.GenerateFormatted(model, *pkgName)

	// Format the output
	formatted, err := format.Source([]byte(out))
	if err != nil {
		formatted = []byte(out)
	}

	if *outFile != "" {
		err := os.WriteFile(*outFile, formatted, 0644)
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Print(string(formatted))
	}
}

func load(pattern string) []*packages.Package {
	absPath, err := filepath.Abs(pattern)
	if err != nil {
		panic(err)
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps | packages.NeedImports,
		Dir:  absPath,
	}

	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		panic(err)
	}
	if len(pkgs) == 0 {
		panic("no packages found")
	}

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				fmt.Fprintf(os.Stderr, "warning: %v\n", e)
			}
		}
	}
	return pkgs
}

func extract(pkgs []*packages.Package) *generator.Model {
	m := generator.NewModel()

	for _, pkg := range pkgs {
		extractEnums(pkg, m)
		extractOneOfs(pkg, m)
		extractStructs(pkg, m)
	}

	return m
}

func extractEnums(pkg *packages.Package, m *generator.Model) {
	for ident, obj := range pkg.TypesInfo.Defs {
		c, ok := obj.(*types.Const)
		if !ok {
			continue
		}
		if ident.Name == "_" || ident.Name == "EnforceVersion" {
			continue
		}
		named, ok := c.Type().(*types.Named)
		if !ok {
			continue
		}
		name := named.Obj().Name()
		e, ok := m.Enums[name]
		if !ok {
			e = &generator.Enum{Name: name}
			m.Enums[name] = e
		}
		e.Values = append(e.Values, ident.Name)
	}
}

func extractOneOfs(pkg *packages.Package, m *generator.Model) {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				name := ts.Name.Name

				if _, ok := ts.Type.(*ast.StructType); ok {
					for ifaceName := range m.OneOfs {
						if m.OneOfs[ifaceName] == "" {
							parentName := ifaceName[2:] // remove "is" prefix
							for i := len(parentName) - 1; i >= 0; i-- {
								if parentName[i] == '_' {
									prefix := parentName[:i]
									if len(name) > len(prefix) && name[:len(prefix)] == prefix && name[len(prefix)] == '_' {
										m.OneOfs[ifaceName] = name
										break
									}
								}
							}
						}
					}
				}

				if _, ok := ts.Type.(*ast.InterfaceType); ok {
					if len(name) > 2 && name[:2] == "is" {
						m.OneOfs[name] = ""
					}
				}
			}
		}
	}
}

func extractStructs(pkg *packages.Package, m *generator.Model) {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				s := &generator.Struct{Name: ts.Name.Name}
				for _, field := range st.Fields.List {
					tr := resolveType(pkg.TypesInfo.TypeOf(field.Type))
					for _, name := range field.Names {
						if generator.ProtoInternalFields[name.Name] {
							continue
						}
						s.Fields = append(s.Fields, generator.Field{
							Name: name.Name,
							Type: tr,
						})
					}
				}
				m.Structs[s.Name] = s
			}
		}
	}
}

func resolveType(t types.Type) generator.TypeRef {
	switch tt := t.(type) {
	case *types.Basic:
		return generator.TypeRef{Kind: "primitive", Name: tt.Name()}
	case *types.Named:
		name := tt.Obj().Name()
		// Use simple type name for external types lookup
		if _, ok := generator.ExternalTypes[name]; ok {
			return generator.TypeRef{Kind: "external", Name: name}
		}
		if _, ok := tt.Underlying().(*types.Struct); ok {
			return generator.TypeRef{Kind: "struct", Name: name}
		}
		if _, ok := tt.Underlying().(*types.Interface); ok {
			return generator.TypeRef{Kind: "oneof", Name: name}
		}
		return generator.TypeRef{Kind: "enum", Name: name}
	case *types.Pointer:
		elem := resolveType(tt.Elem())
		return generator.TypeRef{Kind: "pointer", Elem: &elem}
	case *types.Slice:
		elem := resolveType(tt.Elem())
		return generator.TypeRef{Kind: "slice", Elem: &elem}
	}
	return generator.TypeRef{Kind: "unknown"}
}
