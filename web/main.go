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
			"error": "expected 2 arguments: source code and package name",
		}
	}

	source := args[0].String()
	pkgName := args[1].String()

	model, err := generator.ParseSource(source)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	result, _ := generator.GenerateFormatted(model, pkgName)

	return map[string]interface{}{
		"output": result,
	}
}
