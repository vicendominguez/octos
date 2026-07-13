package main

import "path/filepath"

// Directory constants
const (
	// OctosDir is the root directory for octos state and artifacts.
	OctosDir = ".octos"

	// OctosDirSlash is the path fragment used when filtering paths during directory scanning.
	OctosDirSlash = "/.octos/"
)

// Derived directory paths
var (
	// ArtifactsDir is the directory where step artifacts are stored.
	ArtifactsDir = filepath.Join(OctosDir, "artifacts")

	// StateDir is the directory where pipeline checkpoint state is stored.
	StateDir = filepath.Join(OctosDir, "state")
)

// Interpolation and key prefixes
const (
	// ArtifactKeyPrefix is the prefix used when storing artifact content in ctx.Outputs.
	ArtifactKeyPrefix = "artifact."
)

// Output limits
const (
	// MaxOutputBytes is the maximum size of captured step output before truncation (512 KB).
	MaxOutputBytes = 512 * 1024
)

// Failure policy constants
const (
	FailurePolicyFailFast = "fail_fast"
	FailurePolicyRetry    = "retry"
	FailurePolicySkip     = "skip"
)
