package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/pterm/pterm"

	"package-operator.run/internal/cmd"
)

func init() {
	pterm.DisableColor()
}

// NewPrinter takes a variadic slice of PrinterOptions
// and returns a configured Printer instance.
func NewPrinter(opts ...PrinterOption) *Printer {
	var cfg PrinterConfig

	cfg.Option(opts...)
	cfg.Default()

	return &Printer{
		cfg: cfg,
	}
}

type Printer struct {
	cfg PrinterConfig
}

func (p *Printer) PrintfOut(s string, args ...any) error {
	if _, err := fmt.Fprintf(p.cfg.Out, s, args...); err != nil {
		return fmt.Errorf("printing to out stream: %w", err)
	}

	return nil
}

func (p *Printer) PrintfErr(s string, args ...any) error {
	if _, err := fmt.Fprintf(p.cfg.Err, s, args...); err != nil {
		return fmt.Errorf("printing to err stream: %w", err)
	}

	return nil
}

func (p *Printer) PrintTable(t cmd.Table) error {
	data := [][]string{}

	headers := t.Headers()

	if len(headers) > 0 {
		data = append(data, headers)
	}

	for _, r := range t.Rows() {
		vals := make([]string, 0, len(r))

		for _, f := range r {
			vals = append(vals, fmt.Sprint(f.Value))
		}

		data = append(data, vals)
	}

	table := pterm.DefaultTable.WithData(data).WithSeparator("  ")

	if len(headers) > 0 {
		table = table.WithHasHeader()
	}

	output, err := table.Srender()
	if err != nil {
		return fmt.Errorf("rendering table: %w", err)
	}

	if err := p.PrintfOut("%s\n", output); err != nil {
		return fmt.Errorf("printing table: %w", err)
	}

	return nil
}

type PrinterConfig struct {
	Out io.Writer
	Err io.Writer
}

func (c *PrinterConfig) Option(opts ...PrinterOption) {
	for _, opt := range opts {
		opt.ConfigurePrinter(c)
	}
}

func (c *PrinterConfig) Default() {
	if c.Out == nil {
		c.Out = os.Stdout
	}
	if c.Err == nil {
		c.Err = os.Stderr
	}
}

type PrinterOption interface {
	ConfigurePrinter(*PrinterConfig)
}
