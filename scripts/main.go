package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const usageText = `Usage:
  go run ./scripts build
  go run ./scripts check [-integration]

Commands:
  build              Build Windows, Linux, and macOS binaries for amd64 and arm64.
  check              Run unit tests followed by go vet.
  check -integration Also run disposable-database integration tests.
`

type options struct {
	command     string
	integration bool
	help        bool
}

type buildTarget struct {
	goos   string
	goarch string
}

var buildTargets = []buildTarget{
	{goos: "windows", goarch: "amd64"},
	{goos: "windows", goarch: "arm64"},
	{goos: "linux", goarch: "amd64"},
	{goos: "linux", goarch: "arm64"},
	{goos: "darwin", goarch: "amd64"},
	{goos: "darwin", goarch: "arm64"},
}

func main() {
	os.Exit(run(os.Args[1:], mustGetwd(), os.Environ(), os.Stdout, os.Stderr))
}

func mustGetwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "get current directory: %v\n", err)
		os.Exit(1)
	}
	return cwd
}

func run(args []string, cwd string, environment []string, stdout, stderr io.Writer) int {
	parsed, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprint(stderr, usageText)
		return 2
	}
	if parsed.help {
		fmt.Fprint(stdout, usageText)
		return 0
	}

	root, err := findRepoRoot(cwd)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	childEnvironment := withEnvironment(environment, map[string]string{
		"GOCACHE":    filepath.Join(root, ".cache", "build"),
		"GOPATH":     filepath.Join(root, ".cache", "go"),
		"GOMODCACHE": filepath.Join(root, ".cache", "mod"),
	})

	switch parsed.command {
	case "build":
		return runBuild(root, childEnvironment, stdout, stderr)
	case "check":
		return runCheck(root, childEnvironment, parsed.integration, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "internal error: unsupported command %q\n", parsed.command)
		return 1
	}
}

func parseOptions(args []string) (options, error) {
	if len(args) == 0 {
		return options{help: true}, nil
	}
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		return options{help: true}, nil
	}

	switch args[0] {
	case "build":
		if len(args) != 1 {
			return options{}, errors.New("build does not accept options or arguments")
		}
		return options{command: "build"}, nil
	case "check":
		parsed := options{command: "check"}
		for _, arg := range args[1:] {
			switch {
			case arg == "-integration" && !parsed.integration:
				parsed.integration = true
			case arg == "-integration":
				return options{}, errors.New("-integration may only be specified once")
			case strings.HasPrefix(arg, "-"):
				return options{}, fmt.Errorf("unknown check option: %s", arg)
			default:
				return options{}, fmt.Errorf("unexpected check argument: %s", arg)
			}
		}
		return parsed, nil
	default:
		return options{}, fmt.Errorf("unknown command: %s", args[0])
	}
}

func findRepoRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}
	for {
		info, err := os.Stat(filepath.Join(current, "go.mod"))
		if err == nil && !info.IsDir() {
			return current, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect repository root: %w", err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("could not find repository root from %s", start)
		}
		current = parent
	}
}

func runBuild(root string, environment []string, stdout, stderr io.Writer) int {
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o755); err != nil {
		fmt.Fprintf(stderr, "create dist directory: %v\n", err)
		return 1
	}
	for _, target := range buildTargets {
		suffix := ""
		if target.goos == "windows" {
			suffix = ".exe"
		}
		output := filepath.Join("dist", "sqlterm-"+target.goos+"-"+target.goarch+suffix)
		targetEnvironment := withEnvironment(environment, map[string]string{
			"CGO_ENABLED": "0",
			"GOARCH":      target.goarch,
			"GOOS":        target.goos,
		})
		err := runGo(root, targetEnvironment, stdout, stderr, "build", "-trimpath", "-o", output, "./cmd/sqlterm")
		if err != nil {
			fmt.Fprintf(stderr, "go build failed for %s/%s: %v\n", target.goos, target.goarch, err)
			return exitCode(err)
		}
		fmt.Fprintf(stdout, "Built %s\n", output)
	}
	return 0
}

func runCheck(root string, environment []string, integration bool, stdout, stderr io.Writer) int {
	commands := []struct {
		name string
		args []string
	}{
		{name: "go test", args: []string{"test", "./..."}},
		{name: "go vet", args: []string{"vet", "./..."}},
	}
	if integration {
		commands = append(commands, struct {
			name string
			args []string
		}{name: "integration tests", args: []string{"test", "./internal/db", "-run", "TestIntegration", "-v", "-count=1"}})
	}
	for _, command := range commands {
		if err := runGo(root, environment, stdout, stderr, command.args...); err != nil {
			fmt.Fprintf(stderr, "%s failed: %v\n", command.name, err)
			return exitCode(err)
		}
	}
	return 0
}

func runGo(root string, environment []string, stdout, stderr io.Writer, args ...string) error {
	command := exec.Command("go", args...)
	command.Dir = root
	command.Env = environment
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func withEnvironment(base []string, overrides map[string]string) []string {
	result := append([]string(nil), base...)
	for key, value := range overrides {
		replaced := false
		for i, entry := range result {
			separator := strings.IndexByte(entry, '=')
			if separator < 0 || !environmentKeyEqual(entry[:separator], key) {
				continue
			}
			result[i] = key + "=" + value
			replaced = true
			break
		}
		if !replaced {
			result = append(result, key+"="+value)
		}
	}
	return result
}

func environmentKeyEqual(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func exitCode(err error) int {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if code := exitError.ExitCode(); code >= 0 {
			return code
		}
	}
	return 1
}
