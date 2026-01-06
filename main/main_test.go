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
		Structs: map[string]*generator.Struct{},
		Enums:   map[string]*generator.Enum{},
		OneOfs: map[string]string{
			"isItemVersionReference_Id": "ItemVersionReference_OseonId",
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
			structName: "ProductionOrder",
			want:       `"ProductionOrderID"`,
		},
		{
			name:       "proto enum field",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "enum", Name: "Status"},
			fieldName:  "Status",
			structName: "ProductionOrder",
			want:       "FixtureStatus()",
		},
		{
			name:       "proto struct field",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "struct", Name: "Address"},
			fieldName:  "Address",
			structName: "User",
			want:       "FixtureAddress()",
		},
		{
			name:       "proto pointer field",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "pointer", Elem: &generator.TypeRef{Kind: "struct", Name: "Address"}},
			fieldName:  "Address",
			structName: "User",
			want:       "ptr(FixtureAddress())",
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
			structName: "Operation",
			want:       `[]*Activity{ptr(FixtureActivity())}`,
		},
		{
			name:       "pointer to unknown/external type returns nil",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "pointer", Elem: &generator.TypeRef{Kind: "unknown"}},
			fieldName:  "StateTimestamp",
			structName: "ProductionOrder",
			want:       "nil",
		},
		{
			name:       "pointer to timestamppb.Timestamp",
			model:      emptyModel,
			typeRef:    generator.TypeRef{Kind: "pointer", Elem: &generator.TypeRef{Kind: "external", Name: "Timestamp"}},
			fieldName:  "CreatedAt",
			structName: "ProductionOrder",
			want:       "timestamppb.New(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))",
		},
		{
			name:       "oneof field picks first implementation",
			model:      oneofModel,
			typeRef:    generator.TypeRef{Kind: "oneof", Name: "isItemVersionReference_Id"},
			fieldName:  "Id",
			structName: "ItemVersionReference",
			want:       "&ItemVersionReference_OseonId{}",
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

func TestGenerate(t *testing.T) {
	tests := []struct {
		name     string
		model    *generator.Model
		pkg      string
		contains []string
	}{
		{
			name: "basic struct",
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
				"func FixtureUser() User {",
				`FirstName: "FirstName"`,
				"Age: 1",
			},
		},
		{
			name: "struct with ID field",
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
			},
		},
		{
			name: "proto with enum",
			model: &generator.Model{
				Structs: map[string]*generator.Struct{
					"ProductionOrder": {
						Name: "ProductionOrder",
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
				"func FixtureStatus() Status {",
				"return STATUS_UNSPECIFIED",
				"Status: FixtureStatus()",
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
