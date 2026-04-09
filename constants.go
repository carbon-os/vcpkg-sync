package main

import (
	"path/filepath"
	"strings"
)

const (
	portsDirName = "ports"
	versionsDirName = "versions"
	baselineFileName = "baseline.json"
)

// Relative paths (relative to registry root) — used for git staging.

func portfileRel(portName string) string {
	return filepath.Join(portsDirName, portName, "portfile.cmake")
}

func vcpkgJSONRel(portName string) string {
	return filepath.Join(portsDirName, portName, "vcpkg.json")
}

func baselineRel() string {
	return filepath.Join(versionsDirName, baselineFileName)
}

func versionFileRel(portName string) string {
	prefix := strings.ToLower(string([]rune(portName)[0])) + "-"
	return filepath.Join(versionsDirName, prefix, portName+".json")
}

// portTreePath returns the forward-slash path used to navigate git trees.
func portTreePath(portName string) string {
	return portsDirName + "/" + portName
}

// Absolute paths — used for filesystem reads/writes.

func portfileAbs(root, portName string) string {
	return filepath.Join(root, portfileRel(portName))
}

func vcpkgJSONAbs(root, portName string) string {
	return filepath.Join(root, vcpkgJSONRel(portName))
}

func baselineAbs(root string) string {
	return filepath.Join(root, baselineRel())
}

func versionFileAbs(root, portName string) string {
	return filepath.Join(root, versionFileRel(portName))
}

func portsDirAbs(root string) string {
	return filepath.Join(root, portsDirName)
}