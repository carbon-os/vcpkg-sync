package main

import (
	"fmt"
	"regexp"
)

var (
	refLineRe = regexp.MustCompile(`(?m)^(\s+REF\s+)\S+`)
	urlLineRe = regexp.MustCompile(`(?m)^\s+URL\s+(\S+)`)
)

// updatePortfileRef replaces the REF value in existing portfile.cmake content,
// leaving everything else — options, patches, cmake config — untouched.
func updatePortfileRef(content, newRef string) string {
	return refLineRe.ReplaceAllString(content, "${1}"+newRef)
}

// extractPortfileURL parses the URL field from portfile.cmake content.
func extractPortfileURL(content string) (string, bool) {
	m := urlLineRe.FindStringSubmatch(content)
	if len(m) < 2 {
		return "", false
	}
	return m[1], true
}

// newPortfile generates a minimal portfile.cmake from scratch.
// Only used when no portfile exists yet.
func newPortfile(sourceURL, ref, portName string) string {
	return fmt.Sprintf(`vcpkg_from_git(
    OUT_SOURCE_PATH SOURCE_PATH
    URL             %s
    REF             %s
    HEAD_REF        main
)

vcpkg_cmake_configure(
    SOURCE_PATH "${SOURCE_PATH}"
)

vcpkg_cmake_install()

vcpkg_cmake_config_fixup(
    CONFIG_PATH lib/cmake/%s
)

file(REMOVE_RECURSE "${CURRENT_PACKAGES_DIR}/debug/include")

vcpkg_install_copyright(FILE_LIST "${SOURCE_PATH}/LICENSE")
`, sourceURL, ref, portName)
}