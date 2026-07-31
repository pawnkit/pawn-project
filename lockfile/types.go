// Package lockfile loads and validates pawn.lock files.
package lockfile

// Lock is the decoded, validated form of a pawn.lock file.
type Lock struct {
	SchemaVersion    int                `json:"schemaVersion"`
	GeneratedAt      string             `json:"generatedAt,omitempty"`
	ManifestChecksum string             `json:"manifestChecksum,omitempty"`
	Compiler         *Compiler          `json:"compiler,omitempty"`
	RuntimeProfile   string             `json:"runtimeProfile,omitempty"`
	Packages         []Package          `json:"packages"`
	Resources        []ResolvedResource `json:"resources,omitempty"`

	// SourcePath is the absolute path this Lock was loaded from.
	SourcePath string `json:"-"`
}

// ResolvedResource is a verified package asset for one host target.
type ResolvedResource struct {
	Package  string                 `json:"package"`
	Resource string                 `json:"resource"`
	Target   string                 `json:"target"`
	URL      string                 `json:"url"`
	Size     int64                  `json:"size"`
	Checksum string                 `json:"checksum"`
	Archive  string                 `json:"archive"`
	Files    []ResolvedResourceFile `json:"files"`
}

// ResolvedResourceFile is one file installed from a resource.
type ResolvedResourceFile struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Size        int64  `json:"size"`
	Checksum    string `json:"checksum"`
}

// Compiler mirrors the schema's "compiler" object.
type Compiler struct {
	Vendor   string `json:"vendor"`
	Version  string `json:"version"`
	Checksum string `json:"checksum,omitempty"`
}

// Package is one resolved dependency-graph entry.
type Package struct {
	Key               string             `json:"-"`
	Constraint        string             `json:"-"`
	Name              string             `json:"name"`
	Resolved          string             `json:"resolved"`
	Version           string             `json:"version,omitempty"`
	Commit            string             `json:"commit"`
	Source            PackageSource      `json:"source"`
	Checksum          string             `json:"checksum,omitempty"`
	Integrity         string             `json:"-"`
	Kind              string             `json:"kind"`
	Branch            string             `json:"-"`
	Path              string             `json:"-"`
	Transitive        bool               `json:"-"`
	RequiredBy        []string           `json:"-"`
	PlatformArtifacts []PlatformArtifact `json:"platformArtifacts,omitempty"`
	Dependencies      []string           `json:"dependencies,omitempty"`
}

// PackageSource mirrors the schema's "packages[].source" object.
type PackageSource struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// PlatformArtifact mirrors the schema's "packages[].platformArtifacts[]"
// object.
type PlatformArtifact struct {
	Platform string `json:"platform"`
	URL      string `json:"url,omitempty"`
	Path     string `json:"path,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

// Kind enum values, mirroring the schema.
const (
	KindDependency    = "dependency"
	KindDevDependency = "dev-dependency"
	KindPlugin        = "plugin"
	KindComponent     = "component"
	KindIncludes      = "includes"
	KindFilterscript  = "filterscript"
)

// Source type enum values, mirroring the schema.
const (
	SourceTypeGit     = "git"
	SourceTypeArchive = "archive"
	SourceTypeLocal   = "local"
)

// ByName returns the package named name, if present.
func (l *Lock) ByName(name string) (Package, bool) {
	for _, p := range l.Packages {
		if p.Name == name {
			return p, true
		}
	}

	return Package{}, false
}
