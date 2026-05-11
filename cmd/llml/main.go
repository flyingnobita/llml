package main

import (
	"context"
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
	if len(os.Args) >= 2 && os.Args[1] == "import" {
		runImport(os.Args[2:])
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

func runImport(args []string) {
	fs := flag.NewFlagSet("llml import", flag.ExitOnError)
	target := fs.String("target", "", "local model path to attach imported profiles to")
	dryRun := fs.Bool("dry-run", false, "parse and show profiles without writing")
	force := fs.Bool("force", false, "overwrite existing profiles with same name")
	activate := fs.Bool("activate", false, "set imported profile as active for the target model")
	rescan := fs.Bool("rescan", false, "force fresh model discovery before picker")
	yes := fs.Bool("yes", false, "skip confirmation prompt")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: llml import [flags] <file.toml|https://...>\n")
		os.Exit(1)
	}
	arg := fs.Arg(0)

	isURL := strings.HasPrefix(arg, "https://")

	var f *profiles.PortableFile
	var err error

	if isURL {
		f, err = profiles.FetchPortable(fsParseCtx(), arg)
	} else {
		f, err = profiles.ReadPortable(arg)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "llml import: %v\n", err)
		os.Exit(1)
	}

	if *activate && len(f.Profiles) > 1 {
		fmt.Fprintf(os.Stderr, "llml import: --activate requires a single-profile file (got %d profiles)\n", len(f.Profiles))
		os.Exit(1)
	}

	if *dryRun {
		fmt.Print(profiles.FormatPortablePreview(f, profiles.PreviewOpts{}))
		return
	}

	if *target == "" && !isURL {
		fmt.Fprintf(os.Stderr, "llml import: --target is required (local model path to attach profiles to)\n")
		os.Exit(1)
	}

	if isURL {
		// TODO: picker in step D — for now require --target for URL path.
		if *target == "" {
			fmt.Fprintf(os.Stderr, "llml import: --target is required when importing from a URL\n")
			os.Exit(1)
		}

		// Show preview with target model line
		preview := profiles.FormatPortablePreview(f, profiles.PreviewOpts{TargetModel: *target})
		fmt.Print(preview)

		if !*yes {
			fmt.Print("\nSave this profile? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Cancelled.")
				os.Exit(0)
			}
		}
	}

	_ = rescan // wired in step D

	var imported []profiles.Profile
	for _, pp := range f.Profiles {
		p := profiles.PortableToProfile(pp)
		if _, _, droppedEnv, droppedArgs := profiles.StripModelLocationParams(p.Backend,
			pp.Env, pp.Args); len(droppedEnv) > 0 || len(droppedArgs) > 0 {
			for _, d := range droppedEnv {
				fmt.Fprintf(os.Stderr, "warning: stripped model-location env %s from profile %q\n", d, p.Name)
			}
			for _, d := range droppedArgs {
				fmt.Fprintf(os.Stderr, "warning: stripped model-location arg %s from profile %q\n", d, p.Name)
			}
		}
		imported = append(imported, p)
	}

	result, err := profiles.ImportProfiles(*target, imported, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "llml import: %v\n", err)
		os.Exit(1)
	}

	if *activate && len(imported) == 1 {
		// TODO: set active in step D — placeholder
		_ = activate
	}

	fmt.Printf("Imported to %s: %d added", *target, result.Added)
	if result.Replaced > 0 {
		fmt.Printf(", %d replaced", result.Replaced)
	}
	if result.Skipped > 0 {
		fmt.Printf(", %d skipped (name conflict, use --force to overwrite)", result.Skipped)
	}
	fmt.Println()
}

// fsParseCtx returns a context for use during flag-set parsing / fetch operations.
func fsParseCtx() context.Context {
	return context.Background()
}
