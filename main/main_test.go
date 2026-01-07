package main

import (
	"strings"
	"testing"

	"fixture-generator/pkg/generator"
)

func TestGenValue(t *testing.T) {
	emptyModel := &generator.Model{
		Structs: map[string]*generator.Struct{},
		Enums:   map[string]*generator.Enum{},
		OneOfs:  map[string]string{},
	}

	oneofModel := &generator.Model{
		Structs: map[string]*generator.Struct{
			"UserReference_EmailId": {
				Name: "UserReference_EmailId",
				Fields: []generator.Field{
					{Name: "EmailId", Type: generator.TypeRef{Kind: "primitive", Name: "string"}},
				},
			},
		},
		Enums: map[string]*generator.Enum{},
		OneOfs: map[string]string{
			"isUserReference_Id": "UserReference_EmailId",
		},
	}

	tests := []struct {
		name       string
		model      *generator.Model
		typeRef    generator.TypeRef
		fieldName  string
		structName string
		want       string
	}{
		{
			name:       "basic string field",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "primitive", Name: "string"},
			fieldName:  "FirstName",
			structName: "User",
			want:       `"FirstName"`,
		},
		{
			name:       "basic int field",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "primitive", Name: "int"},
			fieldName:  "Age",
			structName: "User",
			want:       "1",
		},
		{
			name:       "basic bool field",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "primitive", Name: "bool"},
			fieldName:  "Active",
			structName: "User",
			want:       "true",
		},
		{
			name:       "ID field uppercase",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "primitive", Name: "string"},
			fieldName:  "ID",
			structName: "User",
			want:       `"UserID"`,
		},
		{
			name:       "Id field mixed case",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "primitive", Name: "string"},
			fieldName:  "Id",
			structName: "Account",
			want:       `"AccountID"`,
		},
		{
			name:       "proto enum field",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "enum", Name: "Status"},
			fieldName:  "Status",
			structName: "Account",
			want:       "*FixtureStatus()",
		},
		{
			name:       "proto struct field",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "struct", Name: "Address"},
			fieldName:  "Address",
			structName: "User",
			want:       "*FixtureAddress()",
		},
		{
			name:       "proto pointer field",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "pointer", Elem: &generator.TypeRef{Kind: "struct", Name: "Address"}},
			fieldName:  "Address",
			structName: "User",
			want:       "*FixtureAddress()",
		},
		{
			name:       "proto slice field",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "slice", Elem: &generator.TypeRef{Kind: "primitive", Name: "string"}},
			fieldName:  "Tags",
			structName: "User",
			want:       `[]string{"Tags"}`,
		},
		{
			name:       "slice of pointers to struct",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "slice", Elem: &generator.TypeRef{Kind: "pointer", Elem: &generator.TypeRef{Kind: "struct", Name: "Activity"}}},
			fieldName:  "Activities",
			structName: "User",
			want:       `[]*Activity{*FixtureActivity()}`,
		},
		{
			name:       "pointer to unknown/external type returns nil",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "pointer", Elem: &generator.TypeRef{Kind: "unknown"}},
			fieldName:  "StateTimestamp",
			structName: "User",
			want:       "nil",
		},
		{
			name:       "pointer to timestamppb.Timestamp",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "pointer", Elem: &generator.TypeRef{Kind: "external", Name: "Timestamp"}},
			fieldName:  "CreatedAt",
			structName: "User",
			want:       "timestamppb.New(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))",
		},
		{
			name:       "oneof field picks first implementation",
			model:      oneofModel,
			typeRef:    generator.TypeRef{Kind: "oneof", Name: "isUserReference_Id"},
			fieldName:  "Id",
			structName: "UserReference",
			want:       "&UserReference_EmailId{\n\t\t\tEmailId: \"EmailId\",\n\t\t}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generator.GenValue(tt.model, tt.typeRef, tt.fieldName, tt.structName)
			if got != tt.want {
				t.Errorf("GenValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProtoInternalFieldsSkipped(t *testing.T) {
	internalFields := []string{"state", "unknownFields", "sizeCache"}

	for _, field := range internalFields {
		t.Run(field, func(t *testing.T) {
			if !generator.ProtoInternalFields[field] {
				t.Errorf("field %q should be in ProtoInternalFields map", field)
			}
		})
	}
}

func TestGenerateWithOptions(t *testing.T) {
	tests := []struct {
		name     string
		model    *generator.Model
		pkg      string
		opts     generator.GenerateOptions
		contains []string
	}{
		{
			name: "with type and func prefix (mod style)",
			model: &generator.Model{
				Structs: map[string]*generator.Struct{
					"User": {
						Name: "User",
						Fields: []generator.Field{
							{Name: "ID", Type: generator.TypeRef{Kind: "primitive", Name: "string"}},
							{Name: "Role", Type: generator.TypeRef{Kind: "enum", Name: "Role"}},
							{Name: "Address", Type: generator.TypeRef{Kind: "pointer", Elem: &generator.TypeRef{Kind: "struct", Name: "Address"}}},
						},
					},
				},
				Enums: map[string]*generator.Enum{
					"Role": {
						Name:   "Role",
						Values: []string{"ROLE_UNSPECIFIED"},
					},
				},
				OneOfs: map[string]string{},
			},
			pkg: "fixtures",
			opts: generator.GenerateOptions{
				TypePrefix: "account",
				FuncPrefix: "Account",
				ModStyle:   true,
			},
			contains: []string{
				"func FixtureAccountUser(mods ...func(*account.User)) *account.User {",
				"value := &account.User{",
				"Role: *FixtureAccountRole()",
				"Address: *FixtureAccountAddress()",
				"for _, mod := range mods {",
				"mod(value)",
				"return value",
				"func FixtureAccountRole(mods ...func(*account.Role)) *account.Role {",
				"value := account.ROLE_UNSPECIFIED",
			},
		},
		{
			name: "with type and func prefix (classic style)",
			model: &generator.Model{
				Structs: map[string]*generator.Struct{
					"User": {
						Name: "User",
						Fields: []generator.Field{
							{Name: "ID", Type: generator.TypeRef{Kind: "primitive", Name: "string"}},
							{Name: "Role", Type: generator.TypeRef{Kind: "enum", Name: "Role"}},
							{Name: "Address", Type: generator.TypeRef{Kind: "pointer", Elem: &generator.TypeRef{Kind: "struct", Name: "Address"}}},
						},
					},
				},
				Enums: map[string]*generator.Enum{
					"Role": {
						Name:   "Role",
						Values: []string{"ROLE_UNSPECIFIED"},
					},
				},
				OneOfs: map[string]string{},
			},
			pkg: "fixtures",
			opts: generator.GenerateOptions{
				TypePrefix: "account",
				FuncPrefix: "Account",
				ModStyle:   false,
			},
			contains: []string{
				"func FixtureAccountUser() account.User {",
				"return account.User{",
				"Role: FixtureAccountRole()",
				"Address: ptr(FixtureAccountAddress())",
				"func FixtureAccountRole() account.Role {",
				"return account.ROLE_UNSPECIFIED",
			},
		},
		{
			name: "with slice of structs and prefix (mod style)",
			model: &generator.Model{
				Structs: map[string]*generator.Struct{
					"User": {
						Name: "User",
						Fields: []generator.Field{
							{Name: "Addresses", Type: generator.TypeRef{Kind: "slice", Elem: &generator.TypeRef{Kind: "pointer", Elem: &generator.TypeRef{Kind: "struct", Name: "Address"}}}},
						},
					},
				},
				Enums:  map[string]*generator.Enum{},
				OneOfs: map[string]string{},
			},
			pkg: "fixtures",
			opts: generator.GenerateOptions{
				TypePrefix: "account",
				FuncPrefix: "M",
				ModStyle:   true,
			},
			contains: []string{
				"func FixtureMUser(mods ...func(*account.User)) *account.User {",
				"Addresses: []*account.Address{*FixtureMAddress()}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generator.GenerateWithOptions(tt.model, tt.pkg, tt.opts)
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("GenerateWithOptions() output missing %q\nGot:\n%s", want, got)
				}
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	tests := []struct {
		name     string
		model    *generator.Model
		pkg      string
		contains []string
	}{
		{
			name: "basic struct (mod style default)",
			model: &generator.Model{
				Structs: map[string]*generator.Struct{
					"User": {
						Name: "User",
						Fields: []generator.Field{
							{Name: "FirstName", Type: generator.TypeRef{Kind: "primitive", Name: "string"}},
							{Name: "Age", Type: generator.TypeRef{Kind: "primitive", Name: "int"}},
						},
					},
				},
				Enums:  map[string]*generator.Enum{},
				OneOfs: map[string]string{},
			},
			pkg: "fixtures",
			contains: []string{
				"package fixtures",
				"func FixtureUser(mods ...func(*User)) *User {",
				`FirstName: "FirstName"`,
				"Age: 1",
				"for _, mod := range mods {",
				"return value",
			},
		},
		{
			name: "struct with ID field (mod style)",
			model: &generator.Model{
				Structs: map[string]*generator.Struct{
					"User": {
						Name: "User",
						Fields: []generator.Field{
							{Name: "ID", Type: generator.TypeRef{Kind: "primitive", Name: "string"}},
						},
					},
				},
				Enums:  map[string]*generator.Enum{},
				OneOfs: map[string]string{},
			},
			pkg: "fixtures",
			contains: []string{
				`ID: "UserID"`,
				"func FixtureUser(mods ...func(*User)) *User {",
			},
		},
		{
			name: "user with enum (mod style)",
			model: &generator.Model{
				Structs: map[string]*generator.Struct{
					"User": {
						Name: "User",
						Fields: []generator.Field{
							{Name: "Status", Type: generator.TypeRef{Kind: "enum", Name: "Status"}},
						},
					},
				},
				Enums: map[string]*generator.Enum{
					"Status": {
						Name:   "Status",
						Values: []string{"STATUS_UNSPECIFIED", "STATUS_ACTIVE"},
					},
				},
				OneOfs: map[string]string{},
			},
			pkg: "fixtures",
			contains: []string{
				"func FixtureStatus(mods ...func(*Status)) *Status {",
				"value := STATUS_UNSPECIFIED",
				"Status: *FixtureStatus()",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generator.Generate(tt.model, tt.pkg)
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("Generate() output missing %q\nGot:\n%s", want, got)
				}
			}
		})
	}
}
