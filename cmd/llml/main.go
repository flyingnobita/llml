package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
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

	targetModel := *target

	if targetModel == "" && !isURL {
		fmt.Fprintf(os.Stderr, "llml import: --target is required (local model path to attach profiles to)\n")
		os.Exit(1)
	}

	if isURL {
		if targetModel == "" {
			var pickErr error
			targetModel, pickErr = pickTargetModel(f.Profiles, *rescan)
			if pickErr != nil {
				fmt.Fprintf(os.Stderr, "llml import: %v\n", pickErr)
				os.Exit(1)
			}
		} else {
			if err := validateTargetBackend(targetModel, f.Profiles, *rescan); err != nil {
				fmt.Fprintf(os.Stderr, "llml import: %v\n", err)
				os.Exit(1)
			}
		}

		preview := profiles.FormatPortablePreview(f, profiles.PreviewOpts{TargetModel: targetModel})
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
		if targetModel != "" && pp.ModelHint != "" && modelHintsDiffer(pp.ModelHint, targetModel) {
			targetHint := profiles.ModelHint(targetModel)
			fmt.Fprintf(os.Stderr, "warning: profile %q was created for %q but is being imported to %q\n", p.Name, pp.ModelHint, targetHint)
		}

		imported = append(imported, p)
	}

	result, err := profiles.ImportProfiles(targetModel, imported, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "llml import: %v\n", err)
		os.Exit(1)
	}

	if *activate && len(imported) == 1 {
		if err := profiles.SetActiveProfile(targetModel, imported[0].Name); err != nil {
			fmt.Fprintf(os.Stderr, "llml import: setting active profile: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Imported to %s: %d added", targetModel, result.Added)
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

// pickTargetModel resolves a target model for URL imports. It uses cached discovery
// when available, auto-runs discovery on empty cache (2A-revised), and presents an
// interactive picker filtered by backend compatibility.
func pickTargetModel(portableProfiles []profiles.PortableProfile, rescan bool) (string, error) {
	backends := uniqueBackendsFromPortable(portableProfiles)

	modelFiles, err := resolveModels(rescan)
	if err != nil {
		return "", fmt.Errorf("model discovery failed: %w", err)
	}

	if len(modelFiles) == 0 {
		return "", fmt.Errorf("no local model files found. Download a model first, then retry.")
	}

	compatible := config.FilterByBackend(modelFiles, backends)
	if len(compatible) == 0 {
		discovered := config.ModelBackends(modelFiles)
		return "", fmt.Errorf("no compatible local models for this profile (backend: %s; discovered: %s)",
			strings.Join(backends, ", "), strings.Join(discovered, ", "))
	}

	if !isTerminal() {
		return "", fmt.Errorf("not a terminal and no --target provided; cannot show picker")
	}

	return presentModelPicker(compatible, os.Stdin)
}

// validateTargetBackend checks that the given target model is compatible with the
// profiles' backends. It runs discovery if needed and returns an error when the
// target is found but its backend doesn't match.
func validateTargetBackend(target string, portables []profiles.PortableProfile, rescan bool) error {
	backends := uniqueBackendsFromPortable(portables)

	modelFiles, err := resolveModels(rescan)
	if err != nil {
		return err
	}
	if len(modelFiles) == 0 {
		return nil // target not in cache; let user proceed
	}

	// Find the model matching the target.
	key := profiles.ModelParamsKey(target)
	for _, m := range modelFiles {
		if m.Identity() == key {
			allowed := make(map[models.ModelBackend]bool)
			for _, b := range backends {
				if mb, mbErr := models.ParseBackend(b); mbErr == nil {
					allowed[mb] = true
				}
			}
			if !allowed[m.Backend] {
				discovered := config.ModelBackends(modelFiles)
				return fmt.Errorf("no compatible local models for this profile (backend: %s; discovered: %s)",
					strings.Join(backends, ", "), strings.Join(discovered, ", "))
			}
			return nil
		}
	}
	return nil // target not in discovered models; let user proceed
}

// resolveModels returns cached or freshly-scanned models based on the rescan flag
// and cache freshness.
func resolveModels(rescan bool) ([]models.ModelFile, error) {
	if rescan {
		fmt.Fprintln(os.Stderr, "Scanning local models...")
		return config.RunDiscovery()
	}
	modelFiles, err := config.CachedModels()
	var stale *config.CacheStaleError
	if errors.As(err, &stale) {
		fmt.Fprintf(os.Stderr, "Discovery cache is stale (last scan: %s). Scanning local models...\n",
			stale.LastScan.Format("2006-01-02 15:04:05"))
		return config.RunDiscovery()
	}
	if err == nil && len(modelFiles) == 0 {
		fmt.Fprintln(os.Stderr, "Scanning local models...")
		return config.RunDiscovery()
	}
	return modelFiles, err
}

// uniqueBackendsFromPortable returns deduplicated, normalized backend names from a
// set of portable profiles.
func uniqueBackendsFromPortable(pp []profiles.PortableProfile) []string {
	seen := make(map[string]bool)
	var out []string
	for _, p := range pp {
		b := strings.TrimSpace(p.Backend)
		if b == "" {
			b = "llama"
		}
		if !seen[b] {
			seen[b] = true
			out = append(out, b)
		}
	}
	return out
}

// isTerminal reports whether stdin is a character device (TTY).
// Exposed as a variable so tests can override it.
var isTerminal = func() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// presentModelPicker shows a numbered list of models and reads a selection from r.
func presentModelPicker(models []models.ModelFile, r io.Reader) (string, error) {
	fmt.Fprintf(os.Stderr, "\nCompatible local models:\n")
	for i, m := range models {
		loc := m.DisplayLocation()
		fmt.Fprintf(os.Stderr, "  %d) %s  (%s, %s)\n", i+1, m.Name, m.Backend.String(), loc)
	}
	fmt.Fprintf(os.Stderr, "\nPick a model (1-%d) or q to cancel: ", len(models))

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "q" || input == "Q" {
			return "", fmt.Errorf("cancelled")
		}
		n, convErr := strconv.Atoi(input)
		if convErr != nil || n < 1 || n > len(models) {
			fmt.Fprintf(os.Stderr, "Pick a model (1-%d) or q to cancel: ", len(models))
			continue
		}
		return models[n-1].Identity(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return "", fmt.Errorf("cancelled")
}

// modelHintsDiffer returns true when the profile's model_hint and the target
// model path look like they refer to different models. It normalizes both
// strings and checks whether either is a substring of the other.
func modelHintsDiffer(profileHint, targetPath string) bool {
	targetHint := strings.ToLower(profiles.ModelHint(targetPath))
	ph := strings.ToLower(profileHint)

	// Normalize: replace dashes and underscores with spaces, collapse whitespace.
	normalize := func(s string) string {
		r := strings.NewReplacer("-", " ", "_", " ")
		fields := strings.Fields(r.Replace(s))
		return strings.Join(fields, " ")
	}

	phNorm := normalize(ph)
	tgtNorm := normalize(targetHint)

	return !strings.Contains(phNorm, tgtNorm) && !strings.Contains(tgtNorm, phNorm)
}
