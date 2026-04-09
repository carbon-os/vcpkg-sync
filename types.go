package main

// Config holds all resolved runtime settings.
type Config struct {
	RegistryDir string
	Port        string // empty = auto-detect from ports/
	SourceURL   string // empty = parsed from portfile.cmake
	DryRun      bool
	NoPush      bool
	Verbose     bool
}

// VersionEntry is one entry in versions/<x>-/<port>.json.
type VersionEntry struct {
	Version string `json:"version"`
	GitTree string `json:"git-tree"`
}

// VersionManifest is the full structure of versions/<x>-/<port>.json.
type VersionManifest struct {
	Versions []VersionEntry `json:"versions"`
}

// BaselineEntry is the per-port record inside versions/baseline.json.
type BaselineEntry struct {
	Baseline    string `json:"baseline"`
	PortVersion int    `json:"port-version"`
}

// BaselineManifest is versions/baseline.json.
type BaselineManifest struct {
	Default map[string]BaselineEntry `json:"default"`
}