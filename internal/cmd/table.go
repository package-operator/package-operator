package cmd

import "strings"

// Table provides a generic table interface
// for Printer's to consume table data from.
type Table interface {
	// Headers returns the table's headers if any.
	Headers() []string
	// Rows returns a 2-dimensional slice of Fields
	// representing the Table data.
	Rows() [][]Field
}

// NewDefaultTable returns a Table implementation which
// will only select Fields from each row if the Field's
// name matches the provided Headers. If no Headers are
// provided all fields will be present in the table Data.
func NewDefaultTable(opts ...TableOption) *DefaultTable {
	var cfg TableConfig

	cfg.Option(opts...)

	return &DefaultTable{
		cfg: cfg,
	}
}

type DefaultTable struct {
	cfg TableConfig

	data table
}

func (t *DefaultTable) Headers() []string {
	return t.cfg.Headers
}

func (t *DefaultTable) AddRow(fields ...Field) {
	t.data = append(t.data, row(fields))
}

func (t *DefaultTable) Rows() [][]Field {
	if len(t.cfg.Headers) == 0 {
		return t.data.Fields()
	}

	res := make([][]Field, 0, len(t.data))

	for _, r := range t.data {
		row := r.SelectFields(t.cfg.Headers...)
		if len(row) < 1 {
			continue
		}

		res = append(res, row)
	}

	return res
}

type TableConfig struct {
	Headers []string
}

func (c *TableConfig) Option(opts ...TableOption) {
	for _, opt := range opts {
		opt.ConfigureTable(c)
	}
}

type TableOption interface {
	ConfigureTable(*TableConfig)
}

type table []row

func (t table) Fields() [][]Field {
	res := make([][]Field, 0, len(t))

	for _, r := range t {
		res = append(res, []Field(r))
	}

	return res
}

type row []Field

func (r row) SelectFields(names ...string) []Field {
	var res row

	for _, n := range names {
		if ok, f := r.GetField(n); ok {
			res = append(res, f)
		}
	}

	return res
}

func (r row) GetField(name string) (bool, Field) {
	for _, f := range r {
		if normalize(f.Name) == normalize(name) {
			return true, f
		}
	}

	return false, Field{}
}

func normalize(name string) string {
	trimmed := strings.TrimSpace(name)
	snaked := strings.Join(strings.Fields(trimmed), "_")

	return strings.ToLower(snaked)
}

type Field struct {
	Name  string
	Value any
}
