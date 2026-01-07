//go:build js && wasm

package main

import (
	"syscall/js"

	"fixture-generator/pkg/generator"
)

func main() {
	js.Global().Set("generateFixtures", js.FuncOf(generateFixtures))
	select {}
}

func generateFixtures(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return map[string]interface{}{
			"error": "expected at least 2 arguments: source code and package name",
		}
	}

	source := args[0].String()
	pkgName := args[1].String()

	opts := generator.GenerateOptions{
		ModStyle: true, // default to mod style
	}
	if len(args) >= 3 && args[2].String() != "" {
		opts.TypePrefix = args[2].String()
	}
	if len(args) >= 4 && args[3].String() != "" {
		opts.FuncPrefix = args[3].String()
	}
	if len(args) >= 5 {
		opts.ModStyle = args[4].Bool()
	}

	model, err := generator.ParseSource(source)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	result, _ := generator.GenerateFormattedWithOptions(model, pkgName, opts)

	return map[string]interface{}{
		"output": result,
	}
}
