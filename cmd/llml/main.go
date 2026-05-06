package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flyingnobita/llml/internal/profiles"
	"github.com/flyingnobita/llml/internal/tui"
	"github.com/flyingnobita/llml/internal/userdata"
)

// version is injected at link time by GoReleaser (-X main.version=...).
var version = "dev"

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-version", "--version", "-v":
			fmt.Println(version)
			return
		}
	}
	if len(os.Args) >= 2 && os.Args[1] == "export" {
		runExport(os.Args[2:])
		return
	}
	if err := userdata.MaybeBackupOnVersionChange(version); err != nil {
		fmt.Fprintf(os.Stderr, "llml: warning: config backup: %v\n", err)
	}
	if err := tui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func runExport(args []string) {
	fs := flag.NewFlagSet("llml export", flag.ExitOnError)
	modelFilter := fs.String("model", "", "filter by model key (case-insensitive substring)")
	profileFilter := fs.String("profile", "", "filter by profile name (case-insensitive substring)")
	outputPath := fs.String("output", profiles.DefaultExportFilename(), "output file path")
	force := fs.Bool("force", false, "overwrite without prompting")
	all := fs.Bool("all", false, "export all profiles (default when no filters given)")
	fs.Parse(args)

	allProfiles, err := profiles.AllToPortable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "llml export: error reading profiles: %v\n", err)
		os.Exit(1)
	}

	var filtered []profiles.PortableProfile
	hasModel := *modelFilter != ""
	hasProfile := *profileFilter != ""
	useAll := *all || (!hasModel && !hasProfile)

	for _, p := range allProfiles {
		if useAll {
			filtered = append(filtered, p)
			continue
		}
		modelMatch := !hasModel || strings.Contains(strings.ToLower(p.ModelHint), strings.ToLower(*modelFilter))
		profileMatch := !hasProfile || strings.Contains(strings.ToLower(p.Name), strings.ToLower(*profileFilter))
		if modelMatch && profileMatch {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) == 0 {
		fmt.Println("No profiles to export.")
		os.Exit(0)
	}

	dest := *outputPath
	if !filepath.IsAbs(dest) {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "llml export: %v\n", err)
			os.Exit(1)
		}
		dest = filepath.Join(cwd, dest)
	}

	if err := profiles.WritePortable(dest, filtered, *force); err != nil {
		fmt.Fprintf(os.Stderr, "llml export: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Exported %d profiles to %s\n", len(filtered), dest)
}
