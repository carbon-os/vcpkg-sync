package main

import (
	"fmt"
	"io"
	"os"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	gossh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
)

// resolveRemoteHEAD returns the commit hash HEAD resolves to on the remote.
// No local clone is required.
func resolveRemoteHEAD(url string) (string, error) {
	rem := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})
	refs, err := rem.List(&gogit.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("ls-remote %s: %w", url, err)
	}

	lookup := make(map[plumbing.ReferenceName]plumbing.Hash, len(refs))
	var headTarget plumbing.ReferenceName

	for _, r := range refs {
		if r.Name() == plumbing.HEAD {
			if r.Type() == plumbing.SymbolicReference {
				headTarget = r.Target()
			} else if !r.Hash().IsZero() {
				return r.Hash().String(), nil // direct HEAD hash
			}
		} else if !r.Hash().IsZero() {
			lookup[r.Name()] = r.Hash()
		}
	}

	// HEAD was symbolic — follow the target
	if headTarget != "" {
		if h, ok := lookup[headTarget]; ok {
			return h.String(), nil
		}
	}

	// Fallback: prefer main, then master
	for _, name := range []plumbing.ReferenceName{"refs/heads/main", "refs/heads/master"} {
		if h, ok := lookup[name]; ok {
			return h.String(), nil
		}
	}

	return "", fmt.Errorf("cannot resolve HEAD for %s", url)
}

// openRepo opens the git repository rooted at path.
func openRepo(path string) (*gogit.Repository, error) {
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("open repo at %s: %w", path, err)
	}
	return repo, nil
}

// repoSignature builds a commit signature from the repo's git config,
// falling back to a vcpkg-sync identity if not set.
func repoSignature(repo *gogit.Repository) (*object.Signature, error) {
	cfg, err := repo.Config()
	if err != nil {
		return nil, fmt.Errorf("read git config: %w", err)
	}
	name, email := cfg.User.Name, cfg.User.Email
	if name == "" {
		name = "vcpkg-sync"
	}
	if email == "" {
		email = "vcpkg-sync@noreply"
	}
	return &object.Signature{Name: name, Email: email, When: time.Now()}, nil
}

// stageAndCommit stages relPaths (relative to repo root) and creates a commit.
func stageAndCommit(repo *gogit.Repository, relPaths []string, message string, sig *object.Signature) (plumbing.Hash, error) {
	w, err := repo.Worktree()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	for _, p := range relPaths {
		if _, err := w.Add(p); err != nil {
			return plumbing.ZeroHash, fmt.Errorf("git add %s: %w", p, err)
		}
	}
	h, err := w.Commit(message, &gogit.CommitOptions{
		Author:    sig,
		Committer: sig,
	})
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("git commit: %w", err)
	}
	return h, nil
}

// subtreeHash returns the git tree hash of ports/<portName>/ at the given commit.
// This is the value vcpkg stores in the version manifest.
func subtreeHash(repo *gogit.Repository, commitHash plumbing.Hash, portName string) (string, error) {
	commit, err := repo.CommitObject(commitHash)
	if err != nil {
		return "", fmt.Errorf("resolve commit %s: %w", commitHash, err)
	}
	root, err := commit.Tree()
	if err != nil {
		return "", fmt.Errorf("get commit tree: %w", err)
	}
	sub, err := root.Tree(portTreePath(portName))
	if err != nil {
		return "", fmt.Errorf("find port subtree %s: %w", portTreePath(portName), err)
	}
	return sub.Hash.String(), nil
}

// push pushes to origin, retrying with SSH agent auth on failure.
func push(repo *gogit.Repository, verbose bool) error {
	var progress io.Writer
	if verbose {
		progress = os.Stdout
	}
	opts := &gogit.PushOptions{
		RemoteName: "origin",
		Progress:   progress,
	}

	err := repo.Push(opts)
	if err == nil || err == gogit.NoErrAlreadyUpToDate {
		return nil
	}

	// Retry with SSH agent
	if auth, authErr := gossh.NewSSHAgentAuth("git"); authErr == nil {
		opts.Auth = auth
		err = repo.Push(opts)
		if err == nil || err == gogit.NoErrAlreadyUpToDate {
			return nil
		}
	}

	return fmt.Errorf("git push: %w\nhint: ensure your SSH agent is running or credentials are configured", err)
}

// headCommitHash returns the hex hash of HEAD.
func headCommitHash(repo *gogit.Repository) (string, error) {
	ref, err := repo.Head()
	if err != nil {
		return "", err
	}
	return ref.Hash().String(), nil
}