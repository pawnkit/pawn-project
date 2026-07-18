// Command basic demonstrates loading an in-memory Pawn project.
package main

import (
	"fmt"

	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/project"
)

func main() {
	fsys := fsx.NewMem()
	fsys.AddFile("/game/pawn.json", []byte(`{
		"entry": "gamemodes/main.pwn",
		"output": "gamemodes/main.amx",
		"preset": "openmp",
		"include_path": "includes",
		"pawnkit": {"schemaVersion": 1}
	}`))
	fsys.AddFile("/game/gamemodes/main.pwn", []byte(`#include <a_samp>
main() {}`))
	fsys.AddFile("/game/includes/a_samp.inc", []byte("// stub"))

	reg := source.NewRegistry()

	p, err := project.Load(reg, fsys, "/game/gamemodes/main.pwn", project.Options{})
	if err != nil {
		panic(err)
	}

	fmt.Println("root:", p.Root())
	fmt.Println("profile:", p.Selection().ProfileID)
	fmt.Println("entry:", p.Paths().Entry)
	fmt.Println("include roots:", p.Paths().IncludeRoots)

	resolved, ok := p.IncludeResolver().Resolve(p.Paths().Entry, "a_samp", false)
	fmt.Println("#include <a_samp> resolves to:", resolved, ok)

	for _, d := range p.Diagnostics() {
		fmt.Printf("[%s] %s: %s\n", d.Severity, d.Code, d.Message)
	}
}
