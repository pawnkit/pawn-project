package project

import (
	"strconv"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawnkit-core/source"
)

func BenchmarkLoadManifestWithDependencies(b *testing.B) {
	var manifest strings.Builder
	manifest.WriteString(`{"entry":"gamemodes/main.pwn","dependencies":[`)
	for i := range 200 {
		if i != 0 {
			manifest.WriteByte(',')
		}
		manifest.WriteString(`"owner/package-`)
		manifest.WriteString(strconv.Itoa(i))
		manifest.WriteString(`"`)
	}
	manifest.WriteString(`]}`)

	fsys := fsx.NewMem().AddFile("/project/pawn.json", []byte(manifest.String()))
	registry := source.NewRegistry()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := Load(registry, fsys, "/project/gamemodes/main.pwn", Options{}); err != nil {
			b.Fatal(err)
		}
	}
}
