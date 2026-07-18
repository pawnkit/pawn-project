// Package manifest loads and validates pawn.json and pawn.yaml files.
package manifest

// Manifest is a decoded project manifest.
type Manifest struct {
	Entry  string `json:"entry,omitempty" yaml:"entry,omitempty"`
	Output string `json:"output,omitempty" yaml:"output,omitempty"`
	User   string `json:"user,omitempty" yaml:"user,omitempty"`
	Repo   string `json:"repo,omitempty" yaml:"repo,omitempty"`

	Dependencies    []Dependency `json:"-" yaml:"-"`
	DevDependencies []Dependency `json:"-" yaml:"-"`

	Preset      string `json:"preset,omitempty" yaml:"preset,omitempty"`
	Local       bool   `json:"local,omitempty" yaml:"local,omitempty"`
	IncludePath string `json:"include_path,omitempty" yaml:"include_path,omitempty"`

	Resources             map[string]any `json:"resources,omitempty" yaml:"resources,omitempty"`
	ExtractIgnorePatterns []string       `json:"extract_ignore_patterns,omitempty" yaml:"extract_ignore_patterns,omitempty"`
	Contributors          []any          `json:"contributors,omitempty" yaml:"contributors,omitempty"`
	Website               string         `json:"website,omitempty" yaml:"website,omitempty"`
	Experimental          Experimental   `json:"experimental" yaml:"experimental"`

	Build  *Build  `json:"build,omitempty" yaml:"build,omitempty"`
	Builds []Build `json:"builds,omitempty" yaml:"builds,omitempty"`

	Runtime  *Runtime  `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Runtimes []Runtime `json:"runtimes,omitempty" yaml:"runtimes,omitempty"`

	PawnKit *PawnKitExtension `json:"pawnkit,omitempty" yaml:"pawnkit,omitempty"`

	// SourcePath is the absolute path passed to Load.
	SourcePath string `json:"-" yaml:"-"`
}

// Experimental mirrors the schema's "experimental" object.
type Experimental struct {
	// BuildFile distinguishes an omitted value from false.
	BuildFile *bool `json:"build_file,omitempty" yaml:"build_file,omitempty"`
}

// BuildFileEnabled reports the effective value of Experimental.BuildFile,
// applying the schema's documented default of true.
func (e Experimental) BuildFileEnabled() bool {
	if e.BuildFile == nil {
		return true
	}

	return *e.BuildFile
}

// Build mirrors the schema's "build"/"builds[]" object.
type Build struct {
	Name      string         `json:"name,omitempty" yaml:"name,omitempty"`
	Args      []string       `json:"args,omitempty" yaml:"args,omitempty"`
	Constants map[string]any `json:"constants,omitempty" yaml:"constants,omitempty"`
	Compiler  *CompilerRef   `json:"compiler,omitempty" yaml:"compiler,omitempty"`
	Input     string         `json:"input,omitempty" yaml:"input,omitempty"`
	Output    string         `json:"output,omitempty" yaml:"output,omitempty"`
	Includes  []string       `json:"includes,omitempty" yaml:"includes,omitempty"`
}

// CompilerRef mirrors the schema's "build.compiler" object.
type CompilerRef struct {
	Site    string `json:"site,omitempty" yaml:"site,omitempty"`
	User    string `json:"user,omitempty" yaml:"user,omitempty"`
	Repo    string `json:"repo,omitempty" yaml:"repo,omitempty"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// Runtime describes a server configuration. Extra preserves unknown fields.
type Runtime struct {
	Name          string         `json:"name,omitempty" yaml:"name,omitempty"`
	Version       string         `json:"version,omitempty" yaml:"version,omitempty"`
	Mode          string         `json:"mode,omitempty" yaml:"mode,omitempty"`
	Port          int            `json:"port,omitempty" yaml:"port,omitempty"`
	Gamemodes     []string       `json:"gamemodes,omitempty" yaml:"gamemodes,omitempty"`
	Filterscripts []string       `json:"filterscripts,omitempty" yaml:"filterscripts,omitempty"`
	Plugins       []string       `json:"plugins,omitempty" yaml:"plugins,omitempty"`
	Extra         map[string]any `json:"extra,omitempty" yaml:"extra,omitempty"`
}

// PawnKitExtension contains optional PawnKit settings.
type PawnKitExtension struct {
	SchemaVersion int                       `json:"schemaVersion" yaml:"schemaVersion"`
	Profile       string                    `json:"profile,omitempty" yaml:"profile,omitempty"`
	IncludePaths  []string                  `json:"includePaths,omitempty" yaml:"includePaths,omitempty"`
	Tests         map[string]any            `json:"tests,omitempty" yaml:"tests,omitempty"`
	Tool          map[string]map[string]any `json:"tool,omitempty" yaml:"tool,omitempty"`
	Lockfile      string                    `json:"lockfile,omitempty" yaml:"lockfile,omitempty"`
}

// LockfilePath returns the configured lockfile path, applying the schema's
// documented default of "pawn.lock".
func (p *PawnKitExtension) LockfilePath() string {
	if p == nil || p.Lockfile == "" {
		return "pawn.lock"
	}

	return p.Lockfile
}

// EffectiveIncludePaths returns all declared include paths in search order.
func (m *Manifest) EffectiveIncludePaths() []string {
	var out []string

	if m.IncludePath != "" {
		out = append(out, m.IncludePath)
	}

	if m.PawnKit != nil {
		out = append(out, m.PawnKit.IncludePaths...)
	}

	return out
}
