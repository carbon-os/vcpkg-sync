package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("vcpkg-sync: ")

	var cfg Config
	flag.StringVar(&cfg.Port, "port", "", "port to sync (auto-detected when registry contains one port)")
	flag.StringVar(&cfg.SourceURL, "source", "", "upstream git URL (parsed from portfile.cmake if omitted)")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "print planned changes without modifying anything")
	flag.BoolVar(&cfg.NoPush, "no-push", false, "commit locally but skip git push")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "show git operations as they run")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: vcpkg-sync [flags] [registry-dir]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Bumps a vcpkg port to the latest upstream commit and syncs all")
		fmt.Fprintln(os.Stderr, "registry files: portfile.cmake, vcpkg.json, baseline.json, and")
		fmt.Fprintln(os.Stderr, "the version manifest. Works with any git-based vcpkg registry.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  vcpkg-sync                   # run from inside the registry repo")
		fmt.Fprintln(os.Stderr, "  vcpkg-sync ~/my-registry     # point at a registry directory")
		fmt.Fprintln(os.Stderr, "  vcpkg-sync -port mylib       # specify port when multiple exist")
		fmt.Fprintln(os.Stderr, "  vcpkg-sync -dry-run          # preview without touching anything")
		fmt.Fprintln(os.Stderr, "  vcpkg-sync -no-push          # commit locally, push manually")
	}

	flag.Parse()

	registryDir := "."
	if flag.NArg() > 0 {
		registryDir = flag.Arg(0)
	}
	cfg.RegistryDir = registryDir

	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}