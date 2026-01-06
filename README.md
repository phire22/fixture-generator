# Fixture Generator

**Try it online: https://phire22.github.io/fixture-generator/**

> ⚠️ **Disclaimer:** This is a simple tool that was coded heavily with Claude Opus. Not everything has been reviewed, so use with caution.

A simple tool that generates `Fixture<TypeName>()` functions for Go structs, type definitions, and enums.

## Features

- Generates fixture functions for structs with sensible default values
- Supports primitive types, pointers, slices, and nested structs
- Handles protobuf-generated types (skips internal fields like `state`, `sizeCache`, etc.)
- Supports enums (returns the first defined value)
- Supports oneofs (takes the first defined value)

## Installation

```bash
go install github.com/your-org/fixture-generator/main@latest
```

Or clone and build locally:

```bash
git clone <repository-url>
cd fixture-generator
go build -o fixture-generator ./main
```

## CLI Usage

```bash
go run ./main -pkg <package-path> -outpkg <output-package-name> -out <output-file>
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-pkg` | Path to the Go package to generate fixtures for | (required) |
| `-outpkg` | Package name for the generated file | `fixtures` |
| `-out` | Output file path (prints to stdout if not specified) | - |

### Example

```bash
go run ./main \
  -pkg ./path/to/your/package \
  -outpkg yourpackage \
  -out ./path/to/your/package/fixtures.go
```

## Web Interface

A browser-based version that uses WebAssembly to run the generator directly in your browser.

1. Paste your Go struct definitions
2. Click "Generate Fixtures"

To rebuild the WebAssembly binary:

```bash
cd web
./build.sh
```

## Generated Output Example

Given this input:

```go
package example

type User struct {
    ID        string
    FirstName string
    LastName  string
    Age       int
    Active    bool
    Address   *Address
    Tags      []string
}

type Address struct {
    Street  string
    City    string
    Country string
}
```

The generator produces:

```go
package fixtures

func ptr[T any](v T) *T { return &v }

func FixtureUser() User {
    return User{
        ID:        "UserID",
        FirstName: "FirstName",
        LastName:  "LastName",
        Age:       1,
        Active:    true,
        Address:   ptr(FixtureAddress()),
        Tags:      []string{"Tags"},
    }
}

func FixtureAddress() Address {
    return Address{
        Street:  "Street",
        City:    "City",
        Country: "Country",
    }
}
```

## License

MIT
