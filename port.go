package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func run(cfg Config) error {
	// 1. Resolve port name from -port flag or auto-detect
	portName, err := resolvePortName(cfg)
	if err != nil {
		return err
	}

	// 2. Read the existing portfile
	portfilePath := portfileAbs(cfg.RegistryDir, portName)
	raw, err := os.ReadFile(portfilePath)
	if err != nil {
		return fmt.Errorf("read portfile: %w", err)
	}
	portfileContent := string(raw)

	// 3. Resolve source URL — flag takes priority, then parse from portfile
	sourceURL := cfg.SourceURL
	if sourceURL == "" {
		var ok bool
		sourceURL, ok = extractPortfileURL(portfileContent)
		if !ok {
			return fmt.Errorf(
				"cannot parse URL from %s; provide it with -source",
				portfilePath,
			)
		}
	}

	// 4. Fetch latest commit from upstream
	fmt.Printf("→ resolving HEAD of %s\n", sourceURL)
	newRef, err := resolveRemoteHEAD(sourceURL)
	if err != nil {
		return err
	}
	fmt.Printf("  ref: %s\n", newRef)

	// 5. Read current version from version manifest
	versionPath := versionFileAbs(cfg.RegistryDir, portName)
	var manifest VersionManifest
	if err := readJSON(versionPath, &manifest); err != nil {
		return err
	}
	if len(manifest.Versions) == 0 {
		return fmt.Errorf("no versions found in %s", versionPath)
	}
	currentVer := manifest.Versions[0].Version
	newVer, err := bumpPatch(currentVer)
	if err != nil {
		return err
	}

	fmt.Printf("→ port:    %s\n", portName)
	fmt.Printf("→ version: %s → %s\n", currentVer, newVer)

	if cfg.DryRun {
		fmt.Println("\n(dry run — no changes made)")
		return nil
	}

	// 6. Open the registry repo
	repo, err := openRepo(cfg.RegistryDir)
	if err != nil {
		return err
	}
	sig, err := repoSignature(repo)
	if err != nil {
		return err
	}

	// 7. Update portfile.cmake — only the REF line, everything else preserved
	fmt.Println("→ writing port files")
	if err := writeText(portfilePath, updatePortfileRef(portfileContent, newRef)); err != nil {
		return err
	}

	// 8. Update version in vcpkg.json — only the version field, deps preserved
	if err := patchJSONField(vcpkgJSONAbs(cfg.RegistryDir, portName), "version", newVer); err != nil {
		return err
	}

	// 9. Update baseline.json
	var baseline BaselineManifest
	if err := readJSON(baselineAbs(cfg.RegistryDir), &baseline); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if baseline.Default == nil {
		baseline.Default = make(map[string]BaselineEntry)
	}
	baseline.Default[portName] = BaselineEntry{Baseline: newVer, PortVersion: 0}
	if err := writeJSON(baselineAbs(cfg.RegistryDir), baseline); err != nil {
		return err
	}

	// 10. Commit port files + baseline — we derive the tree hash from this commit
	fmt.Println("→ committing port files")
	portCommit, err := stageAndCommit(
		repo,
		[]string{portfileRel(portName), vcpkgJSONRel(portName), baselineRel()},
		fmt.Sprintf("chore(vcpkg-sync): update port %s → %s", portName, newVer),
		sig,
	)
	if err != nil {
		return err
	}

	// 11. Derive the port directory tree hash from that commit
	fmt.Println("→ deriving port tree hash")
	treeHash, err := subtreeHash(repo, portCommit, portName)
	if err != nil {
		return err
	}
	fmt.Printf("  tree: %s\n", treeHash)

	// 12. Prepend new entry to version manifest
	fmt.Println("→ updating version manifest")
	manifest.Versions = append(
		[]VersionEntry{{Version: newVer, GitTree: treeHash}},
		manifest.Versions...,
	)
	if err := writeJSON(versionPath, manifest); err != nil {
		return err
	}

	// 13. Commit version manifest separately
	if _, err := stageAndCommit(
		repo,
		[]string{versionFileRel(portName)},
		fmt.Sprintf("chore(vcpkg-sync): version manifest %s → %s", portName, newVer),
		sig,
	); err != nil {
		return err
	}

	// 14. Push
	if !cfg.NoPush {
		fmt.Println("→ pushing")
		if err := push(repo, cfg.Verbose); err != nil {
			return err
		}
	}

	// 15. Print vcpkg-configuration.json snippet
	baseline2, err := headCommitHash(repo)
	if err != nil {
		return err
	}
	printSnippet(sourceURL, portName, baseline2)

	return nil
}

func resolvePortName(cfg Config) (string, error) {
	if cfg.Port != "" {
		return cfg.Port, nil
	}
	entries, err := os.ReadDir(portsDirAbs(cfg.RegistryDir))
	if err != nil {
		return "", fmt.Errorf("read ports directory: %w", err)
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() {
			found = append(found, e.Name())
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("no ports found in %s", portsDirAbs(cfg.RegistryDir))
	case 1:
		fmt.Printf("→ auto-detected port: %s\n", found[0])
		return found[0], nil
	default:
		return "", fmt.Errorf(
			"multiple ports found (%s) — use -port to specify one",
			strings.Join(found, ", "),
		)
	}
}

func bumpPatch(v string) (string, error) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("version %q is not semver (expected X.Y.Z)", v)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("patch component of %q is not an integer", v)
	}
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1), nil
}

func printSnippet(repoURL, portName, baseline string) {
	snippet, _ := json.MarshalIndent(map[string]any{
		"registries": []map[string]any{{
			"kind":       "git",
			"repository": repoURL,
			"baseline":   baseline,
			"packages":   []string{portName},
		}},
	}, "", "  ")
	fmt.Printf("\n✓ done — paste into vcpkg-configuration.json:\n\n%s\n", snippet)
}