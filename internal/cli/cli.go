package cli

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/compose"
	deploymentconfig "github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	apperrors "github.com/Bare-Systems/Bare-Systems-Installer/internal/errors"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/modules"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/output"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/version"
)

const commandName = "bare-systems"

type App struct {
	name  string
	clock output.Clock
}

func New() *App {
	return &App{name: commandName, clock: time.Now}
}

type globalOptions struct {
	help       bool
	json       bool
	quiet      bool
	verbose    bool
	config     string
	envFile    string
	projectDir string
}

func (a *App) Run(args []string, stdout io.Writer, stderr io.Writer) int {
	opts, remaining, ok := a.parseGlobalFlags(args, stderr)
	if !ok {
		return apperrors.ExitUsage
	}

	if len(remaining) == 0 {
		return a.writeHelp(stdout)
	}

	switch remaining[0] {
	case "help":
		return a.writeHelp(stdout)
	case "version":
		return a.writeVersion(stdout, opts)
	case "validate":
		return a.runValidate(opts, stdout, stderr)
	case "config":
		if len(remaining) > 1 && remaining[1] == "render" {
			return a.runConfigRender(opts, stdout, stderr)
		}
		return a.handleNested("config", remaining[1:], opts, stdout, stderr, []string{"render", "diff"})
	case "module":
		return a.handleNested("module", remaining[1:], opts, stdout, stderr, []string{"list", "enable", "disable"})
	case "service":
		return a.handleNested("service", remaining[1:], opts, stdout, stderr, []string{"install", "uninstall"})
	default:
		if slices.Contains(topLevelCommands(), remaining[0]) {
			return a.writeNotImplemented(remaining[0], opts, stdout, stderr)
		}
		return a.writeUsageError(remaining[0], opts, stdout, stderr)
	}
}

func (a *App) parseGlobalFlags(args []string, stderr io.Writer) (globalOptions, []string, bool) {
	var opts globalOptions
	flags := flag.NewFlagSet(a.name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&opts.help, "help", false, "show help")
	flags.BoolVar(&opts.help, "h", false, "show help")
	flags.BoolVar(&opts.json, "json", false, "emit JSON output where supported")
	flags.BoolVar(&opts.quiet, "quiet", false, "suppress non-essential human output")
	flags.BoolVar(&opts.verbose, "verbose", false, "emit more detailed human output")
	flags.StringVar(&opts.config, "config", "", "path to edge.yml")
	flags.StringVar(&opts.envFile, "env-file", "", "path to non-secret Compose interpolation values")
	flags.StringVar(&opts.projectDir, "project-dir", "", "path to the deployment project directory")
	flags.Usage = func() {
		a.writeUsage(stderr)
	}

	if err := flags.Parse(args); err != nil {
		return opts, nil, false
	}

	if opts.quiet && opts.verbose {
		fmt.Fprintln(stderr, "bare-systems: --quiet and --verbose cannot both be set")
		return opts, nil, false
	}

	if opts.help {
		return opts, []string{"help"}, true
	}

	return opts, flags.Args(), true
}

func (a *App) writeHelp(stdout io.Writer) int {
	a.writeUsage(stdout)
	return apperrors.ExitOK
}

func (a *App) writeUsage(w io.Writer) {
	fmt.Fprintf(w, `%s manages Bare Systems edge deployments.

Usage:
  %s [global flags] <command>

Commands:
  help                     Show this help text.
  version                  Print CLI version and build metadata.
  init                     Initialize local deployment files.
  validate                 Validate config and rendered deployment inputs.
  install                  Prepare and install the deployment.
  start                    Start the Compose deployment.
  stop                     Stop the Compose deployment.
  restart                  Restart the Compose deployment.
  status                   Show deployment health summary.
  ps                       List deployment containers.
  logs                     Show deployment logs.
  update                   Update deployment artifacts.
  rollback                 Roll back to a prior artifact set.
  doctor                   Run local diagnostics checks.
  bundle                   Create a redacted diagnostics bundle.
  report                   Send deployment status to the Portal.
  config render            Render canonical Compose config.
  config diff              Compare desired and active config.
  module list              List available modules.
  module enable <name>     Enable a module.
  module disable <name>    Disable a module.
  service install          Install the system service.
  service uninstall        Uninstall the system service.

Global flags:
  --json          Emit JSON output where supported.
  --quiet         Suppress non-essential human output.
  --verbose       Emit more detailed human output.
  --config PATH   Path to edge.yml.
  --env-file PATH
                  Path to non-secret Compose interpolation values.
  --project-dir PATH
                  Path to the deployment project directory.
`, a.name, a.name)
}

func (a *App) writeVersion(stdout io.Writer, opts globalOptions) int {
	info := version.Current()
	if opts.json {
		if err := output.WriteEnvelope(stdout, "version", apperrors.CodeOK, "Version information", info, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitOK
	}

	if opts.quiet {
		fmt.Fprintln(stdout, info.Version)
		return apperrors.ExitOK
	}

	lines := []string{
		fmt.Sprintf("%s %s", a.name, info.Version),
		fmt.Sprintf("commit: %s", info.Commit),
		fmt.Sprintf("date: %s", info.Date),
	}
	fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return apperrors.ExitOK
}

type deploymentContext struct {
	deployment       deploymentconfig.Deployment
	configSource     string
	envSource        string
	env              deploymentconfig.Environment
	validationResult deploymentconfig.ValidationResult
	composeYAML      []byte
	warnings         []string
}

func (a *App) loadDeploymentContext(opts globalOptions) (deploymentContext, error) {
	configLoad, err := deploymentconfig.LoadDeployment(opts.config, opts.config == "")
	if err != nil {
		return deploymentContext{}, err
	}

	deployment := configLoad.Deployment
	if opts.projectDir != "" {
		deployment.Spec.Runtime.ComposeProjectDirectory = opts.projectDir + "/compose"
		deployment.Spec.Storage.Root = opts.projectDir
	}

	envLoad, err := deploymentconfig.LoadEnv(opts.envFile, opts.envFile != "")
	if err != nil {
		return deploymentContext{}, err
	}

	registry := modules.BuiltInRegistry()
	env := deploymentconfig.MergeEnv(deploymentconfig.DerivedEnv(deployment), envLoad.Environment)
	validationResult, err := deploymentconfig.ValidateDeployment(deployment, env, registry)
	if err != nil {
		return deploymentContext{}, err
	}

	composeYAML, err := compose.Render(deployment, registry, env)
	if err != nil {
		return deploymentContext{}, err
	}
	if err := compose.ValidateRendered(composeYAML); err != nil {
		return deploymentContext{}, err
	}

	warnings := append([]string{}, configLoad.Warnings...)
	warnings = append(warnings, envLoad.Warnings...)
	return deploymentContext{
		deployment:       deployment,
		configSource:     configLoad.Source,
		envSource:        envLoad.Source,
		env:              env,
		validationResult: validationResult,
		composeYAML:      composeYAML,
		warnings:         warnings,
	}, nil
}

func (a *App) runValidate(opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	ctx, err := a.loadDeploymentContext(opts)
	if err != nil {
		return a.writeCommandError("validate", apperrors.CodeConfig, err, opts, stdout, stderr)
	}

	data := map[string]any{
		"configSource":   ctx.configSource,
		"envSource":      ctx.envSource,
		"enabledModules": ctx.validationResult.EnabledModules,
		"profiles":       ctx.validationResult.Profiles,
		"envKeys":        ctx.validationResult.EnvKeys,
	}
	if opts.json {
		if err := output.WriteEnvelopeWithWarnings(stdout, "validate", apperrors.CodeOK, "Deployment model valid", data, ctx.warnings, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitOK
	}

	if !opts.quiet {
		fmt.Fprintln(stdout, "Deployment model valid")
		fmt.Fprintf(stdout, "config: %s\n", ctx.configSource)
		fmt.Fprintf(stdout, "profiles: %s\n", strings.Join(ctx.validationResult.Profiles, ","))
	}
	for _, warning := range ctx.warnings {
		fmt.Fprintf(stderr, "warning: %s\n", warning)
	}
	return apperrors.ExitOK
}

func (a *App) runConfigRender(opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	ctx, err := a.loadDeploymentContext(opts)
	if err != nil {
		return a.writeCommandError("config render", apperrors.CodeConfig, err, opts, stdout, stderr)
	}

	if opts.json {
		data := map[string]any{
			"configSource": ctx.configSource,
			"compose":      string(ctx.composeYAML),
		}
		if err := output.WriteEnvelopeWithWarnings(stdout, "config render", apperrors.CodeOK, "Rendered Compose config", data, ctx.warnings, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitOK
	}

	if _, err := stdout.Write(ctx.composeYAML); err != nil {
		return apperrors.ExitGeneric
	}
	for _, warning := range ctx.warnings {
		fmt.Fprintf(stderr, "warning: %s\n", warning)
	}
	return apperrors.ExitOK
}

func (a *App) handleNested(parent string, args []string, opts globalOptions, stdout io.Writer, stderr io.Writer, subcommands []string) int {
	if len(args) == 0 {
		return a.writeUsageError(parent, opts, stdout, stderr)
	}
	if !slices.Contains(subcommands, args[0]) {
		return a.writeUsageError(parent+" "+args[0], opts, stdout, stderr)
	}

	command := parent + " " + args[0]
	if len(args) > 1 && (command == "module enable" || command == "module disable") {
		return a.writeNotImplemented(command+" "+args[1], opts, stdout, stderr)
	}
	return a.writeNotImplemented(command, opts, stdout, stderr)
}

func (a *App) writeNotImplemented(command string, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	message := fmt.Sprintf("%s is recognized but not implemented yet", command)
	if opts.json {
		if err := output.WriteEnvelope(stdout, command, apperrors.CodeGeneric, message, nil, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitGeneric
	}

	fmt.Fprintf(stderr, "%s: %s\n", a.name, message)
	return apperrors.ExitGeneric
}

func (a *App) writeUsageError(command string, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	message := fmt.Sprintf("unknown or incomplete command %q", command)
	if opts.json {
		if err := output.WriteEnvelope(stdout, command, apperrors.CodeUsage, message, nil, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitUsage
	}

	fmt.Fprintf(stderr, "%s: %s\n\n", a.name, message)
	a.writeUsage(stderr)
	return apperrors.ExitUsage
}

func (a *App) writeCommandError(command string, code apperrors.Code, err error, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	message := err.Error()
	if opts.json {
		if writeErr := output.WriteEnvelope(stdout, command, code, message, nil, a.clock); writeErr != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitCodeFor(code)
	}

	fmt.Fprintf(stderr, "%s: %s\n", a.name, message)
	return apperrors.ExitCodeFor(code)
}

func topLevelCommands() []string {
	return []string{
		"init",
		"validate",
		"install",
		"start",
		"stop",
		"restart",
		"status",
		"ps",
		"logs",
		"update",
		"rollback",
		"doctor",
		"bundle",
		"report",
	}
}
