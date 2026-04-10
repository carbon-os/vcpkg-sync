package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func run(cfg Config) error {
	// 1. Resolve port name from -port flag or auto-detect
	portName, err := resolvePortName(cfg)
	if err != nil {
		return err
	}

	portfilePath := portfileAbs(cfg.RegistryDir, portName)

	// 2. Detect whether this is a first-time bootstrap
	_, statErr := os.Stat(portfilePath)
	isNew := os.IsNotExist(statErr)
	if statErr != nil && !isNew {
		return fmt.Errorf("stat portfile: %w", statErr)
	}

	// 3. Resolve source URL — flag takes priority, then parse from portfile
	sourceURL := cfg.SourceURL
	if !isNew && sourceURL == "" {
		raw, err := os.ReadFile(portfilePath)
		if err != nil {
			return fmt.Errorf("read portfile: %w", err)
		}
		var ok bool
		sourceURL, ok = extractPortfileURL(string(raw))
		if !ok {
			return fmt.Errorf(
				"cannot parse URL from %s; provide it with -source",
				portfilePath,
			)
		}
	}
	if sourceURL == "" {
		return fmt.Errorf("new port %q: provide upstream URL with -source", portName)
	}

	// 4. Fetch latest commit from upstream
	fmt.Printf("→ resolving HEAD of %s\n", sourceURL)
	newRef, err := resolveRemoteHEAD(sourceURL)
	if err != nil {
		return err
	}
	fmt.Printf("  ref: %s\n", newRef)

	// 5. Determine old and new version
	var currentVer, newVer string
	if isNew {
		currentVer = "(none)"
		newVer = "0.0.1"
	} else {
		versionPath := versionFileAbs(cfg.RegistryDir, portName)
		var manifest VersionManifest
		if err := readJSON(versionPath, &manifest); err != nil {
			return err
		}
		if len(manifest.Versions) == 0 {
			return fmt.Errorf("no versions found in %s", versionPath)
		}
		currentVer = manifest.Versions[0].Version
		newVer, err = bumpPatch(currentVer)
		if err != nil {
			return err
		}
	}

	fmt.Printf("→ port:    %s\n", portName)
	fmt.Printf("→ version: %s → %s\n", currentVer, newVer)

	if cfg.DryRun {
		fmt.Println("\n(dry run — no changes made)")
		return nil
	}

	// 6. Write port files
	fmt.Println("→ writing port files")
	if isNew {
		// Create directory tree
		if err := ensureDir(filepath.Dir(portfilePath)); err != nil {
			return err
		}
		if err := ensureDir(filepath.Dir(versionFileAbs(cfg.RegistryDir, portName))); err != nil {
			return err
		}

		// Scaffold portfile.cmake and vcpkg.json from templates
		if err := writeText(portfilePath, newPortfile(sourceURL, newRef, portName)); err != nil {
			return err
		}
		if err := writeText(vcpkgJSONAbs(cfg.RegistryDir, portName), newVcpkgJSON(portName, newVer)); err != nil {
			return err
		}
	} else {
		// Update only the REF line; leave everything else intact
		raw, err := os.ReadFile(portfilePath)
		if err != nil {
			return fmt.Errorf("read portfile: %w", err)
		}
		if err := writeText(portfilePath, updatePortfileRef(string(raw), newRef)); err != nil {
			return err
		}
		if err := patchJSONField(vcpkgJSONAbs(cfg.RegistryDir, portName), "version", newVer); err != nil {
			return err
		}
	}

	// Ensure vcpkg-cmake host deps are present regardless of how the port was created.
	fmt.Println("→ ensuring host dependencies")
	if err := ensureHostDeps(vcpkgJSONAbs(cfg.RegistryDir, portName)); err != nil {
		return err
	}

	// 7. Update baseline.json
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

	// 8. Commit port files + baseline — we derive the tree hash from this commit
	fmt.Println("→ committing port files")
	portCommit, err := stageAndCommit(
		cfg.RegistryDir,
		[]string{portfileRel(portName), vcpkgJSONRel(portName), baselineRel()},
		fmt.Sprintf("chore(vcpkg-sync): update port %s → %s", portName, newVer),
	)
	if err != nil {
		return err
	}

	// 9. Derive the port directory tree hash from that commit
	fmt.Println("→ deriving port tree hash")
	treeHash, err := subtreeHash(cfg.RegistryDir, portCommit, portName)
	if err != nil {
		return err
	}
	fmt.Printf("  tree: %s\n", treeHash)

	// 10. Update version manifest
	fmt.Println("→ updating version manifest")
	var manifest VersionManifest
	if !isNew {
		if err := readJSON(versionFileAbs(cfg.RegistryDir, portName), &manifest); err != nil {
			return err
		}
	}
	manifest.Versions = append(
		[]VersionEntry{{Version: newVer, GitTree: treeHash}},
		manifest.Versions...,
	)
	if err := writeJSON(versionFileAbs(cfg.RegistryDir, portName), manifest); err != nil {
		return err
	}

	// 11. Commit version manifest separately
	if _, err := stageAndCommit(
		cfg.RegistryDir,
		[]string{versionFileRel(portName)},
		fmt.Sprintf("chore(vcpkg-sync): version manifest %s → %s", portName, newVer),
	); err != nil {
		return err
	}

	// 12. Push
	if !cfg.NoPush {
		fmt.Println("→ pushing")
		if err := push(cfg.RegistryDir, cfg.Verbose); err != nil {
			return err
		}
	}

	// 13. Resolve the registry's own remote URL and print the config snippet
	registryURL, err := registryRemoteURL(cfg.RegistryDir)
	if err != nil {
		return err
	}
	head, err := headCommitHash(cfg.RegistryDir)
	if err != nil {
		return err
	}
	printSnippet(registryURL, portName, head)

	return nil
}

func resolvePortName(cfg Config) (string, error) {
	if cfg.Port != "" {
		return cfg.Port, nil
	}

	entries, err := os.ReadDir(portsDirAbs(cfg.RegistryDir))
	if err != nil {
		if os.IsNotExist(err) {
			// Registry not yet initialised — derive the port name from the
			// directory name (e.g. "../fetch" → "fetch").
			name := filepath.Base(cfg.RegistryDir)
			fmt.Printf("→ no ports directory found; using %q as port name\n", name)
			return name, nil
		}
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
		// Directory exists but is empty — same fallback as above.
		name := filepath.Base(cfg.RegistryDir)
		fmt.Printf("→ ports directory is empty; using %q as port name\n", name)
		return name, nil
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

func printSnippet(registryURL, portName, baseline string) {
	snippet, _ := json.MarshalIndent(map[string]any{
		"registries": []map[string]any{{
			"kind":       "git",
			"repository": registryURL,
			"baseline":   baseline,
			"packages":   []string{portName},
		}},
	}, "", "  ")
	fmt.Printf("\n✓ done — paste into vcpkg-configuration.json:\n\n%s\n", snippet)
}