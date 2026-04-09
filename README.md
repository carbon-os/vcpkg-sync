# vcpkg-sync

A CLI tool that keeps a [vcpkg git registry](https://learn.microsoft.com/en-us/vcpkg/maintainers/registries) in sync with the latest upstream commit of any port.

It handles everything in one command — bumping the version, updating `portfile.cmake`, `vcpkg.json`, `baseline.json`, and the version manifest, then committing and pushing.

---

## How it works

1. Resolves the latest `HEAD` commit hash from the upstream source repository
2. Bumps the patch version (`X.Y.Z → X.Y.Z+1`) from the existing version manifest
3. Updates `portfile.cmake` — only the `REF` line, everything else is preserved
4. Updates `vcpkg.json` — only the `version` field, all dependencies are preserved
5. Updates `versions/baseline.json`
6. Commits the port files, then derives the vcpkg port tree hash from that commit
7. Prepends the new entry to `versions/<x>-/<port>.json`
8. Commits the version manifest
9. Pushes to origin
10. Prints a ready-to-use `vcpkg-configuration.json` snippet

---

## Installation

### Using `go install` (recommended)

If you have Go 1.22+ installed, this installs the binary directly into your `$GOPATH/bin`:

```bash
go install github.com/carbon-os/vcpkg-sync@latest
```

Make sure `$GOPATH/bin` is in your `PATH` — see the platform sections below.

---

### Build from source

```bash
git clone https://github.com/carbon-os/vcpkg-sync
cd vcpkg-sync
go build -o vcpkg-sync .
```

Then follow the platform instructions below to make it globally available.

---

### Linux

Move the binary to `/usr/local/bin`:

```bash
sudo mv vcpkg-sync /usr/local/bin/
```

Or if you prefer a user-local install (no sudo):

```bash
mkdir -p ~/.local/bin
mv vcpkg-sync ~/.local/bin/
```

Then add it to your `PATH` in `~/.bashrc` or `~/.zshrc`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Reload your shell:

```bash
source ~/.bashrc   # or ~/.zshrc
```

Verify:

```bash
vcpkg-sync -h
```

---

### macOS

Move the binary to `/usr/local/bin`:

```bash
sudo mv vcpkg-sync /usr/local/bin/
```

Or using a user-local install:

```bash
mkdir -p ~/.local/bin
mv vcpkg-sync ~/.local/bin/
```

Add to your `PATH` in `~/.zshrc` (zsh is the default shell on macOS):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Reload:

```bash
source ~/.zshrc
```

> If you used `go install`, Go binaries land in `$(go env GOPATH)/bin` — typically
> `~/go/bin`. Add that to your PATH instead:
> ```bash
> export PATH="$HOME/go/bin:$PATH"
> ```

Verify:

```bash
vcpkg-sync -h
```

---

### Windows

#### Option A — copy to a directory already on your PATH

```powershell
# Run in an elevated (Administrator) PowerShell
Move-Item .\vcpkg-sync.exe "C:\Windows\System32\vcpkg-sync.exe"
```

#### Option B — add a dedicated tools directory to PATH (recommended)

```powershell
# Create a tools directory
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\bin"

# Move the binary there
Move-Item .\vcpkg-sync.exe "$env:USERPROFILE\bin\vcpkg-sync.exe"

# Add it to your user PATH permanently
[Environment]::SetEnvironmentVariable(
    "PATH",
    "$env:USERPROFILE\bin;" + [Environment]::GetEnvironmentVariable("PATH", "User"),
    "User"
)
```

Restart your terminal, then verify:

```powershell
vcpkg-sync -h
```

> If you used `go install`, the binary lands in `%GOPATH%\bin` (typically
> `%USERPROFILE%\go\bin`). Add that to your PATH via **System Properties →
> Environment Variables** or with the PowerShell snippet above, replacing
> `$env:USERPROFILE\bin` with `$env:USERPROFILE\go\bin`.

---

## Usage

```
vcpkg-sync [flags] [registry-dir]
```

`registry-dir` defaults to `.` — so in most cases you just `cd` into your registry and run:

```bash
vcpkg-sync
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-port` | auto-detected | Port to sync. Required when the registry contains more than one port. |
| `-source` | parsed from portfile | Upstream git URL. Parsed from `portfile.cmake` automatically if omitted. |
| `-dry-run` | `false` | Print planned changes without modifying or committing anything. |
| `-no-push` | `false` | Commit locally but skip `git push`. |
| `-verbose` | `false` | Show git operations as they run. |

---

## Examples

```bash
# Run from inside the registry repo — auto-detects everything
vcpkg-sync

# Point at a registry directory explicitly
vcpkg-sync ~/projects/my-registry

# Specify port when the registry contains more than one
vcpkg-sync -port mylib

# Preview what would change without touching anything
vcpkg-sync -dry-run

# Commit locally, let your CI pipeline handle the push
vcpkg-sync -no-push

# Full example
vcpkg-sync -port mylib -verbose ~/projects/my-registry
```

---

## Registry layout

`vcpkg-sync` expects a standard vcpkg git registry structure:

```
my-registry/
├── ports/
│   └── <port>/
│       ├── portfile.cmake
│       └── vcpkg.json
└── versions/
    ├── baseline.json
    └── <x>-/
        └── <port>.json
```

The port name and source URL are both discovered automatically at runtime — there is nothing to configure.

---

## Authentication

`vcpkg-sync` uses [go-git](https://github.com/go-git/go-git) for all git operations. For pushing, it will attempt to use your SSH agent automatically. Make sure your agent is running:

```bash
eval $(ssh-agent)
ssh-add ~/.ssh/id_ed25519
```

HTTPS credential helpers configured in your global `~/.gitconfig` are also respected.

---

## Requirements

- Go 1.22+
- Git identity configured (`user.name` and `user.email`) — falls back to `vcpkg-sync` if not set
- SSH agent or HTTPS credentials for push access to your registry

---

## License

MIT