package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
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

	// First pass: find oneof interfaces
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

			// Look for oneof interfaces (start with "is") first - these can be lowercase
			if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
				if len(name) > 2 && name[:2] == "is" {
					m.OneOfs[name] = ""
					continue // Don't skip oneof interfaces
				}
			}

			// Skip unexported types (except oneof interfaces handled above)
			if name[0] >= 'a' && name[0] <= 'z' {
				continue
			}
		}
	}

	// Second pass: find struct implementations and build model
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

				// Check if this struct implements a oneof interface
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

			case *ast.Ident:
				// Type alias like `type TenantID string`
				underlying := exprToTypeRef(t)
				if underlying.Kind == "primitive" {
					m.TypeDefs[name] = &TypeDef{
						Name:       name,
						Underlying: underlying,
					}
				}

			case *ast.InterfaceType:
				// Already handled in first pass
				continue
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

// GenerateOptions holds optional configuration for code generation
type GenerateOptions struct {
	// TypePrefix is prepended to type names (e.g., "productionorderbase" -> "productionorderbase.Operation")
	TypePrefix string
	// FuncPrefix is inserted into fixture function names (e.g., "PB" -> "FixturePBOperation")
	FuncPrefix string
	// ModStyle generates fixtures with functional options pattern (default: true)
	ModStyle bool
}

// Generate produces fixture functions from the model
func Generate(m *Model, pkgName string) string {
	return GenerateWithOptions(m, pkgName, GenerateOptions{ModStyle: true})
}

// GenerateWithOptions produces fixture functions from the model with optional prefixes
func GenerateWithOptions(m *Model, pkgName string, opts GenerateOptions) string {
	var b bytes.Buffer
	b.WriteString("package " + pkgName + "\n\n")

	imports := collectImports(m, opts.TypePrefix)
	if len(imports) > 0 {
		b.WriteString("import (\n")
		for _, imp := range imports {
			fmt.Fprintf(&b, "\t%s\n", imp)
		}
		b.WriteString(")\n\n")
	}

	b.WriteString("func ptr[T any](v T) *T { return &v }\n\n")

	// Helper to prefix type names
	prefixType := func(name string) string {
		if opts.TypePrefix != "" {
			return opts.TypePrefix + "." + name
		}
		return name
	}

	// Generate typedef fixtures
	for _, td := range m.TypeDefs {
		if opts.ModStyle {
			fmt.Fprintf(&b, "func Fixture%s%s(mods ...func(*%s)) *%s {\n", opts.FuncPrefix, td.Name, prefixType(td.Name), prefixType(td.Name))
			value := fmt.Sprintf("%s(%s)", prefixType(td.Name), genPrimitiveValue(td.Underlying.Name, td.Name, td.Name))
			fmt.Fprintf(&b, "\tresult := &%s\n", value)
			fmt.Fprintf(&b, "\tfor _, mod := range mods {\n")
			fmt.Fprintf(&b, "\t\tmod(result)\n")
			fmt.Fprintf(&b, "\t}\n")
			fmt.Fprintf(&b, "\treturn result\n")
		} else {
			fmt.Fprintf(&b, "func Fixture%s%s() %s {\n", opts.FuncPrefix, td.Name, prefixType(td.Name))
			fmt.Fprintf(&b, "\treturn %s(%s)\n", prefixType(td.Name), genPrimitiveValue(td.Underlying.Name, td.Name, td.Name))
		}
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
		if opts.ModStyle {
			fmt.Fprintf(&b, "func Fixture%s%s(mods ...func(*%s)) *%s {\n", opts.FuncPrefix, e.Name, prefixType(e.Name), prefixType(e.Name))
			fmt.Fprintf(&b, "\tvalue := %s\n", prefixType(firstValue))
			fmt.Fprintf(&b, "\tfor _, mod := range mods {\n")
			fmt.Fprintf(&b, "\t\tmod(&value)\n")
			fmt.Fprintf(&b, "\t}\n")
			fmt.Fprintf(&b, "\treturn &value\n")
		} else {
			fmt.Fprintf(&b, "func Fixture%s%s() %s {\n", opts.FuncPrefix, e.Name, prefixType(e.Name))
			fmt.Fprintf(&b, "\treturn %s\n", prefixType(firstValue))
		}
		fmt.Fprintf(&b, "}\n\n")
	}

	// Generate struct fixtures
	for _, s := range m.Structs {
		if opts.ModStyle {
			fmt.Fprintf(&b, "func Fixture%s%s(mods ...func(*%s)) *%s {\n", opts.FuncPrefix, s.Name, prefixType(s.Name), prefixType(s.Name))
			fmt.Fprintf(&b, "\tvalue := &%s{\n", prefixType(s.Name))
			for _, f := range s.Fields {
				fmt.Fprintf(&b, "\t\t%s: %s,\n", f.Name, genValue(m, f.Type, f.Name, s.Name, opts))
			}
			fmt.Fprintf(&b, "\t}\n")
			fmt.Fprintf(&b, "\tfor _, mod := range mods {\n")
			fmt.Fprintf(&b, "\t\tmod(value)\n")
			fmt.Fprintf(&b, "\t}\n")
			fmt.Fprintf(&b, "\treturn value\n")
		} else {
			fmt.Fprintf(&b, "func Fixture%s%s() %s {\n", opts.FuncPrefix, s.Name, prefixType(s.Name))
			fmt.Fprintf(&b, "\treturn %s{\n", prefixType(s.Name))
			for _, f := range s.Fields {
				fmt.Fprintf(&b, "\t\t%s: %s,\n", f.Name, genValue(m, f.Type, f.Name, s.Name, opts))
			}
			fmt.Fprintf(&b, "\t}\n")
		}
		fmt.Fprintf(&b, "}\n\n")
	}

	return b.String()
}

// GenerateFormatted produces formatted fixture functions
func GenerateFormatted(m *Model, pkgName string) (string, error) {
	return GenerateFormattedWithOptions(m, pkgName, GenerateOptions{ModStyle: true})
}

// GenerateFormattedWithOptions produces formatted fixture functions with optional prefixes
func GenerateFormattedWithOptions(m *Model, pkgName string, opts GenerateOptions) (string, error) {
	out := GenerateWithOptions(m, pkgName, opts)
	formatted, err := format.Source([]byte(out))
	if err != nil {
		return out, nil
	}
	return string(formatted), nil
}

// GenValue generates a default value for a type (without prefix support, for backward compatibility)
func GenValue(m *Model, t TypeRef, fieldName string, structName string) string {
	return genValue(m, t, fieldName, structName, GenerateOptions{ModStyle: true})
}

// genValue generates a default value for a type with optional prefix support
func genValue(m *Model, t TypeRef, fieldName string, structName string, opts GenerateOptions) string {
	prefixType := func(name string) string {
		if opts.TypePrefix != "" {
			return opts.TypePrefix + "." + name
		}
		return name
	}

	switch t.Kind {
	case "primitive":
		return genPrimitiveValue(t.Name, fieldName, structName)
	case "struct":
		// Check if this is actually a oneof interface (starts with "is")
		if len(t.Name) > 2 && t.Name[:2] == "is" {
			// This is a oneof interface, find the first implementation
			if impl, ok := m.OneOfs[t.Name]; ok && impl != "" {
				// Check if we have the implementation struct in our model
				if implStruct, exists := m.Structs[impl]; exists {
					// Generate populated struct with default values
					var structFields []string
					for _, field := range implStruct.Fields {
						fieldValue := genValue(m, field.Type, field.Name, impl, opts)
						structFields = append(structFields, fmt.Sprintf("%s: %s", field.Name, fieldValue))
					}
					if len(structFields) > 0 {
						return fmt.Sprintf("&%s{\n\t\t\t%s,\n\t\t}", prefixType(impl), strings.Join(structFields, ",\n\t\t\t"))
					}
				}
				// Fallback to empty struct if no fields found
				return "&" + prefixType(impl) + "{}"
			}
			return "nil"
		}

		// Check if it's actually a typedef
		if _, ok := m.TypeDefs[t.Name]; ok {
			if opts.ModStyle {
				return "*Fixture" + opts.FuncPrefix + t.Name + "()"
			}
			return "Fixture" + opts.FuncPrefix + t.Name + "()"
		}
		if opts.ModStyle {
			return "*Fixture" + opts.FuncPrefix + t.Name + "()"
		}
		return "Fixture" + opts.FuncPrefix + t.Name + "()"
	case "enum":
		if opts.ModStyle {
			return "*Fixture" + opts.FuncPrefix + t.Name + "()"
		}
		return "Fixture" + opts.FuncPrefix + t.Name + "()"
	case "typedef":
		if opts.ModStyle {
			return "*Fixture" + opts.FuncPrefix + t.Name + "()"
		}
		return "Fixture" + opts.FuncPrefix + t.Name + "()"
	case "oneof":
		if impl, ok := m.OneOfs[t.Name]; ok && impl != "" {
			// Check if we have the implementation struct in our model
			if implStruct, exists := m.Structs[impl]; exists {
				// Generate populated struct with default values
				var structFields []string
				for _, field := range implStruct.Fields {
					fieldValue := genValue(m, field.Type, field.Name, impl, opts)
					structFields = append(structFields, fmt.Sprintf("%s: %s", field.Name, fieldValue))
				}
				if len(structFields) > 0 {
					return fmt.Sprintf("&%s{\n\t\t\t%s,\n\t\t}", prefixType(impl), strings.Join(structFields, ",\n\t\t\t"))
				}
			}
			// Fallback to empty struct if no fields found
			return "&" + prefixType(impl) + "{}"
		}
		return "nil"
	case "slice":
		if t.Elem == nil {
			return "nil"
		}
		return "[]" + typeName(*t.Elem, opts) + "{" + genValue(m, *t.Elem, fieldName, structName, opts) + "}"
	case "pointer":
		if t.Elem == nil || t.Elem.Kind == "unknown" {
			return "nil"
		}
		if t.Elem.Kind == "external" {
			if ext, ok := ExternalTypes[t.Elem.Name]; ok {
				return ext.Value
			}
		}
		if opts.ModStyle && (t.Elem.Kind == "struct" || t.Elem.Kind == "enum" || t.Elem.Kind == "typedef") {
			return genValue(m, *t.Elem, fieldName, structName, opts)
		}

		return "ptr(" + genValue(m, *t.Elem, fieldName, structName, opts) + ")"
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

// TypeName returns the Go type name for a TypeRef (without prefix support, for backward compatibility)
func TypeName(t TypeRef) string {
	return typeName(t, GenerateOptions{})
}

// typeName returns the Go type name for a TypeRef with optional prefix support
func typeName(t TypeRef, opts GenerateOptions) string {
	prefixType := func(name string) string {
		if opts.TypePrefix != "" {
			return opts.TypePrefix + "." + name
		}
		return name
	}

	switch t.Kind {
	case "pointer":
		if t.Elem != nil {
			return "*" + typeName(*t.Elem, opts)
		}
	case "slice":
		if t.Elem != nil {
			return "[]" + typeName(*t.Elem, opts)
		}
	case "struct", "enum", "typedef":
		if t.Name != "" {
			return prefixType(t.Name)
		}
	}
	if t.Name != "" {
		return t.Name
	}
	if t.Elem != nil {
		return typeName(*t.Elem, opts)
	}
	return "interface{}"
}

func collectImports(m *Model, typePrefix string) []string {
	usedExternals := make(map[string]bool)

	for _, s := range m.Structs {
		for _, f := range s.Fields {
			collectExternalTypes(f.Type, usedExternals)
		}
	}

	// If no external types and no type prefix, no imports needed
	if len(usedExternals) == 0 && typePrefix == "" {
		return nil
	}

	importSet := make(map[string]bool)

	// Add type prefix import if specified
	if typePrefix != "" {
		// The typePrefix is expected to be a package alias or short name
		// The user should provide the full import path via a separate flag if needed
		// For now, we assume the typePrefix is already importable or in the same module
	}

	if len(usedExternals) > 0 {
		for _, imp := range RequiredImports {
			importSet[imp] = true
		}
		for extName := range usedExternals {
			if ext, ok := ExternalTypes[extName]; ok {
				importSet[ext.Import] = true
			}
		}
	}

	if len(importSet) == 0 {
		return nil
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
