package plugins

import "fmt"

var (
	formatters = map[string]InitFormatter{}
	parsers    = map[string]InitParser{}
)

// InitFormatter is a function type that takes a `Config` object as input and returns
// a `Formatter` and an error. It is used to initialize and register formatters.
type InitFormatter func(cfg Config) (Formatter, error)

// InitParser is a function type that takes a `Config` object as input and returns
// a `Parser` and an error. It is used to initialize and register parsers.
type InitParser func(cfg Config) (Parser, error)

// Register registers a formatter and parser with the given plugin name.
// It returns an error if a formatter or parser with the same name has already been registered.
func Register(name string, f InitFormatter, p InitParser) error {
	if _, ok := formatters[name]; ok {
		return fmt.Errorf("duplicate formatter registration for %q", name)
	}
	if _, ok := parsers[name]; ok {
		return fmt.Errorf("duplicate parser registration for %q", name)
	}
	if f == nil || p == nil {
		return fmt.Errorf("cannot register a nil formatter or parser")
	}
	formatters[name] = f
	parsers[name] = p
	return nil
}
