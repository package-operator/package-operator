// +build ignore

package main

import (
	"context"
	_flag "flag"
	_fmt "fmt"
	_ioutil "io/ioutil"
	_log "log"
	"os"
	_filepath "path/filepath"
	_sort "sort"
	"strconv"
	_strings "strings"
	_tabwriter "text/tabwriter"
	"time"
	
)

func main() {
	// Use local types and functions in order to avoid name conflicts with additional magefiles.
	type arguments struct {
		Verbose       bool          // print out log statements
		List          bool          // print out a list of targets
		Help          bool          // print out help for a specific target
		Timeout       time.Duration // set a timeout to running the targets
		Args          []string      // args contain the non-flag command-line arguments
	}

	parseBool := func(env string) bool {
		val := os.Getenv(env)
		if val == "" {
			return false
		}		
		b, err := strconv.ParseBool(val)
		if err != nil {
			_log.Printf("warning: environment variable %s is not a valid bool value: %v", env, val)
			return false
		}
		return b
	}

	parseDuration := func(env string) time.Duration {
		val := os.Getenv(env)
		if val == "" {
			return 0
		}		
		d, err := time.ParseDuration(val)
		if err != nil {
			_log.Printf("warning: environment variable %s is not a valid duration value: %v", env, val)
			return 0
		}
		return d
	}
	args := arguments{}
	fs := _flag.FlagSet{}
	fs.SetOutput(os.Stdout)

	// default flag set with ExitOnError and auto generated PrintDefaults should be sufficient
	fs.BoolVar(&args.Verbose, "v", parseBool("MAGEFILE_VERBOSE"), "show verbose output when running targets")
	fs.BoolVar(&args.List, "l", parseBool("MAGEFILE_LIST"), "list targets for this binary")
	fs.BoolVar(&args.Help, "h", parseBool("MAGEFILE_HELP"), "print out help for a specific target")
	fs.DurationVar(&args.Timeout, "t", parseDuration("MAGEFILE_TIMEOUT"), "timeout in duration parsable format (e.g. 5m30s)")
	fs.Usage = func() {
		_fmt.Fprintf(os.Stdout, `
%s [options] [target]

Commands:
  -l    list targets in this binary
  -h    show this help

Options:
  -h    show description of a target
  -t <string>
        timeout in duration parsable format (e.g. 5m30s)
  -v    show verbose output when running targets
 `[1:], _filepath.Base(os.Args[0]))
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		// flag will have printed out an error already.
		return
	}
	args.Args = fs.Args()
	if args.Help && len(args.Args) == 0 {
		fs.Usage()
		return
	}
		
	// color is ANSI color type
	type color int

	// If you add/change/remove any items in this constant,
	// you will need to run "stringer -type=color" in this directory again.
	// NOTE: Please keep the list in an alphabetical order.
	const (
		black color = iota
		red
		green
		yellow
		blue
		magenta
		cyan
		white
		brightblack
		brightred
		brightgreen
		brightyellow
		brightblue
		brightmagenta
		brightcyan
		brightwhite
	)

	// AnsiColor are ANSI color codes for supported terminal colors.
	var ansiColor = map[color]string{
		black:         "\u001b[30m",
		red:           "\u001b[31m",
		green:         "\u001b[32m",
		yellow:        "\u001b[33m",
		blue:          "\u001b[34m",
		magenta:       "\u001b[35m",
		cyan:          "\u001b[36m",
		white:         "\u001b[37m",
		brightblack:   "\u001b[30;1m",
		brightred:     "\u001b[31;1m",
		brightgreen:   "\u001b[32;1m",
		brightyellow:  "\u001b[33;1m",
		brightblue:    "\u001b[34;1m",
		brightmagenta: "\u001b[35;1m",
		brightcyan:    "\u001b[36;1m",
		brightwhite:   "\u001b[37;1m",
	}
	
	const _color_name = "blackredgreenyellowbluemagentacyanwhitebrightblackbrightredbrightgreenbrightyellowbrightbluebrightmagentabrightcyanbrightwhite"

	var _color_index = [...]uint8{0, 5, 8, 13, 19, 23, 30, 34, 39, 50, 59, 70, 82, 92, 105, 115, 126}

	colorToLowerString := func (i color) string {
		if i < 0 || i >= color(len(_color_index)-1) {
			return "color(" + strconv.FormatInt(int64(i), 10) + ")"
		}
		return _color_name[_color_index[i]:_color_index[i+1]]
	}

	// ansiColorReset is an ANSI color code to reset the terminal color.
	const ansiColorReset = "\033[0m"

	// defaultTargetAnsiColor is a default ANSI color for colorizing targets.
	// It is set to Cyan as an arbitrary color, because it has a neutral meaning
	var defaultTargetAnsiColor = ansiColor[cyan]

	getAnsiColor := func(color string) (string, bool) {
		colorLower := _strings.ToLower(color)
		for k, v := range ansiColor {
			colorConstLower := colorToLowerString(k)
			if colorConstLower == colorLower {
				return v, true
			}
		}
		return "", false
	}

	// Terminals which  don't support color:
	// 	TERM=vt100
	// 	TERM=cygwin
	// 	TERM=xterm-mono
    var noColorTerms = map[string]bool{
		"vt100":      false,
		"cygwin":     false,
		"xterm-mono": false,
	}

	// terminalSupportsColor checks if the current console supports color output
	//
	// Supported:
	// 	linux, mac, or windows's ConEmu, Cmder, putty, git-bash.exe, pwsh.exe
	// Not supported:
	// 	windows cmd.exe, powerShell.exe
	terminalSupportsColor := func() bool {
		envTerm := os.Getenv("TERM")
		if _, ok := noColorTerms[envTerm]; ok {
			return false
		}
		return true
	}

	// enableColor reports whether the user has requested to enable a color output.
	enableColor := func() bool {
		b, _ := strconv.ParseBool(os.Getenv("MAGEFILE_ENABLE_COLOR"))
		return b
	}

	// targetColor returns the ANSI color which should be used to colorize targets.
	targetColor := func() string {
		s, exists := os.LookupEnv("MAGEFILE_TARGET_COLOR")
		if exists == true {
			if c, ok := getAnsiColor(s); ok == true {
				return c
			}
		}
		return defaultTargetAnsiColor
	}

	// store the color terminal variables, so that the detection isn't repeated for each target
	var enableColorValue = enableColor() && terminalSupportsColor()
	var targetColorValue = targetColor()

	printName := func(str string) string {
		if enableColorValue {
			return _fmt.Sprintf("%s%s%s", targetColorValue, str, ansiColorReset)
		} else {
			return str
		}
	}

	list := func() error {
		
		targets := map[string]string{
			"build:binaries": "Build all PKO binaries for the architecture of this machine.",
			"build:binary": "Builds binaries from /cmd directory.",
			"build:image": "Builds the given container image, building binaries as prerequisite as required.",
			"build:images": "Builds all PKO container images.",
			"build:pushImage": "Builds and pushes only the given container image to the default registry.",
			"build:pushImages": "Builds and pushes all container images to the default registry.",
			"build:releaseBinaries": "",
			"dependency:all": "Installs all project dependencies into the local checkout.",
			"dependency:controllerGen": "Ensure controller-gen - kubebuilder code and manifest generator.",
			"dependency:crane": "",
			"dependency:docgen": "",
			"dependency:golangciLint": "",
			"dependency:helm": "",
			"dependency:kind": "Ensure Kind dependency - Kubernetes in Docker (or Podman)",
			"deploy": "",
			"dev:deploy": "Setup local cluster and deploy the Package Operator.",
			"dev:integration": "Setup local dev environment with the package operator installed and run the integration test suite.",
			"dev:load": "images into the development environment.",
			"dev:setup": "Creates an empty development environment via kind.",
			"dev:teardown": "Tears the whole kind development environment down.",
			"generate:all": "Run all code generators.",
			"generate:packageOperatorPackage": "Includes all static-deployment files in the package-operator-package.",
			"generate:remotePhasePackage": "Includes all static-deployment files in the remote-phase-package.",
			"generate:selfBootstrapJob": "generates a self-bootstrap-job.yaml based on the current VERSION.",
			"test:fixLint": "Runs linters.",
			"test:goModTidy": "",
			"test:golangCILint": "",
			"test:golangCILintFix": "",
			"test:integration": "Runs the given integration suite(s) as given by the first positional argument.",
			"test:lint": "",
			"test:packageOperatorIntegrationRun": "Runs PKO integration tests against whatever cluster your KUBECONFIG is pointing at.",
			"test:unit": "Runs unittests.",
			"test:validateGitClean": "",
		}

		keys := make([]string, 0, len(targets))
		for name := range targets {
			keys = append(keys, name)
		}
		_sort.Strings(keys)

		_fmt.Println("Targets:")
		w := _tabwriter.NewWriter(os.Stdout, 0, 4, 4, ' ', 0)
		for _, name := range keys {
			_fmt.Fprintf(w, "  %v\t%v\n", printName(name), targets[name])
		}
		err := w.Flush()
		return err
	}

	var ctx context.Context
	var ctxCancel func()

	getContext := func() (context.Context, func()) {
		if ctx != nil {
			return ctx, ctxCancel
		}

		if args.Timeout != 0 {
			ctx, ctxCancel = context.WithTimeout(context.Background(), args.Timeout)
		} else {
			ctx = context.Background()
			ctxCancel = func() {}
		}
		return ctx, ctxCancel
	}

	runTarget := func(fn func(context.Context) error) interface{} {
		var err interface{}
		ctx, cancel := getContext()
		d := make(chan interface{})
		go func() {
			defer func() {
				err := recover()
				d <- err
			}()
			err := fn(ctx)
			d <- err
		}()
		select {
		case <-ctx.Done():
			cancel()
			e := ctx.Err()
			_fmt.Printf("ctx err: %v\n", e)
			return e
		case err = <-d:
			cancel()
			return err
		}
	}
	// This is necessary in case there aren't any targets, to avoid an unused
	// variable error.
	_ = runTarget

	handleError := func(logger *_log.Logger, err interface{}) {
		if err != nil {
			logger.Printf("Error: %+v\n", err)
			type code interface {
				ExitStatus() int
			}
			if c, ok := err.(code); ok {
				os.Exit(c.ExitStatus())
			}
			os.Exit(1)
		}
	}
	_ = handleError

	// Set MAGEFILE_VERBOSE so mg.Verbose() reflects the flag value.
	if args.Verbose {
		os.Setenv("MAGEFILE_VERBOSE", "1")
	} else {
		os.Setenv("MAGEFILE_VERBOSE", "0")
	}

	_log.SetFlags(0)
	if !args.Verbose {
		_log.SetOutput(_ioutil.Discard)
	}
	logger := _log.New(os.Stderr, "", 0)
	if args.List {
		if err := list(); err != nil {
			_log.Println(err)
			os.Exit(1)
		}
		return
	}

	if args.Help {
		if len(args.Args) < 1 {
			logger.Println("no target specified")
			os.Exit(2)
		}
		switch _strings.ToLower(args.Args[0]) {
			case "build:binaries":
				_fmt.Println("Build all PKO binaries for the architecture of this machine.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage build:binaries\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "build:binary":
				_fmt.Println("Builds binaries from /cmd directory.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage build:binary <cmd> <goos> <goarch>\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "build:image":
				_fmt.Println("Builds the given container image, building binaries as prerequisite as required.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage build:image <name>\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "build:images":
				_fmt.Println("Builds all PKO container images.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage build:images\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "build:pushimage":
				_fmt.Println("Builds and pushes only the given container image to the default registry.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage build:pushimage <imageName>\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "build:pushimages":
				_fmt.Println("Builds and pushes all container images to the default registry.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage build:pushimages\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "build:releasebinaries":
				
				_fmt.Print("Usage:\n\n\tmage build:releasebinaries\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dependency:all":
				_fmt.Println("Installs all project dependencies into the local checkout.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage dependency:all\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dependency:controllergen":
				_fmt.Println("Ensure controller-gen - kubebuilder code and manifest generator.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage dependency:controllergen\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dependency:crane":
				
				_fmt.Print("Usage:\n\n\tmage dependency:crane\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dependency:docgen":
				
				_fmt.Print("Usage:\n\n\tmage dependency:docgen\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dependency:golangcilint":
				
				_fmt.Print("Usage:\n\n\tmage dependency:golangcilint\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dependency:helm":
				
				_fmt.Print("Usage:\n\n\tmage dependency:helm\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dependency:kind":
				_fmt.Println("Ensure Kind dependency - Kubernetes in Docker (or Podman)")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage dependency:kind\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "deploy":
				
				_fmt.Print("Usage:\n\n\tmage deploy\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dev:deploy":
				_fmt.Println("Setup local cluster and deploy the Package Operator.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage dev:deploy\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dev:integration":
				_fmt.Println("Setup local dev environment with the package operator installed and run the integration test suite.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage dev:integration\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dev:load":
				_fmt.Println("Load images into the development environment.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage dev:load\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dev:setup":
				_fmt.Println("Creates an empty development environment via kind.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage dev:setup\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "dev:teardown":
				_fmt.Println("Tears the whole kind development environment down.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage dev:teardown\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "generate:all":
				_fmt.Println("Run all code generators. installYamlFile has to come after code generation")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage generate:all\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "generate:packageoperatorpackage":
				_fmt.Println("Includes all static-deployment files in the package-operator-package.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage generate:packageoperatorpackage\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "generate:remotephasepackage":
				_fmt.Println("Includes all static-deployment files in the remote-phase-package.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage generate:remotephasepackage\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "generate:selfbootstrapjob":
				_fmt.Println("generates a self-bootstrap-job.yaml based on the current VERSION. requires the images to have been build beforehand.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage generate:selfbootstrapjob\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "test:fixlint":
				_fmt.Println("Runs linters.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage test:fixlint\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "test:gomodtidy":
				
				_fmt.Print("Usage:\n\n\tmage test:gomodtidy\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "test:golangcilint":
				
				_fmt.Print("Usage:\n\n\tmage test:golangcilint\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "test:golangcilintfix":
				
				_fmt.Print("Usage:\n\n\tmage test:golangcilintfix\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "test:integration":
				_fmt.Println("Runs the given integration suite(s) as given by the first positional argument. The options are 'all', 'all-local', 'kubectl-package', 'package-operator', and 'package-operator-local'.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage test:integration <suite>\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "test:lint":
				
				_fmt.Print("Usage:\n\n\tmage test:lint\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "test:packageoperatorintegrationrun":
				_fmt.Println("Runs PKO integration tests against whatever cluster your KUBECONFIG is pointing at. Also allows specifying only sub tests to run e.g. ./mage test:integrationrun TestPackage_success")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage test:packageoperatorintegrationrun <filter>\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "test:unit":
				_fmt.Println("Runs unittests.")
				_fmt.Println()
				
				_fmt.Print("Usage:\n\n\tmage test:unit\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			case "test:validategitclean":
				
				_fmt.Print("Usage:\n\n\tmage test:validategitclean\n\n")
				var aliases []string
				if len(aliases) > 0 {
					_fmt.Printf("Aliases: %s\n\n", _strings.Join(aliases, ", "))
				}
				return
			default:
				logger.Printf("Unknown target: %q\n", args.Args[0])
				os.Exit(2)
		}
	}
	if len(args.Args) < 1 {
		if err := list(); err != nil {
			logger.Println("Error:", err)
			os.Exit(1)
		}
		return
	}
	for x := 0; x < len(args.Args); {
		target := args.Args[x]
		x++

		// resolve aliases
		switch _strings.ToLower(target) {
		
		}

		switch _strings.ToLower(target) {
		
			case "build:binaries":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Build:Binaries\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Build:Binaries")
				}
				
				wrapFn := func(ctx context.Context) error {
					Build{}.Binaries()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "build:binary":
				expected := x + 3
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Build:Binary\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Build:Binary")
				}
				
			arg0 := args.Args[x]
			x++
			arg1 := args.Args[x]
			x++
			arg2 := args.Args[x]
			x++
				wrapFn := func(ctx context.Context) error {
					Build{}.Binary(arg0, arg1, arg2)
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "build:image":
				expected := x + 1
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Build:Image\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Build:Image")
				}
				
			arg0 := args.Args[x]
			x++
				wrapFn := func(ctx context.Context) error {
					Build{}.Image(arg0)
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "build:images":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Build:Images\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Build:Images")
				}
				
				wrapFn := func(ctx context.Context) error {
					Build{}.Images()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "build:pushimage":
				expected := x + 1
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Build:PushImage\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Build:PushImage")
				}
				
			arg0 := args.Args[x]
			x++
				wrapFn := func(ctx context.Context) error {
					Build{}.PushImage(arg0)
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "build:pushimages":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Build:PushImages\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Build:PushImages")
				}
				
				wrapFn := func(ctx context.Context) error {
					Build{}.PushImages()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "build:releasebinaries":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Build:ReleaseBinaries\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Build:ReleaseBinaries")
				}
				
				wrapFn := func(ctx context.Context) error {
					Build{}.ReleaseBinaries()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dependency:all":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dependency:All\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dependency:All")
				}
				
				wrapFn := func(ctx context.Context) error {
					Dependency{}.All()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dependency:controllergen":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dependency:ControllerGen\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dependency:ControllerGen")
				}
				
				wrapFn := func(ctx context.Context) error {
					return Dependency{}.ControllerGen()
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dependency:crane":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dependency:Crane\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dependency:Crane")
				}
				
				wrapFn := func(ctx context.Context) error {
					return Dependency{}.Crane()
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dependency:docgen":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dependency:Docgen\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dependency:Docgen")
				}
				
				wrapFn := func(ctx context.Context) error {
					return Dependency{}.Docgen()
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dependency:golangcilint":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dependency:GolangciLint\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dependency:GolangciLint")
				}
				
				wrapFn := func(ctx context.Context) error {
					return Dependency{}.GolangciLint()
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dependency:helm":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dependency:Helm\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dependency:Helm")
				}
				
				wrapFn := func(ctx context.Context) error {
					return Dependency{}.Helm()
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dependency:kind":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dependency:Kind\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dependency:Kind")
				}
				
				wrapFn := func(ctx context.Context) error {
					return Dependency{}.Kind()
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "deploy":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Deploy\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Deploy")
				}
				
				wrapFn := func(ctx context.Context) error {
					Deploy(ctx)
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dev:deploy":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dev:Deploy\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dev:Deploy")
				}
				
				wrapFn := func(ctx context.Context) error {
					Dev{}.Deploy(ctx)
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dev:integration":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dev:Integration\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dev:Integration")
				}
				
				wrapFn := func(ctx context.Context) error {
					Dev{}.Integration(ctx)
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dev:load":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dev:Load\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dev:Load")
				}
				
				wrapFn := func(ctx context.Context) error {
					Dev{}.Load()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dev:setup":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dev:Setup\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dev:Setup")
				}
				
				wrapFn := func(ctx context.Context) error {
					Dev{}.Setup(ctx)
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "dev:teardown":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Dev:Teardown\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Dev:Teardown")
				}
				
				wrapFn := func(ctx context.Context) error {
					Dev{}.Teardown(ctx)
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "generate:all":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Generate:All\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Generate:All")
				}
				
				wrapFn := func(ctx context.Context) error {
					Generate{}.All()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "generate:packageoperatorpackage":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Generate:PackageOperatorPackage\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Generate:PackageOperatorPackage")
				}
				
				wrapFn := func(ctx context.Context) error {
					return Generate{}.PackageOperatorPackage()
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "generate:remotephasepackage":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Generate:RemotePhasePackage\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Generate:RemotePhasePackage")
				}
				
				wrapFn := func(ctx context.Context) error {
					return Generate{}.RemotePhasePackage()
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "generate:selfbootstrapjob":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Generate:SelfBootstrapJob\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Generate:SelfBootstrapJob")
				}
				
				wrapFn := func(ctx context.Context) error {
					Generate{}.SelfBootstrapJob()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "test:fixlint":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Test:FixLint\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Test:FixLint")
				}
				
				wrapFn := func(ctx context.Context) error {
					Test{}.FixLint()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "test:gomodtidy":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Test:GoModTidy\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Test:GoModTidy")
				}
				
				wrapFn := func(ctx context.Context) error {
					Test{}.GoModTidy()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "test:golangcilint":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Test:GolangCILint\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Test:GolangCILint")
				}
				
				wrapFn := func(ctx context.Context) error {
					Test{}.GolangCILint()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "test:golangcilintfix":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Test:GolangCILintFix\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Test:GolangCILintFix")
				}
				
				wrapFn := func(ctx context.Context) error {
					Test{}.GolangCILintFix()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "test:integration":
				expected := x + 1
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Test:Integration\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Test:Integration")
				}
				
			arg0 := args.Args[x]
			x++
				wrapFn := func(ctx context.Context) error {
					Test{}.Integration(ctx, arg0)
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "test:lint":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Test:Lint\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Test:Lint")
				}
				
				wrapFn := func(ctx context.Context) error {
					Test{}.Lint()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "test:packageoperatorintegrationrun":
				expected := x + 1
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Test:PackageOperatorIntegrationRun\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Test:PackageOperatorIntegrationRun")
				}
				
			arg0 := args.Args[x]
			x++
				wrapFn := func(ctx context.Context) error {
					Test{}.PackageOperatorIntegrationRun(ctx, arg0)
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "test:unit":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Test:Unit\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Test:Unit")
				}
				
				wrapFn := func(ctx context.Context) error {
					Test{}.Unit()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
			case "test:validategitclean":
				expected := x + 0
				if expected > len(args.Args) {
					// note that expected and args at this point include the arg for the target itself
					// so we subtract 1 here to show the number of args without the target.
					logger.Printf("not enough arguments for target \"Test:ValidateGitClean\", expected %v, got %v\n", expected-1, len(args.Args)-1)
					os.Exit(2)
				}
				if args.Verbose {
					logger.Println("Running target:", "Test:ValidateGitClean")
				}
				
				wrapFn := func(ctx context.Context) error {
					Test{}.ValidateGitClean()
					return nil
				}
				ret := runTarget(wrapFn)
				handleError(logger, ret)
		
		default:
			logger.Printf("Unknown target specified: %q\n", target)
			os.Exit(2)
		}
	}
}




