package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/compose"
	deploymentconfig "github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	apperrors "github.com/Bare-Systems/Bare-Systems-Installer/internal/errors"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/modules"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/output"
	edgeruntime "github.com/Bare-Systems/Bare-Systems-Installer/internal/runtime"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/version"
)

const commandName = "bare-systems"

type App struct {
	name   string
	clock  output.Clock
	runner edgeruntime.Runner
}

func New() *App {
	return &App{name: commandName, clock: time.Now, runner: edgeruntime.ExecRunner{}}
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
	case "install", "start", "stop", "restart", "status", "ps", "logs", "update":
		return a.runRuntimeCommand(remaining[0], remaining[1:], opts, stdout, stderr)
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
		if len(remaining) > 1 && (remaining[1] == "install" || remaining[1] == "uninstall") {
			return a.runServiceCommand(remaining[1], remaining[2:], opts, stdout, stderr)
		}
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

func (a *App) runRuntimeCommand(command string, args []string, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	if command != "logs" && len(args) > 0 {
		return a.writeUsageError(command+" "+strings.Join(args, " "), opts, stdout, stderr)
	}
	if command == "logs" && len(args) > 1 {
		return a.writeUsageError(command+" "+strings.Join(args, " "), opts, stdout, stderr)
	}

	ctx, err := a.loadDeploymentContext(opts)
	if err != nil {
		return a.writeCommandError(command, apperrors.CodeConfig, err, opts, stdout, stderr)
	}

	if report, err := edgeruntime.CheckDocker(context.Background(), a.runner); err != nil {
		return a.writePrereqError(command, report, err, opts, stdout, stderr)
	}

	manager := edgeruntime.Compose{
		Runner:      a.runner,
		ProjectDir:  ctx.deployment.ComposeProjectDirectory(),
		ProjectName: ctx.deployment.ComposeProjectName(),
		ComposeFile: filepath.Join(ctx.deployment.ComposeProjectDirectory(), "generated.compose.yml"),
		Profiles:    ctx.validationResult.Profiles,
	}

	switch command {
	case "install":
		composeFile, err := writeComposeArtifact(ctx)
		if err != nil {
			return a.writeCommandError(command, apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		if result, err := manager.Config(context.Background()); err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		result, err := manager.Pull(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		return a.writeRuntimeSuccess(command, "Deployment artifacts rendered and images pulled", map[string]any{"composeFile": composeFile}, result, opts, stdout, stderr)
	case "start":
		composeFile, err := writeComposeArtifact(ctx)
		if err != nil {
			return a.writeCommandError(command, apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		result, err := manager.Up(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		return a.writeRuntimeSuccess(command, "Deployment started", map[string]any{"composeFile": composeFile}, result, opts, stdout, stderr)
	case "update":
		composeFile, err := writeComposeArtifact(ctx)
		if err != nil {
			return a.writeCommandError(command, apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		if result, err := manager.Pull(context.Background()); err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		result, err := manager.Up(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		return a.writeRuntimeSuccess(command, "Deployment updated", map[string]any{"composeFile": composeFile}, result, opts, stdout, stderr)
	case "stop":
		result, err := manager.Stop(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		return a.writeRuntimeSuccess(command, "Deployment stopped", nil, result, opts, stdout, stderr)
	case "restart":
		result, err := manager.Restart(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		return a.writeRuntimeSuccess(command, "Deployment restarted", nil, result, opts, stdout, stderr)
	case "status":
		result, err := manager.PS(context.Background(), true)
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		state, err := edgeruntime.ParsePSJSON(result.Stdout)
		if err != nil {
			return a.writeCommandError(command, apperrors.CodeRuntime, err, opts, stdout, stderr)
		}
		if opts.json {
			if err := output.WriteEnvelope(stdout, command, apperrors.CodeOK, "Runtime status collected", state, a.clock); err != nil {
				return apperrors.ExitGeneric
			}
			return apperrors.ExitOK
		}
		fmt.Fprintf(stdout, "containers: %d\n", state.Summary.Total)
		for stateName, count := range state.Summary.ByState {
			fmt.Fprintf(stdout, "%s: %d\n", stateName, count)
		}
		return apperrors.ExitOK
	case "ps":
		result, err := manager.PS(context.Background(), opts.json)
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		if opts.json {
			state, err := edgeruntime.ParsePSJSON(result.Stdout)
			if err != nil {
				return a.writeCommandError(command, apperrors.CodeRuntime, err, opts, stdout, stderr)
			}
			if err := output.WriteEnvelope(stdout, command, apperrors.CodeOK, "Runtime containers listed", state, a.clock); err != nil {
				return apperrors.ExitGeneric
			}
			return apperrors.ExitOK
		}
		fmt.Fprint(stdout, result.Stdout)
		fmt.Fprint(stderr, result.Stderr)
		return apperrors.ExitOK
	case "logs":
		service := ""
		if len(args) == 1 {
			service = args[0]
		}
		result, err := manager.Logs(context.Background(), service)
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		if opts.json {
			if err := output.WriteEnvelope(stdout, command, apperrors.CodeOK, "Runtime logs collected", map[string]any{"service": service, "stdout": result.Stdout, "stderr": result.Stderr}, a.clock); err != nil {
				return apperrors.ExitGeneric
			}
			return apperrors.ExitOK
		}
		fmt.Fprint(stdout, result.Stdout)
		fmt.Fprint(stderr, result.Stderr)
		return apperrors.ExitOK
	default:
		return a.writeNotImplemented(command, opts, stdout, stderr)
	}
}

func (a *App) runServiceCommand(action string, args []string, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 {
		return a.writeUsageError("service "+action+" "+strings.Join(args, " "), opts, stdout, stderr)
	}
	ctx, err := a.loadDeploymentContext(opts)
	if err != nil {
		return a.writeCommandError("service "+action, apperrors.CodeConfig, err, opts, stdout, stderr)
	}

	options := edgeruntime.ServiceOptions{
		ProjectDir: ctx.deployment.Spec.Storage.Root,
	}
	var result edgeruntime.ServiceResult
	switch action {
	case "install":
		result, err = edgeruntime.InstallSystemdService(context.Background(), a.runner, options)
	case "uninstall":
		result, err = edgeruntime.UninstallSystemdService(context.Background(), a.runner, options)
	}
	if err != nil {
		return a.writeCommandError("service "+action, apperrors.CodePrereq, err, opts, stdout, stderr)
	}
	if opts.json {
		if err := output.WriteEnvelope(stdout, "service "+action, apperrors.CodeOK, result.Message, result, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitOK
	}
	if !opts.quiet {
		fmt.Fprintf(stdout, "%s: %s\n", result.Message, result.UnitPath)
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

func (a *App) writePrereqError(command string, report edgeruntime.PrereqReport, err error, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	if opts.json {
		if writeErr := output.WriteEnvelope(stdout, command, apperrors.CodePrereq, err.Error(), report, a.clock); writeErr != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitPrereq
	}
	fmt.Fprintf(stderr, "%s: %s\n", a.name, err.Error())
	return apperrors.ExitPrereq
}

func (a *App) writeRuntimeError(command string, result edgeruntime.Result, err error, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	data := map[string]any{
		"stdout": result.Stdout,
		"stderr": result.Stderr,
	}
	if opts.json {
		if writeErr := output.WriteEnvelope(stdout, command, apperrors.CodeRuntime, err.Error(), data, a.clock); writeErr != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitRuntime
	}
	fmt.Fprintf(stderr, "%s: %s\n", a.name, err.Error())
	return apperrors.ExitRuntime
}

func (a *App) writeRuntimeSuccess(command string, message string, data map[string]any, result edgeruntime.Result, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	if data == nil {
		data = map[string]any{}
	}
	if result.Stdout != "" {
		data["stdout"] = result.Stdout
	}
	if result.Stderr != "" {
		data["stderr"] = result.Stderr
	}
	if opts.json {
		if err := output.WriteEnvelope(stdout, command, apperrors.CodeOK, message, data, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitOK
	}
	if !opts.quiet {
		fmt.Fprintln(stdout, message)
	}
	if opts.verbose {
		fmt.Fprint(stdout, result.Stdout)
		fmt.Fprint(stderr, result.Stderr)
	}
	return apperrors.ExitOK
}

func writeComposeArtifact(ctx deploymentContext) (string, error) {
	dir := ctx.deployment.ComposeProjectDirectory()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create Compose project directory %s: %w", dir, err)
	}
	path := filepath.Join(dir, "generated.compose.yml")
	if err := os.WriteFile(path, ctx.composeYAML, 0o644); err != nil {
		return "", fmt.Errorf("write generated Compose file %s: %w", path, err)
	}
	return path, nil
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
