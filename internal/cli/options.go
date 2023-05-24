package cli

import "io"

// WithOut configures the Out stream
// to the given io.Writer implementations.
type WithOut struct{ Out io.Writer }

func (w WithOut) ConfigurePrinter(c *PrinterConfig) {
	c.Out = w.Out
}

// WithErr configures the Err stream
// to the given io.Writer implementations.
type WithErr struct{ Err io.Writer }

func (w WithErr) ConfigurePrinter(c *PrinterConfig) {
	c.Err = w.Err
}
