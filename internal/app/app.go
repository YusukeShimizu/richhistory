package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/subcommands"

	"github.com/YusukeShimizu/richhistory/internal/config"
	"github.com/YusukeShimizu/richhistory/internal/paths"
	"github.com/YusukeShimizu/richhistory/internal/record"
	"github.com/YusukeShimizu/richhistory/internal/store"
	"github.com/YusukeShimizu/richhistory/internal/term"
)

const (
	defaultListLimit = 20
	timeLayout       = time.RFC3339
	defaultCLIName   = "richhistory"
)

type runtime struct {
	stdout io.Writer
	stderr io.Writer
	name   string
	binRef string

	loaded bool
	cfg    config.Config
	root   string
}

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return RunAs(defaultCLIName, args, stdout, stderr)
}

func RunAs(invokedAs string, args []string, stdout io.Writer, stderr io.Writer) int {
	name := displayName(invokedAs)
	topFlags := flag.NewFlagSet(name, flag.ContinueOnError)
	topFlags.SetOutput(stderr)
	parseErr := topFlags.Parse(args)
	if parseErr != nil {
		return int(subcommands.ExitUsageError)
	}

	rt := &runtime{
		stdout: stdout,
		stderr: stderr,
		name:   name,
		binRef: commandRef(invokedAs, name),
	}
	commander := newCommander(topFlags, rt)
	return int(commander.Execute(context.Background()))
}

func newCommander(topFlags *flag.FlagSet, rt *runtime) *subcommands.Commander {
	commander := subcommands.NewCommander(topFlags, rt.name)
	commander.Output = rt.stdout
	commander.Error = rt.stderr

	commander.Register(commander.HelpCommand(), "")
	commander.Register(commander.FlagsCommand(), "")
	commander.Register(commander.CommandsCommand(), "")
	commander.Register(termCommand{runtime: rt}, "")
	commander.Register(recordCommand{runtime: rt}, "")
	commander.Register(&showCommand{runtime: rt}, "")
	commander.Register(&searchCommand{runtime: rt}, "")

	return commander
}

type termCommand struct {
	runtime *runtime
}

func (termCommand) Name() string { return "term" }

func (termCommand) Synopsis() string { return "terminal integration helpers" }

func (termCommand) Usage() string {
	return "term init zsh [--name NAME]\n"
}

func (termCommand) SetFlags(*flag.FlagSet) {}

func (command termCommand) Execute(_ context.Context, fs *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	args := fs.Args()
	if len(args) < 2 || args[0] != "init" || args[1] != "zsh" {
		fmt.Fprintf(command.runtime.stderr, "usage: %s term init zsh [--name NAME]\n", command.runtime.name)
		return subcommands.ExitUsageError
	}

	initFS := newFlagSet("term init zsh", command.runtime.stderr)
	name := initFS.String("name", "", "session name")
	parseErr := initFS.Parse(args[2:])
	if parseErr != nil {
		return subcommands.ExitUsageError
	}

	fmt.Fprint(command.runtime.stdout, term.ZshInit(command.runtime.binRef, *name))
	return subcommands.ExitSuccess
}

type recordCommand struct {
	runtime *runtime
}

func (recordCommand) Name() string { return "record" }

func (recordCommand) Synopsis() string { return "record command lifecycle events" }

func (recordCommand) Usage() string {
	return "record start|finish ...\n"
}

func (recordCommand) SetFlags(*flag.FlagSet) {}

func (command recordCommand) Execute(_ context.Context, fs *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	cfg, root, runtimeErr := command.runtime.runtime()
	if runtimeErr != nil {
		fmt.Fprintf(command.runtime.stderr, "load runtime: %v\n", runtimeErr)
		return subcommands.ExitFailure
	}

	args := fs.Args()
	if len(args) == 0 {
		fmt.Fprintf(command.runtime.stderr, "usage: %s record start|finish ...\n", command.runtime.name)
		return subcommands.ExitUsageError
	}

	switch args[0] {
	case "start":
		return command.executeStart(cfg, root, args[1:])
	case "finish":
		return command.executeFinish(cfg, root, args[1:])
	default:
		fmt.Fprintf(command.runtime.stderr, "unknown record subcommand %q\n", args[0])
		return subcommands.ExitUsageError
	}
}

func (command recordCommand) executeStart(cfg config.Config, root string, args []string) subcommands.ExitStatus {
	startFS := newFlagSet("record start", command.runtime.stderr)
	format := startFS.String("format", "shell", "output format")
	sessionID := startFS.String("session-id", "", "session id")
	sessionName := startFS.String("session-name", "", "session name")
	seq := startFS.Int("seq", 0, "session sequence")
	shellName := startFS.String("shell", "", "shell name")
	shellPID := startFS.Int("shell-pid", 0, "shell pid")
	ttyName := startFS.String("tty", "", "tty name")
	pwd := startFS.String("pwd", "", "pwd")
	commandText := startFS.String("command", "", "command")
	captureOutput := startFS.Bool("capture-output", false, "capture stdout/stderr")
	startedAtRaw := startFS.String("started-at", "", "start time")
	parseErr := startFS.Parse(args)
	if parseErr != nil {
		return subcommands.ExitUsageError
	}
	if *format != "shell" {
		fmt.Fprintln(command.runtime.stderr, "only --format shell is supported")
		return subcommands.ExitUsageError
	}

	startedAt, timeErr := time.Parse(timeLayout, *startedAtRaw)
	if timeErr != nil {
		fmt.Fprintf(command.runtime.stderr, "parse --started-at: %v\n", timeErr)
		return subcommands.ExitUsageError
	}

	result, startErr := record.Start(root, cfg, record.StartInput{
		SessionID:     *sessionID,
		SessionName:   *sessionName,
		Seq:           *seq,
		Shell:         *shellName,
		ShellPID:      *shellPID,
		TTY:           *ttyName,
		Command:       *commandText,
		PWD:           *pwd,
		CaptureOutput: *captureOutput,
		StartedAt:     startedAt,
	})
	if startErr != nil {
		fmt.Fprintf(command.runtime.stderr, "record start: %v\n", startErr)
		return subcommands.ExitFailure
	}

	fmt.Fprint(command.runtime.stdout, term.ShellAssignments(map[string]string{
		"RICHHISTORY_CAPTURE_MODE":        result.CaptureMode,
		"RICHHISTORY_EVENT_ID":            result.EventID,
		"RICHHISTORY_EVENT_STATE":         result.StateFile,
		"RICHHISTORY_CAPTURE_BEFORE_FILE": result.CaptureBeforeFile,
		"RICHHISTORY_CAPTURE_AFTER_FILE":  result.CaptureAfterFile,
	}))
	fmt.Fprint(command.runtime.stdout, "\n")
	return subcommands.ExitSuccess
}

func (command recordCommand) executeFinish(cfg config.Config, root string, args []string) subcommands.ExitStatus {
	finishFS := newFlagSet("record finish", command.runtime.stderr)
	stateFile := finishFS.String("state-file", "", "state file")
	pwdAfter := finishFS.String("pwd-after", "", "pwd after")
	exitCode := finishFS.Int("exit-code", 0, "exit code")
	finishedAtRaw := finishFS.String("finished-at", "", "finish time")
	parseErr := finishFS.Parse(args)
	if parseErr != nil {
		return subcommands.ExitUsageError
	}

	finishedAt, timeErr := parseFinishedAt(*finishedAtRaw)
	if timeErr != nil {
		fmt.Fprintf(command.runtime.stderr, "parse --finished-at: %v\n", timeErr)
		return subcommands.ExitUsageError
	}

	event, written, finishErr := record.Finish(root, cfg, record.FinishInput{
		StateFile:  *stateFile,
		PWDAfter:   *pwdAfter,
		ExitCode:   *exitCode,
		FinishedAt: finishedAt,
	})
	if finishErr != nil {
		fmt.Fprintf(command.runtime.stderr, "record finish: %v\n", finishErr)
		return subcommands.ExitFailure
	}
	if !written {
		return subcommands.ExitSuccess
	}

	fmt.Fprintln(command.runtime.stdout, event.ID)
	return subcommands.ExitSuccess
}

type showCommand struct {
	runtime *runtime
	limit   int
	session string
	cwd     string
	status  string
	asJSON  bool
}

func (*showCommand) Name() string { return "show" }

func (*showCommand) Synopsis() string { return "show recorded events" }

func (*showCommand) Usage() string {
	return "show [--n N] [--session ID|NAME] [--cwd PREFIX] [--status ok|fail|any] [--json]\n" +
		"show <event-id> [--json]\n"
}

func (command *showCommand) SetFlags(fs *flag.FlagSet) {
	fs.IntVar(&command.limit, "n", defaultListLimit, "limit")
	fs.StringVar(&command.session, "session", "", "session id or name")
	fs.StringVar(&command.cwd, "cwd", "", "cwd prefix")
	fs.StringVar(&command.status, "status", string(store.StatusAny), "ok, fail, or any")
	fs.BoolVar(&command.asJSON, "json", false, "json output")
}

func (command *showCommand) Execute(_ context.Context, fs *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	_, root, runtimeErr := command.runtime.runtime()
	if runtimeErr != nil {
		fmt.Fprintf(command.runtime.stderr, "load runtime: %v\n", runtimeErr)
		return subcommands.ExitFailure
	}

	args := fs.Args()
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return command.showEvent(root, args[0], args[1:])
	}

	events, listErr := store.List(root, store.Filter{
		Limit:     command.limit,
		Session:   command.session,
		CWDPrefix: command.cwd,
		Status:    store.StatusFilter(command.status),
	})
	if listErr != nil {
		fmt.Fprintf(command.runtime.stderr, "show events: %v\n", listErr)
		return subcommands.ExitFailure
	}

	if command.asJSON {
		if jsonErr := printJSON(command.runtime.stdout, events); jsonErr != nil {
			fmt.Fprintf(command.runtime.stderr, "%v\n", jsonErr)
			return subcommands.ExitFailure
		}
		return subcommands.ExitSuccess
	}

	for _, event := range events {
		fmt.Fprintf(
			command.runtime.stdout,
			"%s  exit=%d  session=%s  cwd=%s\n",
			event.ID,
			event.ExitCode,
			event.SessionID,
			event.PWDBefore,
		)
		fmt.Fprintf(command.runtime.stdout, "  %s\n", event.Command)
	}

	return subcommands.ExitSuccess
}

func (command *showCommand) showEvent(root string, id string, args []string) subcommands.ExitStatus {
	event, findErr := store.FindByID(root, id)
	if findErr != nil {
		fmt.Fprintf(command.runtime.stderr, "show event: %v\n", findErr)
		return subcommands.ExitFailure
	}

	if printErr := printEvent(command.runtime.stdout, event, containsFlag(args, "--json")); printErr != nil {
		fmt.Fprintf(command.runtime.stderr, "%v\n", printErr)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

type searchCommand struct {
	runtime *runtime
	field   string
	limit   int
	asJSON  bool
}

func (*searchCommand) Name() string { return "search" }

func (*searchCommand) Synopsis() string { return "search recorded events" }

func (*searchCommand) Usage() string {
	return "search <query> [--field FIELD] [--n N] [--json]\n"
}

func (command *searchCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&command.field, "field", "all", "query field")
	fs.IntVar(&command.limit, "n", defaultListLimit, "limit")
	fs.BoolVar(&command.asJSON, "json", false, "json output")
}

func (command *searchCommand) Execute(_ context.Context, fs *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	_, root, runtimeErr := command.runtime.runtime()
	if runtimeErr != nil {
		fmt.Fprintf(command.runtime.stderr, "load runtime: %v\n", runtimeErr)
		return subcommands.ExitFailure
	}

	args := fs.Args()
	if len(args) == 0 {
		fmt.Fprintf(
			command.runtime.stderr,
			"usage: %s search <query> [--field FIELD] [--n N] [--json]\n",
			command.runtime.name,
		)
		return subcommands.ExitUsageError
	}

	query := args[0]
	searchFS := newFlagSet("search", command.runtime.stderr)
	searchFS.StringVar(&command.field, "field", command.field, "query field")
	searchFS.IntVar(&command.limit, "n", command.limit, "limit")
	searchFS.BoolVar(&command.asJSON, "json", command.asJSON, "json output")
	parseErr := searchFS.Parse(args[1:])
	if parseErr != nil {
		return subcommands.ExitUsageError
	}

	events, listErr := store.List(root, store.Filter{
		Limit:      command.limit,
		Query:      query,
		QueryField: command.field,
		Status:     store.StatusAny,
	})
	if listErr != nil {
		fmt.Fprintf(command.runtime.stderr, "search events: %v\n", listErr)
		return subcommands.ExitFailure
	}

	if command.asJSON {
		if jsonErr := printJSON(command.runtime.stdout, events); jsonErr != nil {
			fmt.Fprintf(command.runtime.stderr, "%v\n", jsonErr)
			return subcommands.ExitFailure
		}
		return subcommands.ExitSuccess
	}

	for _, event := range events {
		fmt.Fprintf(command.runtime.stdout, "%s  %s\n", event.ID, event.Command)
	}

	return subcommands.ExitSuccess
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func parseFinishedAt(raw string) (time.Time, error) {
	if raw == "" {
		return time.Now().UTC(), nil
	}

	return time.Parse(timeLayout, raw)
}

func (rt *runtime) runtime() (config.Config, string, error) {
	if rt.loaded {
		return rt.cfg, rt.root, nil
	}

	cfg, loadErr := config.Load()
	if loadErr != nil {
		return config.Config{}, "", loadErr
	}

	root, rootErr := paths.StateRoot()
	if rootErr != nil {
		return config.Config{}, "", rootErr
	}

	mkdirErr := os.MkdirAll(filepath.Join(root, "events"), 0o750)
	if mkdirErr != nil {
		return config.Config{}, "", fmt.Errorf("create state root: %w", mkdirErr)
	}

	rt.cfg = cfg
	rt.root = root
	rt.loaded = true

	return rt.cfg, rt.root, nil
}

func printEvent(stdout io.Writer, event store.Event, asJSON bool) error {
	if asJSON {
		return printJSON(stdout, event)
	}

	fmt.Fprintf(stdout, "id: %s\n", event.ID)
	fmt.Fprintf(stdout, "session: %s\n", event.SessionID)
	if event.SessionName != "" {
		fmt.Fprintf(stdout, "session_name: %s\n", event.SessionName)
	}
	fmt.Fprintf(stdout, "command: %s\n", event.Command)
	fmt.Fprintf(stdout, "exit_code: %d\n", event.ExitCode)
	fmt.Fprintf(stdout, "cwd_before: %s\n", event.PWDBefore)
	fmt.Fprintf(stdout, "cwd_after: %s\n", event.PWDAfter)
	fmt.Fprintf(stdout, "stdout:\n%s\n", event.StdoutText)
	fmt.Fprintf(stdout, "stderr:\n%s\n", event.StderrText)
	return nil
}

func printJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func containsFlag(args []string, target string) bool {
	return slices.Contains(args, target)
}

func displayName(invokedAs string) string {
	base := filepath.Base(strings.TrimSpace(invokedAs))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return defaultCLIName
	}

	return base
}

func commandRef(invokedAs string, fallback string) string {
	value := strings.TrimSpace(invokedAs)
	if value == "" {
		return fallback
	}
	if strings.ContainsRune(value, filepath.Separator) {
		absolute, err := filepath.Abs(value)
		if err == nil {
			return absolute
		}
	}

	return filepath.Base(value)
}
