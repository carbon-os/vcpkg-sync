package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runGit is a helper to run local git commands and capture stdout.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// resolveRemoteHEAD returns the commit hash HEAD resolves to on the remote.
func resolveRemoteHEAD(url string) (string, error) {
	cmd := exec.Command("git", "ls-remote", url, "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git ls-remote %s: %w", url, err)
	}
	
	// Output format: <hash>\tHEAD\n
	fields := strings.Fields(string(out))
	if len(fields) < 1 {
		return "", fmt.Errorf("cannot resolve HEAD for %s", url)
	}
	return fields[0], nil
}

// stageAndCommit stages relPaths (relative to repo root) and creates a commit.
func stageAndCommit(repoDir string, relPaths []string, message string) (string, error) {
	args := append([]string{"add"}, relPaths...)
	if _, err := runGit(repoDir, args...); err != nil {
		return "", err
	}
	if _, err := runGit(repoDir, "commit", "-m", message); err != nil {
		return "", err
	}
	return headCommitHash(repoDir)
}

// subtreeHash returns the git tree hash of ports/<portName>/ at the given commit.
func subtreeHash(repoDir string, commitHash string, portName string) (string, error) {
	// git rev-parse <commit>:<path> returns the tree hash for a directory
	path := portTreePath(portName)
	return runGit(repoDir, "rev-parse", commitHash+":"+path)
}

// push pushes the current branch to origin.
func push(repoDir string, verbose bool) error {
	cmd := exec.Command("git", "push", "origin")
	cmd.Dir = repoDir
	
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// headCommitHash returns the hex hash of HEAD.
func headCommitHash(repoDir string) (string, error) {
	return runGit(repoDir, "rev-parse", "HEAD")
}