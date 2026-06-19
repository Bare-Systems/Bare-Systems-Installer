package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"time"

	"github.com/Bare-Systems/Bare-Systems-Installer/internal/compose"
	deploymentconfig "github.com/Bare-Systems/Bare-Systems-Installer/internal/config"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/diagnostics"
	apperrors "github.com/Bare-Systems/Bare-Systems-Installer/internal/errors"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/health"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/modules"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/output"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/portal"
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
	portal     string
	tokenFile  string
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
	case "doctor":
		return a.runDoctor(opts, stdout, stderr)
	case "bundle":
		return a.runBundle(opts, stdout, stderr)
	case "init":
		return a.runInit(remaining[1:], opts, stdout, stderr)
	case "enroll":
		return a.runEnroll(remaining[1:], opts, stdout, stderr)
	case "report":
		return a.runReport(remaining[1:], opts, stdout, stderr)
	case "validate":
		return a.runValidate(opts, stdout, stderr)
	case "config":
		if len(remaining) > 1 && remaining[1] == "render" {
			return a.runConfigRender(remaining[2:], opts, stdout, stderr)
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
	flags.StringVar(&opts.portal, "portal", "", "Portal base URL for enrollment")
	flags.StringVar(&opts.tokenFile, "token-file", "", "path to one-time enrollment token")
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
  init [--force] [--image-registry REGISTRY] [--image-tag TAG]
                           Initialize local deployment files.
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
  enroll                   Enroll this device with the Portal.
  report                   Send deployment status to the Portal.
  config render [--write]  Render canonical Compose config.
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
  --portal URL    Portal base URL for enrollment.
  --token-file PATH
                  Path to one-time enrollment token.
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
	configPath, allowDefaultConfig := resolveConfigPath(opts)
	configLoad, err := deploymentconfig.LoadDeployment(configPath, allowDefaultConfig)
	if err != nil {
		return deploymentContext{}, err
	}

	deployment := configLoad.Deployment
	if opts.projectDir != "" {
		deployment = deploymentForProjectDir(deployment, opts.projectDir)
	}

	envPath, explicitEnv := resolveEnvPath(opts)
	envLoad, err := deploymentconfig.LoadEnv(envPath, explicitEnv)
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

func resolveConfigPath(opts globalOptions) (string, bool) {
	if opts.config != "" {
		return opts.config, false
	}
	if opts.projectDir != "" {
		return filepath.Join(opts.projectDir, "edge.yml"), true
	}
	return "", true
}

func resolveEnvPath(opts globalOptions) (string, bool) {
	if opts.envFile != "" {
		return opts.envFile, true
	}
	if opts.projectDir != "" {
		return filepath.Join(opts.projectDir, ".env"), false
	}
	return "", false
}

func deploymentForProjectDir(deployment deploymentconfig.Deployment, projectDir string) deploymentconfig.Deployment {
	deployment.Spec.Runtime.ComposeProjectDirectory = filepath.Join(projectDir, "compose")
	deployment.Spec.Storage.Root = projectDir
	deployment.Spec.Storage.Backups = filepath.Join(projectDir, "backups")
	return deployment
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

type initOptions struct {
	force         bool
	imageRegistry string
	imageTag      string
}

func (a *App) runInit(args []string, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	initOpts, ok := a.parseInitFlags(args, stderr)
	if !ok {
		return apperrors.ExitUsage
	}

	projectDir := valueOr(opts.projectDir, deploymentconfig.DefaultProjectDir)
	configPath := opts.config
	if configPath == "" {
		if opts.projectDir != "" {
			configPath = filepath.Join(projectDir, "edge.yml")
		} else {
			configPath = deploymentconfig.DefaultConfigPath
		}
	}
	envPath := opts.envFile
	if envPath == "" {
		if opts.projectDir != "" {
			envPath = filepath.Join(projectDir, ".env")
		} else {
			envPath = deploymentconfig.DefaultEnvPath
		}
	}

	deployment := deploymentconfig.DefaultDeployment()
	if opts.projectDir != "" {
		deployment = deploymentForProjectDir(deployment, projectDir)
	}
	registry := modules.BuiltInRegistry()
	env := deploymentconfig.DerivedEnv(deployment)
	if strings.TrimSpace(initOpts.imageRegistry) != "" {
		env["BARE_IMAGE_REGISTRY"] = strings.TrimRight(strings.TrimSpace(initOpts.imageRegistry), "/")
	}
	if strings.TrimSpace(initOpts.imageTag) != "" {
		env["BARE_IMAGE_TAG"] = strings.TrimSpace(initOpts.imageTag)
	}
	composeYAML, err := compose.Render(deployment, registry, env)
	if err != nil {
		return a.writeCommandError("init", apperrors.CodeConfig, err, opts, stdout, stderr)
	}
	if err := compose.ValidateRendered(composeYAML); err != nil {
		return a.writeCommandError("init", apperrors.CodeConfig, err, opts, stdout, stderr)
	}

	paths := initPaths{
		ProjectDir: projectDir,
		ConfigDir:  filepath.Dir(configPath),
		ConfigFile: configPath,
		EnvFile:    envPath,
		ComposeDir: deployment.ComposeProjectDirectory(),
		ComposeFile: filepath.Join(
			deployment.ComposeProjectDirectory(),
			"generated.compose.yml",
		),
		StateDir:             filepath.Join(projectDir, "state"),
		BundleDir:            filepath.Join(projectDir, "bundles"),
		ManifestDir:          filepath.Join(projectDir, "manifests"),
		BackupsDir:           filepath.Join(projectDir, "backups"),
		TardigradeDir:        filepath.Join(projectDir, "tardigrade"),
		TardigradePublicDir:  edgeruntime.TardigradePublicDir(projectDir),
		TardigradeConfigFile: edgeruntime.TardigradeConfigPath(projectDir),
	}

	result, err := initializeProject(paths, deployment, env, composeYAML, initOpts.force)
	if err != nil {
		return a.writeCommandError("init", apperrors.CodePermissions, err, opts, stdout, stderr)
	}

	if opts.json {
		if err := output.WriteEnvelope(stdout, "init", apperrors.CodeOK, "Deployment project initialized", result, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitOK
	}
	if !opts.quiet {
		fmt.Fprintf(stdout, "Deployment project initialized: %s\n", projectDir)
		fmt.Fprintf(stdout, "config: %s\n", configPath)
		fmt.Fprintf(stdout, "env: %s\n", envPath)
		fmt.Fprintf(stdout, "compose: %s\n", paths.ComposeFile)
		fmt.Fprintf(stdout, "tardigrade: %s\n", paths.TardigradeConfigFile)
		if len(result.Skipped) > 0 {
			fmt.Fprintf(stdout, "skipped existing files: %s\n", strings.Join(result.Skipped, ", "))
		}
	}
	return apperrors.ExitOK
}

func (a *App) parseInitFlags(args []string, stderr io.Writer) (initOptions, bool) {
	var initOpts initOptions
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&initOpts.force, "force", false, "regenerate default files")
	flags.StringVar(&initOpts.imageRegistry, "image-registry", "", "default image registry/repository prefix")
	flags.StringVar(&initOpts.imageTag, "image-tag", "", "default image tag")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage:\n  %s [global flags] init [--force] [--image-registry REGISTRY] [--image-tag TAG]\n", a.name)
	}
	if err := flags.Parse(args); err != nil {
		return initOpts, false
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "%s: unexpected init argument %q\n", a.name, flags.Arg(0))
		flags.Usage()
		return initOpts, false
	}
	return initOpts, true
}

type configRenderOptions struct {
	write bool
}

func (a *App) runConfigRender(args []string, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	renderOpts, ok := a.parseConfigRenderFlags(args, stderr)
	if !ok {
		return apperrors.ExitUsage
	}
	ctx, err := a.loadDeploymentContext(opts)
	if err != nil {
		return a.writeCommandError("config render", apperrors.CodeConfig, err, opts, stdout, stderr)
	}

	if renderOpts.write {
		composeFile, err := writeComposeArtifact(ctx)
		if err != nil {
			return a.writeCommandError("config render", apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		tardigradeConfigFile, err := writeTardigradeArtifact(ctx)
		if err != nil {
			return a.writeCommandError("config render", apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		data := map[string]any{
			"configSource":         ctx.configSource,
			"composeFile":          composeFile,
			"tardigradeConfigFile": tardigradeConfigFile,
		}
		if opts.json {
			if err := output.WriteEnvelopeWithWarnings(stdout, "config render", apperrors.CodeOK, "Rendered runtime config written", data, ctx.warnings, a.clock); err != nil {
				return apperrors.ExitGeneric
			}
			return apperrors.ExitOK
		}
		if !opts.quiet {
			fmt.Fprintf(stdout, "Rendered Compose config: %s\n", composeFile)
			fmt.Fprintf(stdout, "Rendered Tardigrade config: %s\n", tardigradeConfigFile)
		}
		for _, warning := range ctx.warnings {
			fmt.Fprintf(stderr, "warning: %s\n", warning)
		}
		return apperrors.ExitOK
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

func (a *App) parseConfigRenderFlags(args []string, stderr io.Writer) (configRenderOptions, bool) {
	var renderOpts configRenderOptions
	flags := flag.NewFlagSet("config render", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&renderOpts.write, "write", false, "write generated runtime artifacts to the project directory")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage:\n  %s [global flags] config render [--write]\n", a.name)
	}
	if err := flags.Parse(args); err != nil {
		return renderOpts, false
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "%s: unexpected config render argument %q\n", a.name, flags.Arg(0))
		flags.Usage()
		return renderOpts, false
	}
	return renderOpts, true
}

func (a *App) runEnroll(args []string, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	var ok bool
	opts, ok = a.parseEnrollFlags(args, opts, stderr)
	if !ok {
		return apperrors.ExitUsage
	}
	if strings.TrimSpace(opts.portal) == "" {
		return a.writeCommandError("enroll", apperrors.CodeUsage, errors.New("--portal is required"), opts, stdout, stderr)
	}
	if strings.TrimSpace(opts.tokenFile) == "" {
		return a.writeCommandError("enroll", apperrors.CodeUsage, errors.New("--token-file is required"), opts, stdout, stderr)
	}

	ctx, err := a.loadDeploymentContext(opts)
	if err != nil {
		return a.writeCommandError("enroll", apperrors.CodeConfig, err, opts, stdout, stderr)
	}
	bootstrapToken, err := portal.ReadBootstrapToken(opts.tokenFile)
	if err != nil {
		return a.writeCommandError("enroll", apperrors.CodeAuth, err, opts, stdout, stderr)
	}
	client, err := portal.NewHTTPClient(opts.portal)
	if err != nil {
		return a.writeCommandError("enroll", apperrors.CodeUsage, err, opts, stdout, stderr)
	}

	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "unknown"
	}
	response, err := client.Enroll(context.Background(), portal.EnrollmentRequest{
		EnrollmentToken:    bootstrapToken,
		Hostname:           hostname,
		Platform:           goruntime.GOOS,
		Arch:               goruntime.GOARCH,
		BareSystemsVersion: version.Current().Version,
	})
	if err != nil {
		code := apperrors.CodeNetwork
		if portal.IsAuthError(err) {
			code = apperrors.CodeAuth
		}
		return a.writeCommandError("enroll", code, err, opts, stdout, stderr)
	}

	stateDir := portal.StateDir(ctx.deployment)
	identity, err := portal.PersistEnrollment(stateDir, response, a.clock().UTC())
	if err != nil {
		return a.writeCommandError("enroll", apperrors.CodePermissions, err, opts, stdout, stderr)
	}

	data := map[string]any{
		"deviceId":       identity.DeviceID,
		"portalUrl":      identity.PortalURL,
		"identityFile":   portal.IdentityPath(stateDir),
		"credentialFile": identity.CredentialFile,
		"enrolledAt":     identity.EnrolledAt,
	}
	if opts.json {
		if err := output.WriteEnvelope(stdout, "enroll", apperrors.CodeOK, "Device enrolled", data, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitOK
	}
	if !opts.quiet {
		fmt.Fprintf(stdout, "Enrolled device %s\n", identity.DeviceID)
		fmt.Fprintf(stdout, "identity: %s\n", portal.IdentityPath(stateDir))
	}
	return apperrors.ExitOK
}

func (a *App) parseEnrollFlags(args []string, opts globalOptions, stderr io.Writer) (globalOptions, bool) {
	flags := flag.NewFlagSet("enroll", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&opts.portal, "portal", opts.portal, "Portal base URL")
	flags.StringVar(&opts.tokenFile, "token-file", opts.tokenFile, "path to one-time enrollment token")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage:\n  %s [global flags] enroll --portal URL --token-file PATH\n", a.name)
	}
	if err := flags.Parse(args); err != nil {
		return opts, false
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "%s: unexpected enroll argument %q\n", a.name, flags.Arg(0))
		flags.Usage()
		return opts, false
	}
	return opts, true
}

func (a *App) runReport(args []string, opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 {
		return a.writeUsageError("report "+strings.Join(args, " "), opts, stdout, stderr)
	}

	ctx, err := a.loadDeploymentContext(opts)
	if err != nil {
		return a.writeCommandError("report", apperrors.CodeConfig, err, opts, stdout, stderr)
	}
	stateDir := portal.StateDir(ctx.deployment)
	identity, err := portal.LoadIdentity(stateDir)
	if err != nil {
		return a.writeCommandError("report", apperrors.CodeAuth, err, opts, stdout, stderr)
	}
	deviceToken, err := portal.ReadDeviceToken(identity.CredentialFile)
	if err != nil {
		return a.writeCommandError("report", apperrors.CodeAuth, err, opts, stdout, stderr)
	}
	client, err := portal.NewHTTPClient(identity.PortalURL)
	if err != nil {
		return a.writeCommandError("report", apperrors.CodeAuth, err, opts, stdout, stderr)
	}

	healthReport := a.evaluateHealth(ctx)
	runtimeState, runtimeErr := a.collectRuntimeState(ctx)
	payload := portal.BuildReportPayload(portal.ReportInput{
		Identity:         identity,
		Deployment:       ctx.deployment,
		ValidationResult: ctx.validationResult,
		ComposeYAML:      ctx.composeYAML,
		RuntimeState:     runtimeState,
		RuntimeError:     runtimeErr,
		HealthReport:     healthReport,
		CLIVersion:       version.Current(),
		Timestamp:        a.clock().UTC(),
		Warnings:         ctx.warnings,
	})

	if err := client.Report(context.Background(), payload, deviceToken); err != nil {
		if portal.IsAuthError(err) {
			return a.writeCommandError("report", apperrors.CodeAuth, err, opts, stdout, stderr)
		}
		spool, spoolErr := portal.SpoolReportFailure(stateDir, payload, err, a.clock().UTC())
		if spoolErr != nil {
			return a.writeCommandError("report", apperrors.CodePermissions, fmt.Errorf("Portal report failed and spool failed: %w", spoolErr), opts, stdout, stderr)
		}
		data := map[string]any{
			"deviceId":  identity.DeviceID,
			"spooled":   true,
			"spoolPath": spool.Path,
			"error":     err.Error(),
		}
		if opts.json {
			if writeErr := output.WriteEnvelope(stdout, "report", apperrors.CodeNetwork, "Portal report failed; payload spooled", data, a.clock); writeErr != nil {
				return apperrors.ExitGeneric
			}
			return apperrors.ExitNetwork
		}
		fmt.Fprintf(stderr, "%s: Portal report failed; payload spooled at %s: %s\n", a.name, spool.Path, err.Error())
		return apperrors.ExitNetwork
	}

	data := map[string]any{
		"deviceId":       identity.DeviceID,
		"configRevision": payload.ConfigRevision,
		"enabledModules": payload.EnabledModules,
		"profiles":       payload.Profiles,
		"healthSummary":  payload.HealthSummary,
	}
	if opts.json {
		if err := output.WriteEnvelope(stdout, "report", apperrors.CodeOK, "Portal report sent", data, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitOK
	}
	if !opts.quiet {
		fmt.Fprintf(stdout, "Portal report sent for device %s\n", identity.DeviceID)
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
	tardigrade := tardigradeRuntime(ctx, a.runner)

	switch command {
	case "install":
		composeFile, err := writeComposeArtifact(ctx)
		if err != nil {
			return a.writeCommandError(command, apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		tardigradeConfigFile, err := writeTardigradeArtifact(ctx)
		if err != nil {
			return a.writeCommandError(command, apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		if result, err := manager.Config(context.Background()); err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		if result, err := tardigrade.Validate(context.Background()); err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		result, err := manager.Pull(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		return a.writeRuntimeSuccess(command, "Deployment artifacts rendered, images pulled, and Tardigrade config validated", map[string]any{"composeFile": composeFile, "tardigradeConfigFile": tardigradeConfigFile}, result, opts, stdout, stderr)
	case "start":
		composeFile, err := writeComposeArtifact(ctx)
		if err != nil {
			return a.writeCommandError(command, apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		tardigradeConfigFile, err := writeTardigradeArtifact(ctx)
		if err != nil {
			return a.writeCommandError(command, apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		result, err := manager.Up(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		result, err = tardigrade.StartOrReload(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		return a.writeRuntimeSuccess(command, "Deployment and Tardigrade started", map[string]any{"composeFile": composeFile, "tardigradeConfigFile": tardigradeConfigFile}, result, opts, stdout, stderr)
	case "update":
		composeFile, err := writeComposeArtifact(ctx)
		if err != nil {
			return a.writeCommandError(command, apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		tardigradeConfigFile, err := writeTardigradeArtifact(ctx)
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
		result, err = tardigrade.StartOrReload(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		return a.writeRuntimeSuccess(command, "Deployment and Tardigrade updated", map[string]any{"composeFile": composeFile, "tardigradeConfigFile": tardigradeConfigFile}, result, opts, stdout, stderr)
	case "stop":
		if _, err := writeTardigradeArtifact(ctx); err != nil {
			return a.writeCommandError(command, apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		result, err := tardigrade.StopIfRunning(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		result, err = manager.Stop(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		return a.writeRuntimeSuccess(command, "Deployment and Tardigrade stopped", nil, result, opts, stdout, stderr)
	case "restart":
		composeFile, err := writeComposeArtifact(ctx)
		if err != nil {
			return a.writeCommandError(command, apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		tardigradeConfigFile, err := writeTardigradeArtifact(ctx)
		if err != nil {
			return a.writeCommandError(command, apperrors.CodePermissions, err, opts, stdout, stderr)
		}
		result, err := manager.Restart(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		result, err = tardigrade.StartOrReload(context.Background())
		if err != nil {
			return a.writeRuntimeError(command, result, err, opts, stdout, stderr)
		}
		return a.writeRuntimeSuccess(command, "Deployment restarted and Tardigrade reloaded", map[string]any{"composeFile": composeFile, "tardigradeConfigFile": tardigradeConfigFile}, result, opts, stdout, stderr)
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
	if _, err := a.loadDeploymentContext(opts); err != nil {
		return a.writeCommandError("service "+action, apperrors.CodeConfig, err, opts, stdout, stderr)
	}

	options := edgeruntime.ServiceOptions{
		ProjectDir: opts.projectDir,
	}
	if binaryPath, err := os.Executable(); err == nil {
		options.BinaryPath = binaryPath
	}
	var result edgeruntime.ServiceResult
	var err error
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

func (a *App) runDoctor(opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	_, report, _ := a.buildDoctorReport(opts)
	code := apperrors.CodeOK
	exitCode := apperrors.ExitOK
	if report.HasFailures() {
		code = apperrors.CodeHealth
		exitCode = apperrors.ExitHealth
	}
	if opts.json {
		if err := output.WriteEnvelope(stdout, "doctor", code, report.Summary.Message, report, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return exitCode
	}

	for _, check := range report.Checks {
		fmt.Fprintf(stdout, "%s %s: %s\n", strings.ToUpper(string(check.Status)), check.Name, check.Message)
		if check.Remediation != "" {
			fmt.Fprintf(stdout, "  remediation: %s\n", check.Remediation)
		}
	}
	if report.Summary.Message != "" {
		fmt.Fprintf(stdout, "\n%s\n", report.Summary.Message)
	}
	return exitCode
}

func (a *App) runBundle(opts globalOptions, stdout io.Writer, stderr io.Writer) int {
	ctx, report, ctxErr := a.buildDoctorReport(opts)
	if ctxErr != nil {
		ctx = fallbackDeploymentContext(opts, ctxErr)
	}

	result, err := diagnostics.Create(context.Background(), diagnostics.Options{
		Deployment:       ctx.deployment,
		ConfigSource:     ctx.configSource,
		EnvSource:        ctx.envSource,
		Env:              ctx.env,
		ComposeYAML:      ctx.composeYAML,
		ValidationResult: ctx.validationResult,
		Registry:         modules.BuiltInRegistry(),
		Runner:           a.runner,
		HealthReport:     report,
		Now:              a.clock().UTC(),
	})
	if err != nil {
		return a.writeCommandError("bundle", apperrors.CodeDiagnostics, err, opts, stdout, stderr)
	}
	if opts.json {
		if err := output.WriteEnvelope(stdout, "bundle", apperrors.CodeOK, "Diagnostics bundle created", result, a.clock); err != nil {
			return apperrors.ExitGeneric
		}
		return apperrors.ExitOK
	}
	if !opts.quiet {
		fmt.Fprintf(stdout, "Diagnostics bundle: %s\n", result.Path)
		fmt.Fprintf(stdout, "Size: %d bytes\n", result.SizeBytes)
	}
	return apperrors.ExitOK
}

func (a *App) buildDoctorReport(opts globalOptions) (deploymentContext, health.Report, error) {
	ctx, err := a.loadDeploymentContext(opts)
	if err != nil {
		return deploymentContext{}, health.ConfigFailure(err), err
	}
	return ctx, a.evaluateHealth(ctx), nil
}

func (a *App) evaluateHealth(ctx deploymentContext) health.Report {
	return health.Evaluate(context.Background(), health.Options{
		Deployment:       ctx.deployment,
		ValidationResult: ctx.validationResult,
		ComposeYAML:      ctx.composeYAML,
		Registry:         modules.BuiltInRegistry(),
		Runner:           a.runner,
		ComposeFile:      filepath.Join(ctx.deployment.ComposeProjectDirectory(), "generated.compose.yml"),
	})
}

func (a *App) collectRuntimeState(ctx deploymentContext) (edgeruntime.RuntimeState, error) {
	empty := edgeruntime.RuntimeState{
		Summary: edgeruntime.StateSummary{
			ByState:  map[string]int{},
			ByHealth: map[string]int{},
		},
	}
	if _, err := edgeruntime.CheckDocker(context.Background(), a.runner); err != nil {
		return empty, err
	}
	manager := edgeruntime.Compose{
		Runner:      a.runner,
		ProjectDir:  ctx.deployment.ComposeProjectDirectory(),
		ProjectName: ctx.deployment.ComposeProjectName(),
		ComposeFile: filepath.Join(ctx.deployment.ComposeProjectDirectory(), "generated.compose.yml"),
		Profiles:    ctx.validationResult.Profiles,
	}
	result, err := manager.PS(context.Background(), true)
	if err != nil {
		return empty, err
	}
	state, err := edgeruntime.ParsePSJSON(result.Stdout)
	if err != nil {
		return empty, err
	}
	return state, nil
}

func fallbackDeploymentContext(opts globalOptions, err error) deploymentContext {
	deployment := deploymentconfig.DefaultDeployment()
	if opts.projectDir != "" {
		deployment.Spec.Storage.Root = opts.projectDir
		deployment.Spec.Runtime.ComposeProjectDirectory = filepath.Join(opts.projectDir, "compose")
	}
	return deploymentContext{
		deployment:       deployment,
		configSource:     valueOr(opts.config, "unavailable"),
		envSource:        valueOr(opts.envFile, "unavailable"),
		env:              deploymentconfig.DerivedEnv(deployment),
		validationResult: deploymentconfig.ValidationResult{EnabledModules: []string{"core"}, Profiles: []string{"core"}},
		composeYAML:      []byte("# Compose render unavailable: " + diagnostics.RedactString(err.Error()) + "\n"),
		warnings:         []string{err.Error()},
	}
}

func valueOr(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
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

type initPaths struct {
	ProjectDir           string `json:"projectDir"`
	ConfigDir            string `json:"configDir"`
	ConfigFile           string `json:"configFile"`
	EnvFile              string `json:"envFile"`
	ComposeDir           string `json:"composeDir"`
	ComposeFile          string `json:"composeFile"`
	TardigradeDir        string `json:"tardigradeDir"`
	TardigradePublicDir  string `json:"tardigradePublicDir"`
	TardigradeConfigFile string `json:"tardigradeConfigFile"`
	StateDir             string `json:"stateDir"`
	BundleDir            string `json:"bundleDir"`
	ManifestDir          string `json:"manifestDir"`
	BackupsDir           string `json:"backupsDir"`
}

type initResult struct {
	Paths       initPaths `json:"paths"`
	CreatedDirs []string  `json:"createdDirs"`
	Written     []string  `json:"written"`
	Skipped     []string  `json:"skipped"`
	Forced      bool      `json:"forced"`
}

func initializeProject(paths initPaths, deployment deploymentconfig.Deployment, env deploymentconfig.Environment, composeYAML []byte, force bool) (initResult, error) {
	result := initResult{Paths: paths, Forced: force}
	for _, dir := range []struct {
		path string
		mode os.FileMode
	}{
		{paths.ProjectDir, 0o755},
		{paths.ConfigDir, 0o755},
		{paths.ComposeDir, 0o755},
		{paths.TardigradeDir, 0o755},
		{paths.TardigradePublicDir, 0o755},
		{paths.StateDir, 0o700},
		{paths.BundleDir, 0o755},
		{paths.ManifestDir, 0o755},
		{paths.BackupsDir, 0o755},
	} {
		created, err := ensureDir(dir.path, dir.mode)
		if err != nil {
			return initResult{}, err
		}
		if created {
			result.CreatedDirs = append(result.CreatedDirs, dir.path)
		}
	}

	configYAML, err := deploymentconfig.DeploymentYAML(deployment)
	if err != nil {
		return initResult{}, fmt.Errorf("marshal default deployment: %w", err)
	}
	tardigradeConfig := []byte(edgeruntime.RenderTardigradeConfig(tardigradeConfigOptions(deployment, env)))
	for _, file := range []struct {
		path string
		data []byte
	}{
		{paths.ConfigFile, configYAML},
		{paths.EnvFile, defaultEnvFile(env)},
		{paths.ComposeFile, composeYAML},
		{paths.TardigradeConfigFile, tardigradeConfig},
		{filepath.Join(paths.ManifestDir, "README.md"), []byte(defaultManifestReadme())},
	} {
		written, err := writeDefaultFile(file.path, file.data, force)
		if err != nil {
			return initResult{}, err
		}
		if written {
			result.Written = append(result.Written, file.path)
		} else {
			result.Skipped = append(result.Skipped, file.path)
		}
	}
	return result, nil
}

func ensureDir(path string, mode os.FileMode) (bool, error) {
	if path == "" || path == "." {
		return false, nil
	}
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("%s exists and is not a directory", path)
		}
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat directory %s: %w", path, err)
	}
	if err := os.MkdirAll(path, mode); err != nil {
		return false, fmt.Errorf("create directory %s: %w", path, err)
	}
	return true, nil
}

func writeDefaultFile(path string, data []byte, force bool) (bool, error) {
	if _, err := os.Stat(path); err == nil && !force {
		return false, nil
	} else if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("stat file %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create parent directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, fmt.Errorf("write default file %s: %w", path, err)
	}
	return true, nil
}

func defaultEnvFile(env deploymentconfig.Environment) []byte {
	var builder strings.Builder
	builder.WriteString("# Non-secret Bare Systems Compose interpolation values.\n")
	builder.WriteString("# Do not put tokens, passwords, API keys, private keys, or TLS keys here.\n\n")
	for _, key := range env.Keys() {
		fmt.Fprintf(&builder, "%s=%s\n", key, env[key])
	}
	builder.WriteString("\n")
	builder.WriteString("# Optional per-service image overrides. Leave commented to use BARE_IMAGE_REGISTRY/BARE_IMAGE_TAG.\n")
	for _, key := range []string{"BEARCLAW_WEB_IMAGE", "KOALA_ORCHESTRATOR_IMAGE", "KOALA_WORKER_IMAGE", "POLAR_IMAGE", "KODIAK_IMAGE", "URSA_IMAGE"} {
		fmt.Fprintf(&builder, "# %s=\n", key)
	}
	return []byte(builder.String())
}

func defaultManifestReadme() string {
	return `# Module Manifests

Bare Systems currently ships built-in manifests for core, koala, polar, kodiak, and ursa.
This directory is reserved for future operator-supplied manifest overrides.
`
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

func writeTardigradeArtifact(ctx deploymentContext) (string, error) {
	projectRoot := ctx.deployment.Spec.Storage.Root
	path := edgeruntime.TardigradeConfigPath(projectRoot)
	if err := edgeruntime.WriteTardigradeConfig(path, tardigradeConfigOptions(ctx.deployment, ctx.env)); err != nil {
		return "", err
	}
	return path, nil
}

func tardigradeConfigOptions(deployment deploymentconfig.Deployment, env deploymentconfig.Environment) edgeruntime.TardigradeConfigOptions {
	projectRoot := deployment.Spec.Storage.Root
	return edgeruntime.TardigradeConfigOptions{
		ListenPort:  edgeruntime.TardigradeListenPort(env["PUBLIC_HTTP_PORT"]),
		PidFile:     edgeruntime.TardigradePidFile(projectRoot),
		PublicDir:   edgeruntime.TardigradePublicDir(projectRoot),
		UpstreamURL: edgeruntime.TardigradeUpstreamURL(env["BEARCLAW_WEB_BIND_ADDRESS"], env["BEARCLAW_WEB_PORT"]),
		ServerNames: edgeruntime.TardigradeServerNames(env["TARDIGRADE_SERVER_NAMES"]),
	}
}

func tardigradeRuntime(ctx deploymentContext, runner edgeruntime.Runner) edgeruntime.Tardigrade {
	configPath := edgeruntime.TardigradeConfigPath(ctx.deployment.Spec.Storage.Root)
	return edgeruntime.Tardigrade{
		Runner:     runner,
		ConfigFile: configPath,
		WorkDir:    filepath.Dir(configPath),
	}
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
