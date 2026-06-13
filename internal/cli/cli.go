package cli

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	apperrors "github.com/Bare-Systems/Bare-Systems-Installer/internal/errors"
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
	case "config":
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
