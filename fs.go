package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeText(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// patchJSONField reads a JSON file as a raw map, sets one key, and writes it
// back — preserving all other fields exactly as-is, including unknown ones.
func patchJSONField(path, key string, value any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw[key] = encoded
	return writeJSON(path, raw)
}

func ensureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	return nil
}

// ensureHostDeps reads ports/<name>/vcpkg.json and guarantees that
// vcpkg-cmake and vcpkg-cmake-config are present as host dependencies.
// Safe to call on both new and existing ports — skips entries already present.
func ensureHostDeps(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	// Decode existing dependencies (may be absent, or contain strings or objects).
	var deps []json.RawMessage
	if existing, ok := raw["dependencies"]; ok {
		if err := json.Unmarshal(existing, &deps); err != nil {
			return fmt.Errorf("parse dependencies in %s: %w", path, err)
		}
	}

	required := []struct{ name string }{
		{"vcpkg-cmake"},
		{"vcpkg-cmake-config"},
	}

	for _, req := range required {
		found := false
		for _, dep := range deps {
			// dep is either a JSON string "name" or an object {"name":"...","host":true}
			var s string
			if json.Unmarshal(dep, &s) == nil && s == req.name {
				found = true
				break
			}
			var obj map[string]json.RawMessage
			if json.Unmarshal(dep, &obj) == nil {
				var n string
				if json.Unmarshal(obj["name"], &n) == nil && n == req.name {
					found = true
					break
				}
			}
		}
		if !found {
			entry, _ := json.Marshal(map[string]any{
				"name": req.name,
				"host": true,
			})
			deps = append(deps, entry)
		}
	}

	encoded, err := json.Marshal(deps)
	if err != nil {
		return err
	}
	raw["dependencies"] = encoded
	return writeJSON(path, raw)
}