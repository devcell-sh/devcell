package main

import (
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// usageError is a flag or argument mistake by the user. Execute prints it
// without the version banner, which exists for bug reports, not for typos.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

var flagNameRe = regexp.MustCompile(`--[A-Za-z0-9][A-Za-z0-9-]*`)

// flagUsageError turns a pflag parse error such as "flag needs an argument:
// --engine" into a usageError that also shows the flag's own help line (when
// the flag is known to cmd) and points at the command's --help.
func flagUsageError(cmd *cobra.Command, err error) error {
	var b strings.Builder
	b.WriteString(err.Error())
	if name := flagNameRe.FindString(err.Error()); name != "" {
		if f := lookupFlag(cmd, strings.TrimPrefix(name, "--")); f != nil {
			fs := pflag.NewFlagSet("", pflag.ContinueOnError)
			fs.AddFlag(f)
			b.WriteString("\n\n")
			b.WriteString(strings.TrimRight(fs.FlagUsages(), "\n"))
		}
	}
	b.WriteString("\n\nRun '" + cmd.CommandPath() + " --help' for all flags.")
	return &usageError{err: &textError{msg: b.String()}}
}

// lookupFlag finds a flag on cmd or any of its parents, including persistent
// flags that cobra has not merged yet.
func lookupFlag(cmd *cobra.Command, name string) *pflag.Flag {
	for c := cmd; c != nil; c = c.Parent() {
		if f := c.Flags().Lookup(name); f != nil {
			return f
		}
		if f := c.PersistentFlags().Lookup(name); f != nil {
			return f
		}
	}
	return nil
}

type textError struct{ msg string }

func (e *textError) Error() string { return e.msg }
