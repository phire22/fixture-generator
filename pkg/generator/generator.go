package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
)

// Model holds all extracted type information
type Model struct {
	Structs  map[string]*Struct
	Enums    map[string]*Enum
	TypeDefs map[string]*TypeDef
	OneOfs   map[string]string // interface name -> first implementation name
}

// NewModel creates an empty Model
func NewModel() *Model {
	return &Model{
		Structs:  make(map[string]*Struct),
		Enums:    make(map[string]*Enum),
		TypeDefs: make(map[string]*TypeDef),
		OneOfs:   make(map[string]string),
	}
}

// Struct represents a Go struct type
type Struct struct {
	Name   string
	Fields []Field
}

// Field represents a struct field
type Field struct {
	Name string
	Type TypeRef
}

// Enum represents a Go enum type (constants of the same type)
type Enum struct {
	Name   string
	Values []string
}

// TypeDef represents a type alias like `type TenantID string`
type TypeDef struct {
	Name       string
	Underlying TypeRef
}

// TypeRef represents a type reference
type TypeRef struct {
	Kind string // "primitive", "struct", "enum", "oneof", "pointer", "slice", "external", "typedef", "unknown"
	Name string
	Elem *TypeRef
}

// ProtoInternalFields are protobuf-generated fields to skip
var ProtoInternalFields = map[string]bool{
	"state":          true,
	"unknownFields":  true,
	"sizeCache":      true,
	"EnforceVersion": true,
}

// ExternalType defines an external type with its import and default value
type ExternalType struct {
	Import string
	Value  string
}

// ExternalTypes maps type names to their import and default value
var ExternalTypes = map[string]ExternalType{
	"Timestamp": {
		Import: `timestamppb "google.golang.org/protobuf/types/known/timestamppb"`,
		Value:  "timestamppb.New(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))",
	},
	"Time": {
		Import: `"time"`,
		Value:  "time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)",
	},
}

// RequiredImports are always included when external types are used
var RequiredImports = []string{
	`"time"`,
}

// ParseSource parses Go source code and extracts type information into a Model
func ParseSource(source string) (*Model, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "input.go", source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	m := NewModel()

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			name := typeSpec.Name.Name

			// Skip unexported types
			if name[0] >= 'a' && name[0] <= 'z' {
				continue
			}

			switch t := typeSpec.Type.(type) {
			case *ast.StructType:
				s := &Struct{Name: name}

				for _, field := range t.Fields.List {
					if len(field.Names) == 0 {
						continue
					}

					fieldName := field.Names[0].Name

					if ProtoInternalFields[fieldName] {
						continue
					}

					// Skip unexported fields
					if fieldName[0] >= 'a' && fieldName[0] <= 'z' {
						continue
					}

					typeRef := exprToTypeRef(field.Type)
					s.Fields = append(s.Fields, Field{Name: fieldName, Type: typeRef})
				}

				if len(s.Fields) > 0 {
					m.Structs[s.Name] = s
				}

			case *ast.Ident:
				// Type alias like `type TenantID string`
				underlying := exprToTypeRef(t)
				if underlying.Kind == "primitive" {
					m.TypeDefs[name] = &TypeDef{
						Name:       name,
						Underlying: underlying,
					}
				}
			}
		}
	}

	return m, nil
}

func exprToTypeRef(expr ast.Expr) TypeRef {
	switch t := expr.(type) {
	case *ast.Ident:
		name := t.Name
		switch name {
		case "string", "bool",
			"int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64",
			"float32", "float64", "byte", "rune":
			return TypeRef{Kind: "primitive", Name: name}
		}
		if _, ok := ExternalTypes[name]; ok {
			return TypeRef{Kind: "external", Name: name}
		}
		return TypeRef{Kind: "struct", Name: name}

	case *ast.StarExpr:
		elem := exprToTypeRef(t.X)
		return TypeRef{Kind: "pointer", Elem: &elem}

	case *ast.ArrayType:
		elem := exprToTypeRef(t.Elt)
		return TypeRef{Kind: "slice", Elem: &elem, Name: elem.Name}

	case *ast.SelectorExpr:
		typeName := t.Sel.Name
		if _, ok := ExternalTypes[typeName]; ok {
			return TypeRef{Kind: "external", Name: typeName}
		}
		return TypeRef{Kind: "struct", Name: typeName}

	default:
		return TypeRef{Kind: "unknown"}
	}
}

// Generate produces fixture functions from the model
func Generate(m *Model, pkgName string) string {
	var b bytes.Buffer
	b.WriteString("package " + pkgName + "\n\n")

	imports := collectImports(m)
	if len(imports) > 0 {
		b.WriteString("import (\n")
		for _, imp := range imports {
			fmt.Fprintf(&b, "\t%s\n", imp)
		}
		b.WriteString(")\n\n")
	}

	b.WriteString("func ptr[T any](v T) *T { return &v }\n\n")

	// Generate typedef fixtures
	for _, td := range m.TypeDefs {
		fmt.Fprintf(&b, "func Fixture%s() %s {\n", td.Name, td.Name)
		fmt.Fprintf(&b, "\treturn %s(%s)\n", td.Name, genPrimitiveValue(td.Underlying.Name, td.Name, td.Name))
		fmt.Fprintf(&b, "}\n\n")
	}

	// Generate enum fixtures
	for _, e := range m.Enums {
		var firstValue string
		for _, v := range e.Values {
			if v != "_" && v != "EnforceVersion" {
				firstValue = v
				break
			}
		}
		if firstValue == "" {
			continue
		}
		fmt.Fprintf(&b, "func Fixture%s() %s {\n", e.Name, e.Name)
		fmt.Fprintf(&b, "\treturn %s\n", firstValue)
		fmt.Fprintf(&b, "}\n\n")
	}

	// Generate struct fixtures
	for _, s := range m.Structs {
		fmt.Fprintf(&b, "func Fixture%s() %s {\n", s.Name, s.Name)
		fmt.Fprintf(&b, "\treturn %s{\n", s.Name)
		for _, f := range s.Fields {
			fmt.Fprintf(&b, "\t\t%s: %s,\n", f.Name, GenValue(m, f.Type, f.Name, s.Name))
		}
		fmt.Fprintf(&b, "\t}\n")
		fmt.Fprintf(&b, "}\n\n")
	}

	return b.String()
}

// GenerateFormatted produces formatted fixture functions
func GenerateFormatted(m *Model, pkgName string) (string, error) {
	out := Generate(m, pkgName)
	formatted, err := format.Source([]byte(out))
	if err != nil {
		return out, nil
	}
	return string(formatted), nil
}

// GenValue generates a default value for a type
func GenValue(m *Model, t TypeRef, fieldName string, structName string) string {
	switch t.Kind {
	case "primitive":
		return genPrimitiveValue(t.Name, fieldName, structName)
	case "struct":
		// Check if it's actually a typedef
		if _, ok := m.TypeDefs[t.Name]; ok {
			return "Fixture" + t.Name + "()"
		}
		return "Fixture" + t.Name + "()"
	case "enum":
		return "Fixture" + t.Name + "()"
	case "typedef":
		return "Fixture" + t.Name + "()"
	case "oneof":
		if impl, ok := m.OneOfs[t.Name]; ok && impl != "" {
			return "&" + impl + "{}"
		}
		return "nil"
	case "slice":
		if t.Elem == nil {
			return "nil"
		}
		return "[]" + TypeName(*t.Elem) + "{" + GenValue(m, *t.Elem, fieldName, structName) + "}"
	case "pointer":
		if t.Elem == nil || t.Elem.Kind == "unknown" {
			return "nil"
		}
		if t.Elem.Kind == "external" {
			if ext, ok := ExternalTypes[t.Elem.Name]; ok {
				return ext.Value
			}
		}
		return "ptr(" + GenValue(m, *t.Elem, fieldName, structName) + ")"
	case "external":
		if ext, ok := ExternalTypes[t.Name]; ok {
			return ext.Value
		}
		return "nil"
	}
	return "nil"
}

func genPrimitiveValue(typeName, fieldName, structName string) string {
	switch typeName {
	case "string":
		if fieldName == "ID" || fieldName == "Id" {
			return fmt.Sprintf(`"%sID"`, structName)
		}
		return fmt.Sprintf(`"%s"`, fieldName)
	case "bool":
		return "true"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "byte", "rune":
		return "1"
	default:
		return "nil"
	}
}

// TypeName returns the Go type name for a TypeRef
func TypeName(t TypeRef) string {
	switch t.Kind {
	case "pointer":
		if t.Elem != nil {
			return "*" + TypeName(*t.Elem)
		}
	case "slice":
		if t.Elem != nil {
			return "[]" + TypeName(*t.Elem)
		}
	}
	if t.Name != "" {
		return t.Name
	}
	if t.Elem != nil {
		return TypeName(*t.Elem)
	}
	return "interface{}"
}

func collectImports(m *Model) []string {
	usedExternals := make(map[string]bool)

	for _, s := range m.Structs {
		for _, f := range s.Fields {
			collectExternalTypes(f.Type, usedExternals)
		}
	}

	if len(usedExternals) == 0 {
		return nil
	}

	importSet := make(map[string]bool)
	for _, imp := range RequiredImports {
		importSet[imp] = true
	}
	for extName := range usedExternals {
		if ext, ok := ExternalTypes[extName]; ok {
			importSet[ext.Import] = true
		}
	}

	imports := make([]string, 0, len(importSet))
	for imp := range importSet {
		imports = append(imports, imp)
	}
	return imports
}

func collectExternalTypes(t TypeRef, used map[string]bool) {
	if t.Kind == "external" {
		used[t.Name] = true
	}
	if t.Elem != nil {
		collectExternalTypes(*t.Elem, used)
	}
}
