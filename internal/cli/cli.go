package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	apperrors "github.com/Bare-Systems/Bare-Systems-Installer/internal/errors"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/output"
	"github.com/Bare-Systems/Bare-Systems-Installer/internal/version"
)

const commandName = "bare-systems"

type App struct {
	name string
}

func New() *App {
	return &App{name: commandName}
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
	default:
		fmt.Fprintf(stderr, "%s: unknown command %q\n\n", a.name, remaining[0])
		a.writeUsage(stderr)
		return apperrors.ExitUsage
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
  help       Show this help text.
  version    Print CLI version and build metadata.

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
		if err := output.JSON(stdout, info); err != nil {
			return apperrors.ExitGeneric
		}
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
