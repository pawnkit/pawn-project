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

	Resources             []Resource   `json:"resources,omitempty" yaml:"resources,omitempty"`
	ExtractIgnorePatterns []string     `json:"extract_ignore_patterns,omitempty" yaml:"extract_ignore_patterns,omitempty"`
	Contributors          []any        `json:"contributors,omitempty" yaml:"contributors,omitempty"`
	Website               string       `json:"website,omitempty" yaml:"website,omitempty"`
	Experimental          Experimental `json:"experimental" yaml:"experimental"`

	Build  *Build  `json:"build,omitempty" yaml:"build,omitempty"`
	Builds []Build `json:"builds,omitempty" yaml:"builds,omitempty"`

	Runtime  *Runtime  `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Runtimes []Runtime `json:"runtimes,omitempty" yaml:"runtimes,omitempty"`

	PawnKit *PawnKitExtension `json:"pawnkit,omitempty" yaml:"pawnkit,omitempty"`

	// SourcePath is the absolute path passed to Load.
	SourcePath string `json:"-" yaml:"-"`
}

// Resource describes one sampctl release asset.
type Resource struct {
	Name     string            `json:"name,omitempty" yaml:"name,omitempty"`
	Platform string            `json:"platform,omitempty" yaml:"platform,omitempty"`
	Version  string            `json:"version,omitempty" yaml:"version,omitempty"`
	Archive  bool              `json:"archive,omitempty" yaml:"archive,omitempty"`
	Includes []string          `json:"includes,omitempty" yaml:"includes,omitempty"`
	Plugins  []string          `json:"plugins,omitempty" yaml:"plugins,omitempty"`
	Files    map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
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
	Name              string         `json:"name,omitempty" yaml:"name,omitempty"`
	Version           string         `json:"version,omitempty" yaml:"version,omitempty"`
	Endpoint          string         `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Mode              string         `json:"mode,omitempty" yaml:"mode,omitempty"`
	Port              int            `json:"port,omitempty" yaml:"port,omitempty"`
	Gamemodes         []string       `json:"gamemodes,omitempty" yaml:"gamemodes,omitempty"`
	Filterscripts     []string       `json:"filterscripts,omitempty" yaml:"filterscripts,omitempty"`
	Plugins           []string       `json:"plugins,omitempty" yaml:"plugins,omitempty"`
	RCONPassword      string         `json:"rcon_password,omitempty" yaml:"rcon_password,omitempty"`
	Announce          *bool          `json:"announce,omitempty" yaml:"announce,omitempty"`
	MaxPlayers        int            `json:"maxplayers,omitempty" yaml:"maxplayers,omitempty"`
	LANMode           *bool          `json:"lanmode,omitempty" yaml:"lanmode,omitempty"`
	Query             *bool          `json:"query,omitempty" yaml:"query,omitempty"`
	RCON              *bool          `json:"rcon,omitempty" yaml:"rcon,omitempty"`
	LogQueries        *bool          `json:"logqueries,omitempty" yaml:"logqueries,omitempty"`
	StreamRate        int            `json:"stream_rate,omitempty" yaml:"stream_rate,omitempty"`
	StreamDistance    float64        `json:"stream_distance,omitempty" yaml:"stream_distance,omitempty"`
	Sleep             any            `json:"sleep,omitempty" yaml:"sleep,omitempty"`
	MaxNPC            int            `json:"maxnpc,omitempty" yaml:"maxnpc,omitempty"`
	OnFootRate        int            `json:"onfoot_rate,omitempty" yaml:"onfoot_rate,omitempty"`
	InCarRate         int            `json:"incar_rate,omitempty" yaml:"incar_rate,omitempty"`
	WeaponRate        int            `json:"weapon_rate,omitempty" yaml:"weapon_rate,omitempty"`
	ChatLogging       *bool          `json:"chatlogging,omitempty" yaml:"chatlogging,omitempty"`
	Timestamp         *bool          `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
	Bind              string         `json:"bind,omitempty" yaml:"bind,omitempty"`
	Password          string         `json:"password,omitempty" yaml:"password,omitempty"`
	Hostname          string         `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	Language          string         `json:"language,omitempty" yaml:"language,omitempty"`
	MapName           string         `json:"mapname,omitempty" yaml:"mapname,omitempty"`
	WebURL            string         `json:"weburl,omitempty" yaml:"weburl,omitempty"`
	GameModeText      string         `json:"gamemodetext,omitempty" yaml:"gamemodetext,omitempty"`
	NoSign            string         `json:"nosign,omitempty" yaml:"nosign,omitempty"`
	LogTimeFormat     string         `json:"logtimeformat,omitempty" yaml:"logtimeformat,omitempty"`
	MessageHoleLimit  int            `json:"messageholelimit,omitempty" yaml:"messageholelimit,omitempty"`
	MessagesLimit     int            `json:"messageslimit,omitempty" yaml:"messageslimit,omitempty"`
	AckLimit          int            `json:"ackslimit,omitempty" yaml:"ackslimit,omitempty"`
	PlayerTimeout     int            `json:"playertimeout,omitempty" yaml:"playertimeout,omitempty"`
	MinConnectionTime int            `json:"minconnectiontime,omitempty" yaml:"minconnectiontime,omitempty"`
	LagCompMode       int            `json:"lagcompmode,omitempty" yaml:"lagcompmode,omitempty"`
	ConnectionSeed    int            `json:"connseedtime,omitempty" yaml:"connseedtime,omitempty"`
	DBLogging         *bool          `json:"db_logging,omitempty" yaml:"db_logging,omitempty"`
	DBLogQueries      *bool          `json:"db_log_queries,omitempty" yaml:"db_log_queries,omitempty"`
	ConnectionCookies *bool          `json:"conncookies,omitempty" yaml:"conncookies,omitempty"`
	CookieLogging     *bool          `json:"cookielogging,omitempty" yaml:"cookielogging,omitempty"`
	Extra             map[string]any `json:"extra,omitempty" yaml:"extra,omitempty"`
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
