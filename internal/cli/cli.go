// Package cli wires Cobra commands to the app, TUI and headless renderer.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/yufeiminds/nameary/internal/app"
	"github.com/yufeiminds/nameary/internal/cache"
	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/check/domain"
	"github.com/yufeiminds/nameary/internal/config"
	"github.com/yufeiminds/nameary/internal/render"
	"github.com/yufeiminds/nameary/internal/tui"
	"github.com/yufeiminds/nameary/internal/version"
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("exit %d", e.code)
	}
	return e.err.Error()
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	root := newRoot()
	err := root.Execute()
	if err == nil {
		return 0
	}
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.err != nil {
			fmt.Fprintln(os.Stderr, "nameary:", ee.err)
		}
		return ee.code
	}
	fmt.Fprintln(os.Stderr, "nameary:", err)
	return 1
}

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "nameary [name]",
		Short: "Find a name before you commit",
		Long:  "nameary checks a name across domains, package registries and handles, streaming each result as it arrives.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return argError(args, "accepts at most 1 name")
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return run(cmd, name)
		},
	}
	root.Version = versionString()
	root.SetVersionTemplate("{{.Version}}\n")
	addFlags(root)

	checkCmd := &cobra.Command{
		Use:   "check <name>",
		Short: "Check one name (script-friendly)",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return argError(args, "check needs exactly one name")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args[0])
		},
	}
	addFlags(checkCmd)
	root.AddCommand(checkCmd)

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(versionString())
			return nil
		},
	})

	root.AddCommand(newConfigCmd())
	root.AddCommand(newCacheCmd())
	return root
}

func argError(args []string, base string) error {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return &exitError{code: 2, err: fmt.Errorf("%s; flags must come before the name, and '--' is not a nameary argument separator (try: go run ./cmd/nameary acme)", base)}
		}
	}
	return &exitError{code: 2, err: fmt.Errorf("%s, received %d", base, len(args))}
}

func addFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String("preset", "", "preset: all (default), indie, web, go, rust, jvm, apple, linux, data")
	f.StringSlice("targets", nil, "comma-separated targets or all (domains, npm, pypi, crates, rubygems, nuget, dockerhub, homebrew, packagist, hex, pub, github, maven, conda, cocoapods, aur, snap, chocolatey, hackage, jsr)")
	f.StringSlice("tlds", nil, "comma-separated TLDs (com, io, dev, app, ai, co...)")
	f.Int("concurrency", 0, "global concurrent checks (default 16)")
	f.Duration("timeout", 0, "override the per-request timeout (default 3s)")
	f.String("format", "auto", "output format: auto, tui, console, table, json, ndjson")
	f.String("style", "auto", "appearance: auto, plain, ascii, mono, calm (comma-separated)")
	f.Bool("only-available", false, "only show available results")
	f.String("cache-mode", "auto", "cache behavior: auto, off, refresh")
	f.String("cache", "", "cache database path")
	f.Duration("cache-ttl", 0, "override the cache TTL (default is per-target)")
	f.Bool("celebrate", false, "show a small celebration when everything is available")
	f.Bool("fail-if-taken", false, "exit 3 when any result is taken")
	f.String("github-token", "", "GitHub token (prefer GITHUB_TOKEN / GH_TOKEN over shell history)")
	f.Bool("debug", false, "write debug logs to the cache directory")
}

type runOptions struct {
	app.Options
	format        string
	onlyAvailable bool
	failIfTaken   bool
}

func resolveOptions(cmd *cobra.Command) (runOptions, error) {
	cfg, err := config.Load()
	if err != nil {
		return runOptions{}, err
	}
	f := cmd.Flags()

	presetName, _ := f.GetString("preset")
	if presetName == "" {
		presetName = cfg.Preset
	}
	preset, ok := config.Presets[presetName]
	if !ok {
		return runOptions{}, fmt.Errorf("unknown preset %q", presetName)
	}

	targets := append([]string(nil), cfg.Targets...)
	if f.Changed("targets") {
		targets, _ = f.GetStringSlice("targets")
	} else if f.Changed("preset") {
		targets = append([]string(nil), preset.Targets...)
	}
	tlds := append([]string(nil), cfg.TLDs...)
	if f.Changed("tlds") {
		tlds, _ = f.GetStringSlice("tlds")
	} else if f.Changed("preset") {
		tlds = append([]string(nil), preset.TLDs...)
	}

	concurrency, _ := f.GetInt("concurrency")
	if concurrency <= 0 {
		concurrency = cfg.Concurrency
	}
	timeout, _ := f.GetDuration("timeout")
	if timeout <= 0 {
		timeout = config.ParseDuration(cfg.Timeout, 0)
	}
	format, _ := f.GetString("format")
	style, _ := f.GetString("style")

	noColor := os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0" || !term.IsTerminal(os.Stdout.Fd())
	ascii := false
	reduced := os.Getenv("NAMEARY_REDUCED_MOTION") == "1"
	for _, part := range strings.Split(strings.ToLower(strings.TrimSpace(style)), ",") {
		switch part {
		case "", "auto", "color":
		case "plain", "accessible":
			ascii, noColor, reduced = true, true, true
		case "ascii":
			ascii = true
		case "mono", "no-color":
			noColor = true
		case "calm", "no-motion", "reduced-motion":
			reduced = true
		default:
			return runOptions{}, fmt.Errorf("unknown style %q (use auto, plain, ascii, mono or calm)", part)
		}
	}

	cacheMode, _ := f.GetString("cache-mode")
	refresh, noCache := false, false
	switch strings.ToLower(strings.TrimSpace(cacheMode)) {
	case "", "auto":
	case "off", "none", "disable":
		noCache = true
	case "refresh", "reload":
		refresh = true
	default:
		return runOptions{}, fmt.Errorf("unknown cache mode %q (use auto, off or refresh)", cacheMode)
	}

	onlyAvailable, _ := f.GetBool("only-available")
	cachePath, _ := f.GetString("cache")
	cacheTTL, _ := f.GetDuration("cache-ttl")
	if cacheTTL <= 0 {
		cacheTTL = config.ParseDuration(cfg.CacheTTL, 0)
	}
	celebrate, _ := f.GetBool("celebrate")
	failIfTaken, _ := f.GetBool("fail-if-taken")
	token, _ := f.GetString("github-token")
	if token == "" {
		envName := cfg.Github.TokenEnv
		if envName == "" {
			envName = "GITHUB_TOKEN"
		}
		token = os.Getenv(envName)
		if token == "" {
			token = os.Getenv("GH_TOKEN")
		}
	}
	token = strings.TrimSpace(token)
	if strings.ContainsAny(token, "\r\n\x00") {
		return runOptions{}, fmt.Errorf("GitHub token contains invalid control characters")
	}

	return runOptions{
		Options: app.Options{
			Targets:       targets,
			TLDs:          tlds,
			Concurrency:   concurrency,
			Timeout:       timeout,
			CachePath:     cachePath,
			CacheTTL:      cacheTTL,
			NoCache:       noCache,
			Refresh:       refresh,
			GithubToken:   token,
			ReducedMotion: reduced,
			ASCII:         ascii,
			NoColor:       noColor,
			Celebrate:     celebrate,
		},
		format:        format,
		onlyAvailable: onlyAvailable,
		failIfTaken:   failIfTaken,
	}, nil
}

func run(cmd *cobra.Command, name string) error {
	opts, err := resolveOptions(cmd)
	if err != nil {
		return err
	}
	if debugFlag, _ := cmd.Flags().GetBool("debug"); debugFlag {
		setupDebug()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// A full domain argument means "check this domain", not "check a package
	// with dots in it". Explicit --targets / --preset still wins.
	if name != "" && domain.Valid(name) && !cmd.Flags().Changed("targets") && !cmd.Flags().Changed("preset") {
		opts.Targets = []string{string(check.TargetDomains)}
	}

	tty := isTTY()
	stdoutTTY := term.IsTerminal(os.Stdout.Fd())
	if os.Getenv("TERM") == "dumb" {
		tty = false
		stdoutTTY = false
		opts.NoColor = true
	}

	format := opts.format
	if format == "" {
		format = "auto"
	}
	useTUI := false
	if format == "tui" {
		useTUI = true
		format = "auto"
	} else if format == "auto" {
		useTUI = tty
	}
	if useTUI && !tty {
		return &exitError{code: 2, err: fmt.Errorf("the TUI needs a TTY; use --format console or --format table")}
	}
	if format == "auto" && !useTUI {
		if stdoutTTY {
			format = "console"
		} else {
			format = "table"
		}
	}
	if !useTUI && format != "console" && format != "table" && format != "json" && format != "ndjson" {
		return &exitError{code: 2, err: fmt.Errorf("unknown format %q (use auto, tui, console, table, json or ndjson)", format)}
	}

	if name == "" && !useTUI {
		if stdoutTTY && format == "console" {
			_ = cmd.Help()
			return nil
		}
		return &exitError{code: 2, err: fmt.Errorf("a name is required (for example: nameary acme)")}
	}

	opts.Options.Name = name
	session, err := app.Prepare(runCtx, opts.Options)
	if err != nil {
		return err
	}
	defer func() {
		cancel()
		session.Close()
	}()

	if useTUI {
		res, err := tui.Run(runCtx, tui.Options{
			InitialName:   name,
			Targets:       opts.Targets,
			TLDs:          opts.TLDs,
			Engine:        session.Engine,
			Plan:          session.Plan,
			ReducedMotion: opts.ReducedMotion,
			ASCII:         opts.ASCII,
			NoColor:       opts.NoColor,
			Celebrate:     opts.Celebrate,
		})
		if err != nil {
			return err
		}
		if opts.failIfTaken && res.Counts[check.StatusTaken] > 0 {
			return &exitError{code: 3, err: fmt.Errorf("%d name(s) already taken", res.Counts[check.StatusTaken])}
		}
		return nil
	}

	events := session.Engine.Run(runCtx, 1, session.Jobs)
	var res render.Result
	if format == "console" {
		width, height := 100, 40
		if w, h, werr := term.GetSize(os.Stdout.Fd()); werr == nil && w > 0 && h > 0 {
			width, height = w, h
		}
		light := false
		if stdoutTTY && term.IsTerminal(os.Stdin.Fd()) {
			light = !lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
		}
		res, err = render.RunConsole(events, session.Jobs, render.ConsoleOptions{
			OnlyAvailable: opts.onlyAvailable,
			NoColor:       opts.NoColor,
			ASCII:         opts.ASCII,
			Light:         light,
			Celebrate:     opts.Celebrate,
			Width:         width,
			Height:        height,
			Live:          stdoutTTY && !opts.ReducedMotion,
			Out:           os.Stdout,
		})
	} else {
		res, err = render.Run(events, session.Jobs, render.Options{
			Format:        format,
			OnlyAvailable: opts.onlyAvailable,
			NoColor:       opts.NoColor,
			ASCII:         opts.ASCII,
			Out:           os.Stdout,
			Err:           os.Stderr,
		})
	}
	if err != nil {
		return err
	}
	if opts.failIfTaken && res.Counts[check.StatusTaken] > 0 {
		return &exitError{code: 3, err: fmt.Errorf("%d name(s) already taken", res.Counts[check.StatusTaken])}
	}
	return nil
}

func isTTY() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
}

func setupDebug() {
	dir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	dir = filepath.Join(dir, "nameary")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	// #nosec G304 -- the log path is under the user cache directory.
	f, err := os.OpenFile(filepath.Join(dir, "debug.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_ = f.Chmod(0o600)
	log.SetOutput(f)
	log.SetLevel(log.DebugLevel)
	log.Debug("nameary starting", "pid", os.Getpid())
}

func versionString() string {
	return version.String()
}

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Inspect or clear the result cache",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the cache database path",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), cache.DefaultPath())
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Remove the default cache database",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := cache.DefaultPath()
			if err := os.Remove(path); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					fmt.Fprintln(cmd.OutOrStdout(), "cache is already empty")
					return nil
				}
				return fmt.Errorf("clear cache: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "cleared %s\n", path)
			return nil
		},
	})
	return cmd
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and initialize configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(config.Path())
			return nil
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(config.Path())
			return nil
		},
	})
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create a config file (interactive on a TTY)",
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			preset, _ := cmd.Flags().GetString("preset")
			targets, _ := cmd.Flags().GetStringSlice("targets")
			tlds, _ := cmd.Flags().GetStringSlice("tlds")
			return runConfigInit(force, preset, targets, tlds)
		},
	}
	initCmd.Flags().Bool("force", false, "overwrite an existing config")
	initCmd.Flags().String("preset", "", "preset to use")
	initCmd.Flags().StringSlice("targets", nil, "targets to use")
	initCmd.Flags().StringSlice("tlds", nil, "TLDs to use")
	cmd.AddCommand(initCmd)
	return cmd
}

func runConfigInit(force bool, preset string, targets, tlds []string) error {
	path := config.Path()
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("config already exists at %s (use --force to overwrite)", path)
	}
	cfg := config.Defaults()
	if preset != "" {
		p, ok := config.Presets[preset]
		if !ok {
			return fmt.Errorf("unknown preset %q", preset)
		}
		cfg.Preset = preset
		cfg.Targets = append([]string(nil), p.Targets...)
		cfg.TLDs = append([]string(nil), p.TLDs...)
	}
	if len(targets) > 0 {
		cfg.Targets = targets
	}
	if len(tlds) > 0 {
		cfg.TLDs = tlds
	}

	if isTTY() && preset == "" && len(targets) == 0 && len(tlds) == 0 {
		var chosenPreset string
		var chosenTargets []string
		var tldText string
		chosenPreset = cfg.Preset
		chosenTargets = append([]string(nil), cfg.Targets...)
		tldText = strings.Join(cfg.TLDs, ", ")

		presetOptions := make([]huh.Option[string], 0, len(config.PresetNames()))
		for _, p := range config.PresetNames() {
			presetOptions = append(presetOptions, huh.NewOption(p, p))
		}
		targetOptions := []huh.Option[string]{
			huh.NewOption("domains", "domains"),
			huh.NewOption("npm", "npm"),
			huh.NewOption("PyPI", "pypi"),
			huh.NewOption("crates.io", "crates"),
			huh.NewOption("RubyGems", "rubygems"),
			huh.NewOption("NuGet", "nuget"),
			huh.NewOption("Docker Hub", "dockerhub"),
			huh.NewOption("Homebrew", "homebrew"),
			huh.NewOption("Packagist", "packagist"),
			huh.NewOption("Hex", "hex"),
			huh.NewOption("pub.dev", "pub"),
			huh.NewOption("GitHub", "github"),
			huh.NewOption("Maven Central", "maven"),
			huh.NewOption("Conda", "conda"),
			huh.NewOption("CocoaPods", "cocoapods"),
			huh.NewOption("AUR", "aur"),
			huh.NewOption("Snap Store", "snap"),
			huh.NewOption("Chocolatey", "chocolatey"),
			huh.NewOption("Hackage", "hackage"),
			huh.NewOption("JSR", "jsr"),
		}
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Preset").
					Options(presetOptions...).
					Value(&chosenPreset),
			),
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Targets").
					Options(targetOptions...).
					Value(&chosenTargets),
				huh.NewInput().
					Title("Domains").
					Placeholder("com, io, dev").
					Value(&tldText),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
		cfg.Preset = chosenPreset
		if len(chosenTargets) > 0 {
			cfg.Targets = chosenTargets
		}
		if strings.TrimSpace(tldText) != "" {
			var parsed []string
			for _, t := range strings.Split(tldText, ",") {
				t = strings.TrimSpace(strings.TrimPrefix(t, "."))
				if t != "" {
					parsed = append(parsed, t)
				}
			}
			if len(parsed) > 0 {
				cfg.TLDs = parsed
			}
		}
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", path)
	return nil
}
