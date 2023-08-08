package rolloutcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"package-operator.run/internal/cli"
	internalcmd "package-operator.run/internal/cmd"
)

func newPrinter(p *cli.Printer) *printer {
	return &printer{
		printer: p,
	}
}

type printer struct {
	printer *cli.Printer
}

func (p *printer) PrintObjectSet(os internalcmd.ObjectSet, opts options) error {
	var (
		data []byte
		err  error
	)

	switch strings.ToLower(opts.Output) {
	case "json":
		data, err = json.MarshalIndent(&os, "", "    ")

		data = append(data, '\n')
	case "", "yaml":
		data, err = os.MarshalYAML()
	default:
		return fmt.Errorf("%w: %q", errInvalidOutputFormat, opts.Output)
	}
	if err != nil {
		return err
	}

	return p.printer.PrintfOut(string(data))
}

func (p *printer) PrintObjectSetList(l internalcmd.ObjectSetList, opts options) error {
	switch strings.ToLower(opts.Output) {
	case "json":
		data, err := l.RenderJSON()
		if err != nil {
			return fmt.Errorf("rendering object set list to json: %w", err)
		}

		return p.printer.PrintfOut(string(data) + "\n")
	case "yaml":
		data, err := l.RenderYAML()
		if err != nil {
			return fmt.Errorf("rendering object set list to yaml: %w", err)
		}

		return p.printer.PrintfOut(string(data))
	case "":
		table := l.RenderTable("REVISION", "SUCCESSFUL", "CHANGE-CAUSE")

		return p.printer.PrintTable(table)
	default:
		return fmt.Errorf("%w: %q", errInvalidOutputFormat, opts.Output)
	}
}

var errInvalidOutputFormat = errors.New("invalid output format")
