package scaffold

// RunInitFlow tests stay offline; ResolveNixhome's clone path has its own
// tests.
func init() {
	resolveNixhome = func(string, string, string, bool) error { return nil }
	detectOllama = func() string { return "" }
}
