package include

import (
	"strconv"
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
)

func BenchmarkResolveAcrossRoots(b *testing.B) {
	fsys := fsx.NewMem()
	roots := make([]string, 50)
	for i := range roots {
		roots[i] = "/includes/" + strconv.Itoa(i)
		fsys.AddFile(roots[i]+"/library.inc", nil)
	}
	resolver := New(fsys, roots)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, ok := resolver.Resolve("/project/main.pwn", "library", false); !ok {
			b.Fatal("include was not resolved")
		}
	}
}
