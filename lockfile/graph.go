package lockfile

// Graph is a read-only view over a Lock's resolved dependency edges.
type Graph struct {
	byName map[string]Package
	order  []string
}

// Graph builds a [Graph] view of l's packages. Safe to call on an l with
// validation diagnostics (e.g. duplicate names): later entries win.
func (l *Lock) Graph() Graph {
	g := Graph{byName: make(map[string]Package, len(l.Packages))}

	for _, p := range l.Packages {
		if _, exists := g.byName[p.Name]; !exists {
			g.order = append(g.order, p.Name)
		}

		g.byName[p.Name] = p
	}

	return g
}

// Names returns every package name, in the lockfile's declared order.
func (g Graph) Names() []string {
	out := make([]string, len(g.order))
	copy(out, g.order)

	return out
}

// Dependencies returns the direct dependency names of name, or nil if name
// is unknown.
func (g Graph) Dependencies(name string) []string {
	p, ok := g.byName[name]
	if !ok {
		return nil
	}

	return p.Dependencies
}

// Roots returns package names that no other package in the graph depends
// on directly — the entry points of the resolved graph.
func (g Graph) Roots() []string {
	referenced := make(map[string]bool)

	for _, name := range g.order {
		for _, dep := range g.byName[name].Dependencies {
			referenced[dep] = true
		}
	}

	var roots []string

	for _, name := range g.order {
		if !referenced[name] {
			roots = append(roots, name)
		}
	}

	return roots
}
