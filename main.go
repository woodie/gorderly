package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	style, passthrough, err := parseFlags(args)
	if err != nil {
		warn(stderr, err)
		return 2
	}

	input, wait, err := openInput(stdin, passthrough, stderr)
	if err != nil {
		warn(stderr, err)
		return 1
	}

	pkgs, err := Parse(input)
	if err != nil {
		warn(stderr, err)
		return 1
	}

	// wait() drains any subprocess exit status only after Parse has fully
	// read stdout to EOF -- calling it earlier would deadlock on a full pipe.
	if wait != nil {
		if werr := wait(); werr != nil {
			if _, ok := werr.(*exec.ExitError); !ok {
				warn(stderr, werr)
				return 1
			}
		}
	}

	colorEnabled := os.Getenv("NO_COLOR") == ""
	failed, err := Render(pkgs, style, stdout, colorEnabled)
	if err != nil {
		warn(stderr, err)
		return 1
	}
	if failed > 0 {
		return 1
	}
	return 0
}

func warn(w io.Writer, err error) {
	_, _ = fmt.Fprintf(w, "gorderly: %v\n", err)
}

func parseFlags(args []string) (style Style, passthrough []string, err error) {
	style = StyleClassic
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-fd":
			style = StyleFd
		case "-fs":
			style = StyleFs
		case "--format":
			i++
			if i >= len(args) {
				return style, nil, fmt.Errorf("--format requires an argument (documentation|spec)")
			}
			switch args[i] {
			case "documentation":
				style = StyleFd
			case "spec":
				style = StyleFs
			default:
				return style, nil, fmt.Errorf("unknown --format %q", args[i])
			}
		default:
			passthrough = append(passthrough, args[i])
		}
	}
	return style, passthrough, nil
}

// openInput reads piped stdin directly (the xctidy-style "raw output piped
// straight in" path) when nothing is piped in via stdin AND no package
// path was given it instead shells out to `go test -v` itself, the
// ginkgo-fd-style wrapper convenience path, so `gorderly -fd .` and
// `go test -v ./... | gorderly -fd` both work. wait is nil for the stdin path.
func openInput(stdin io.Reader, passthrough []string, stderr io.Writer) (io.Reader, func() error, error) {
	if f, ok := stdin.(*os.File); ok && len(passthrough) == 0 {
		if stat, statErr := f.Stat(); statErr == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
			return stdin, nil, nil
		}
	}

	if len(passthrough) == 0 {
		passthrough = []string{"./..."}
	}
	cmd := exec.Command("go", append([]string{"test", "-v"}, passthrough...)...)
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return stdout, cmd.Wait, nil
}
