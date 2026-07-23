// Package backend defines resolved build-backend messages.
package backend

import "github.com/pawnkit/pawnkit-core/protocol"

const (
	ProtocolVersion = 1
	SchemaVersion   = 1
)

type Operation string

const (
	Restore Operation = "restore"
	Build   Operation = "build"
	Run     Operation = "run"
)

func (o Operation) IsValid() bool {
	switch o {
	case Restore, Build, Run:
		return true
	default:
		return false
	}
}

type Compiler struct {
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
}

type Request struct {
	Kind          string            `json:"kind"`
	SchemaVersion int               `json:"schemaVersion"`
	Operation     Operation         `json:"operation"`
	ProjectRoot   string            `json:"projectRoot"`
	Profile       string            `json:"profile"`
	Target        string            `json:"target"`
	Entry         string            `json:"entry,omitempty"`
	Output        string            `json:"output,omitempty"`
	IncludePaths  []string          `json:"includePaths"`
	Defines       map[string]string `json:"defines"`
	Compiler      *Compiler         `json:"compiler,omitempty"`
	Arguments     []string          `json:"arguments"`
}

type Identity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Kind            string      `json:"kind"`
	ProtocolVersion int         `json:"protocolVersion"`
	Name            string      `json:"name"`
	Version         string      `json:"version"`
	Operations      []Operation `json:"operations"`
	Profiles        []string    `json:"profiles"`
	Features        []string    `json:"features,omitempty"`
}

type Artifact struct {
	Path      string `json:"path"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256,omitempty"`
}

type Process struct {
	ExitCode  *int   `json:"exitCode,omitempty"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

type Runtime struct {
	Tier           string `json:"tier"`
	Implementation string `json:"implementation"`
	Version        string `json:"version,omitempty"`
}

type Result struct {
	Kind          string                `json:"kind"`
	SchemaVersion int                   `json:"schemaVersion"`
	Status        string                `json:"status"`
	Backend       Identity              `json:"backend"`
	Artifacts     []Artifact            `json:"artifacts"`
	Diagnostics   []protocol.Diagnostic `json:"diagnostics"`
	Process       *Process              `json:"process,omitempty"`
	Runtime       *Runtime              `json:"runtime,omitempty"`
}

type RequestOptions struct {
	Compiler *Compiler
	Output   string
}
