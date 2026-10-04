package physmath

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strings"
	"testing"
	"unicode"
)

// Plan9.3 gates [4] (math purity) + import discipline. Mechanical part:
// (a) module imports stdlib + phys-artifact only; (b) no exported identifier
// contains a physical/theory vocabulary whole-word. The mathematician test
// itself (is this API explainable without physics?) remains a REVIEW criterion.
func TestImportDiscipline(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.HasPrefix(dep, "github.com/PithomLabs/phys-math") ||
			strings.HasPrefix(dep, "github.com/PithomLabs/phys-artifact") {
			continue
		}
		if strings.Contains(strings.Split(dep, "/")[0], ".") {
			t.Fatalf("non-stdlib dependency: %s", dep)
		}
	}
}

// physicalVocab lists whole-word fragments that must not appear in exported
// API. Mathematical Dimension/operators/Function are deliberately absent:
// the ban targets physical meaning, not mathematical vocabulary (D16).
var physicalVocab = []string{
	"GR", "QM", "Physical", "Physics", "Framework", "Assumption",
	"Interpretation", "Metric", "Curvature", "Christoffel", "Ricci",
	"Einstein", "Vacuum", "Bohm", "Quantum", "Trust", "Corpus",
	"Provenance", "Session", "Spacetime", "Minkowski", "Schwarzschild",
	"Meter", "Second", "Chart", "Ansatz", "Hilbert", "Schrodinger",
}

func exportedNames(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	var names []string
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				n := d.Name.Name
				if d.Recv == nil && unicode.IsUpper(rune(n[0])) {
					names = append(names, n)
				}
				if d.Recv != nil && unicode.IsUpper(rune(n[0])) {
					names = append(names, n) // exported methods
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if unicode.IsUpper(rune(s.Name.Name[0])) {
							names = append(names, s.Name.Name)
						}
					case *ast.ValueSpec:
						for _, id := range s.Names {
							if unicode.IsUpper(rune(id.Name[0])) {
								names = append(names, id.Name)
							}
						}
					}
				}
			}
		}
	}
	return names
}

func TestNoPhysicalVocabulary(t *testing.T) {
	for _, n := range exportedNames(t) {
		for _, v := range physicalVocab {
			if strings.Contains(n, v) {
				t.Fatalf("exported identifier %q contains physical vocabulary %q", n, v)
			}
		}
	}
}
