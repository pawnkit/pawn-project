package backend

import "errors"

var (
	ErrInvalidOperation = errors.New("backend: invalid operation")
	ErrMissingProfile   = errors.New("backend: project has no selected profile")
	ErrMissingEntry     = errors.New("backend: build requires an entry")
	ErrMissingOutput    = errors.New("backend: build or run requires an output")
	ErrInvalidDefine    = errors.New("backend: define value must be scalar")
	ErrInvalidCompiler  = errors.New("backend: compiler path must be absolute")
	ErrInvalidOutput    = errors.New("backend: output override must be absolute")
)
