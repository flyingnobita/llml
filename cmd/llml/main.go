package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/flyingnobita/llml/internal/config"
	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
	"github.com/flyingnobita/llml/internal/settings"
	"github.com/flyingnobita/llml/internal/tui"
	"github.com/flyingnobita/llml/internal/userdata"
)

// version is injected at link time by GoReleaser (-X main.version=...).
var version = "dev"

// cli carries the streams and terminal detection every subcommand needs, so no
// command reaches for os.Stdout or os.Exit on its own. main is the only place
// that touches the process.
type cli struct {
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	isTerminal func() bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is main's body with the process boundary passed in, so tests can drive
// every path with fake streams and read the exit code as a value. It returns
// the process exit status; it never calls os.Exit.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	c := cli{stdin: stdin, stdout: stdout, stderr: stderr, isTerminal: stdinIsTerminal}

	if len(args) > 0 {
		switch args[0] {
		case "-h", "-help", "--help", "help":
			printUsage(stdout)
			return 0
		}
	}
	for _, arg := range args {
		switch arg {
		case "-version", "--version", "-v":
			fmt.Fprintln(stdout, version)
			return 0
		}
	}
	if len(args) == 0 {
		return c.runTUI()
	}

	var err error
	switch args[0] {
	case "export":
		err = c.runExport(args[1:])
	case "import":
		err = c.runImport(args[1:])
	default:
		// Anything else falls through to the TUI, which takes no arguments.
		return c.runTUI()
	}
	switch {
	case err == nil:
		return 0
	case errors.Is(err, flag.ErrHelp):
		// The flag set already printed the subcommand's usage.
		return 0
	case errors.Is(err, errCancelled):
		// The user declined a prompt; that is not a failure.
		return 0
	default:
		fmt.Fprintf(stderr, "llml: %v\n", err)
		return 1
	}
}

// errCancelled means the user declined a prompt or the picker. It unwinds the
// command but exits 0.
var errCancelled = errors.New("cancelled")

func printUsage(w io.Writer) {
	fmt.Fprint(w, `llml - terminal UI for discovering and launching local LLMs

Usage:
  llml                      Start the terminal UI
  llml export [flags]       Export parameter profiles to a portable TOML file
  llml import [flags] SRC   Import profiles from a file or an https:// URL
  llml --version            Print the version

Run "llml export --help" or "llml import --help" for subcommand flags.
`)
}

func (c cli) runTUI() int {
	if err := userdata.MaybeBackupOnVersionChange(version); err != nil {
		fmt.Fprintf(c.stderr, "llml: warning: config backup: %v\n", err)
	}
	if err := tui.Run(); err != nil {
		fmt.Fprintf(c.stderr, "%v\n", err)
		return 1
	}
	return 0
}

// newFlagSet returns a flag set that reports errors instead of exiting, so a
// bad flag unwinds through run like any other error.
func (c cli) newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	return fs
}

func (c cli) runExport(args []string) error {
	fs := c.newFlagSet("llml export")
	modelFilter := fs.String("model", "", "filter by model key (case-insensitive substring)")
	profileFilter := fs.String("profile", "", "filter by profile name (case-insensitive substring)")
	outputPath := fs.String("output", profiles.DefaultExportFilename(), "output file path")
	force := fs.Bool("force", false, "overwrite without prompting")
	all := fs.Bool("all", false, "export all profiles (default when no filters given)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	allProfiles, err := profiles.AllToPortable()
	if err != nil {
		return fmt.Errorf("export: reading profiles: %w", err)
	}

	filtered := filterProfilesForExport(allProfiles, *modelFilter, *profileFilter, *all)
	if len(filtered) == 0 {
		fmt.Fprintln(c.stdout, "No profiles to export.")
		return nil
	}

	dest := *outputPath
	if !filepath.IsAbs(dest) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("export: %w", err)
		}
		dest = filepath.Join(cwd, dest)
	}

	if err := profiles.WritePortable(dest, filtered, *force); err != nil {
		return fmt.Errorf("export: %w", err)
	}

	fmt.Fprintf(c.stdout, "Exported %d profiles to %s\n", len(filtered), dest)
	return nil
}

// filterProfilesForExport applies the --model and --profile substring filters.
// With no filters, or with --all, everything is exported.
func filterProfilesForExport(all []profiles.PortableProfile, modelFilter, profileFilter string, forceAll bool) []profiles.PortableProfile {
	hasModel := modelFilter != ""
	hasProfile := profileFilter != ""
	if forceAll || (!hasModel && !hasProfile) {
		return all
	}
	var out []profiles.PortableProfile
	for _, p := range all {
		modelMatch := !hasModel || strings.Contains(strings.ToLower(p.ModelHint), strings.ToLower(modelFilter))
		profileMatch := !hasProfile || strings.Contains(strings.ToLower(p.Name), strings.ToLower(profileFilter))
		if modelMatch && profileMatch {
			out = append(out, p)
		}
	}
	return out
}

// importOpts is the parsed form of the import subcommand's flags.
type importOpts struct {
	target   string
	dryRun   bool
	force    bool
	activate bool
	rescan   bool
	yes      bool
	source   string
	isURL    bool
}

func (c cli) runImport(args []string) error {
	opts, err := c.parseImportArgs(args)
	if err != nil {
		return err
	}

	f, err := c.loadPortable(opts)
	if err != nil {
		return err
	}
	if opts.activate && len(f.Profiles) > 1 {
		return fmt.Errorf("import: --activate requires a single-profile file (got %d profiles)", len(f.Profiles))
	}
	if opts.dryRun {
		fmt.Fprint(c.stdout, profiles.FormatPortablePreview(f, profiles.PreviewOpts{}))
		return nil
	}

	targetModel, err := c.resolveImportTarget(f, opts)
	if err != nil {
		return err
	}
	return c.applyImport(f, targetModel, opts)
}

func (c cli) parseImportArgs(args []string) (importOpts, error) {
	fs := c.newFlagSet("llml import")
	target := fs.String("target", "", "local model path to attach imported profiles to")
	dryRun := fs.Bool("dry-run", false, "parse and show profiles without writing")
	force := fs.Bool("force", false, "overwrite existing profiles with same name")
	activate := fs.Bool("activate", false, "set imported profile as active for the target model")
	rescan := fs.Bool("rescan", false, "force fresh model discovery before picker")
	yes := fs.Bool("yes", false, "skip confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return importOpts{}, err
	}
	if fs.NArg() < 1 {
		fs.Usage()
		return importOpts{}, errors.New("import: a source file or https:// URL is required")
	}
	src := fs.Arg(0)
	return importOpts{
		target:   *target,
		dryRun:   *dryRun,
		force:    *force,
		activate: *activate,
		rescan:   *rescan,
		yes:      *yes,
		source:   src,
		isURL:    strings.HasPrefix(src, "https://"),
	}, nil
}

// loadPortable reads the profile document from a file or a URL.
func (c cli) loadPortable(opts importOpts) (*profiles.PortableFile, error) {
	if !opts.isURL {
		f, err := profiles.ReadPortable(opts.source)
		if err != nil {
			return nil, fmt.Errorf("import: %w", err)
		}
		return f, nil
	}
	// A fetch can hang, so let the user interrupt it.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	f, err := profiles.FetchPortable(ctx, opts.source)
	if err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}
	return f, nil
}

// resolveImportTarget decides which model the profiles attach to, prompting or
// validating as needed, and confirms the import on the URL path.
func (c cli) resolveImportTarget(f *profiles.PortableFile, opts importOpts) (string, error) {
	if !opts.isURL {
		if opts.target == "" {
			return "", errors.New("import: --target is required (local model path to attach profiles to)")
		}
		return opts.target, nil
	}

	targetModel := opts.target
	if targetModel == "" {
		picked, err := c.pickTargetModel(f.Profiles, opts.rescan)
		if err != nil {
			return "", fmt.Errorf("import: %w", err)
		}
		targetModel = picked
	} else if err := c.validateTargetBackend(targetModel, f.Profiles, opts.rescan); err != nil {
		return "", fmt.Errorf("import: %w", err)
	}

	fmt.Fprint(c.stdout, profiles.FormatPortablePreview(f, profiles.PreviewOpts{TargetModel: targetModel}))
	if !opts.yes {
		if err := c.confirmImport(); err != nil {
			return "", err
		}
	}
	return targetModel, nil
}

// confirmImport asks for consent before writing. A declined prompt returns
// errCancelled, which run treats as a clean exit.
func (c cli) confirmImport() error {
	fmt.Fprint(c.stdout, "\nSave this profile? [y/N]: ")
	scanner := bufio.NewScanner(c.stdin)
	response := ""
	if scanner.Scan() {
		response = strings.TrimSpace(strings.ToLower(scanner.Text()))
	}
	if response != "y" && response != "yes" {
		fmt.Fprintln(c.stdout, "Cancelled.")
		return errCancelled
	}
	return nil
}

// applyImport converts, warns, writes, and reports.
func (c cli) applyImport(f *profiles.PortableFile, targetModel string, opts importOpts) error {
	imported := make([]profiles.Profile, 0, len(f.Profiles))
	for _, pp := range f.Profiles {
		p := profiles.PortableToProfile(pp)
		c.warnStrippedParams(pp, p)
		if targetModel != "" && pp.ModelHint != "" && modelHintsDiffer(pp.ModelHint, targetModel) {
			fmt.Fprintf(c.stderr, "warning: profile %q was created for %q but is being imported to %q\n",
				p.Name, pp.ModelHint, profiles.ModelHint(targetModel))
		}
		imported = append(imported, p)
	}

	result, err := profiles.ImportProfiles(targetModel, imported, opts.force)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}
	if opts.activate && len(imported) == 1 {
		if err := profiles.SetActiveProfile(targetModel, imported[0].Name); err != nil {
			return fmt.Errorf("import: setting active profile: %w", err)
		}
	}

	fmt.Fprintf(c.stdout, "Imported to %s: %d added", targetModel, result.Added)
	if result.Replaced > 0 {
		fmt.Fprintf(c.stdout, ", %d replaced", result.Replaced)
	}
	if result.Skipped > 0 {
		fmt.Fprintf(c.stdout, ", %d skipped (name conflict, use --force to overwrite)", result.Skipped)
	}
	fmt.Fprintln(c.stdout)
	return nil
}

// warnStrippedParams reports the model-location parameters import dropped, so
// the user is not surprised that a flag they wrote is missing.
func (c cli) warnStrippedParams(pp profiles.PortableProfile, p profiles.Profile) {
	_, _, droppedEnv, droppedArgs := profiles.StripModelLocationParams(p.Backend, pp.Env, pp.Args)
	for _, d := range droppedEnv {
		fmt.Fprintf(c.stderr, "warning: stripped model-location env %s from profile %q\n", d, p.Name)
	}
	for _, d := range droppedArgs {
		fmt.Fprintf(c.stderr, "warning: stripped model-location arg %s from profile %q\n", d, p.Name)
	}
}

// pickTargetModel resolves a target model for URL imports. It uses cached discovery
// when available, auto-runs discovery on empty cache (2A-revised), and presents an
// interactive picker filtered by backend compatibility.
// Terminal detection and the input stream come from the cli value, so tests can
// drive both branches without a package-level override.
func (c cli) pickTargetModel(portableProfiles []profiles.PortableProfile, rescan bool) (string, error) {
	backends := uniqueBackendsFromPortable(portableProfiles)

	modelFiles, err := c.resolveModels(rescan)
	if err != nil {
		return "", fmt.Errorf("model discovery failed: %w", err)
	}

	if len(modelFiles) == 0 {
		return "", fmt.Errorf("no local model files found: download a model first, then retry")
	}

	compatible := config.FilterByBackend(modelFiles, backends)
	if len(compatible) == 0 {
		discovered := config.ModelBackends(modelFiles)
		return "", fmt.Errorf("no compatible local models for this profile (backend: %s; discovered: %s)",
			strings.Join(backends, ", "), strings.Join(discovered, ", "))
	}

	if !c.isTerminal() {
		return "", errors.New("not a terminal and no --target provided; cannot show picker")
	}

	return c.presentModelPicker(compatible)
}

// validateTargetBackend checks that the given target model is compatible with the
// profiles' backends. It runs discovery if needed and returns an error when the
// target is found but its backend doesn't match.
func (c cli) validateTargetBackend(target string, portables []profiles.PortableProfile, rescan bool) error {
	backends := uniqueBackendsFromPortable(portables)

	modelFiles, err := c.resolveModels(rescan)
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
// and cache freshness. A scan resolves settings from the environment, config.toml,
// and the built-in defaults, in that order of precedence.
func (c cli) resolveModels(rescan bool) ([]models.ModelFile, error) {
	scan := func() ([]models.ModelFile, error) {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		return config.RunDiscovery(ctx, config.Resolve(settings.OSGetenv))
	}
	if rescan {
		fmt.Fprintln(c.stderr, "Scanning local models...")
		return scan()
	}
	modelFiles, err := config.CachedModels()
	var stale *config.CacheStaleError
	if errors.As(err, &stale) {
		fmt.Fprintf(c.stderr, "Discovery cache is stale (last scan: %s). Scanning local models...\n",
			stale.LastScan.Format("2006-01-02 15:04:05"))
		return scan()
	}
	if err == nil && len(modelFiles) == 0 {
		fmt.Fprintln(c.stderr, "Scanning local models...")
		return scan()
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

// stdinIsTerminal reports whether stdin is a character device (TTY).
func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// presentModelPicker shows a numbered list of models and reads a selection from r.
func (c cli) presentModelPicker(candidates []models.ModelFile) (string, error) {
	fmt.Fprintf(c.stderr, "\nCompatible local models:\n")
	for i, m := range candidates {
		fmt.Fprintf(c.stderr, "  %d) %s  (%s, %s)\n", i+1, m.Name, m.Backend.String(), m.DisplayLocation())
	}
	fmt.Fprintf(c.stderr, "\nPick a model (1-%d) or q to cancel: ", len(candidates))

	scanner := bufio.NewScanner(c.stdin)
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "q" || input == "Q" {
			return "", errCancelled
		}
		n, convErr := strconv.Atoi(input)
		if convErr != nil || n < 1 || n > len(candidates) {
			fmt.Fprintf(c.stderr, "Pick a model (1-%d) or q to cancel: ", len(candidates))
			continue
		}
		return candidates[n-1].Identity(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return "", errCancelled
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
