package engine_test

import (
	"fmt"

	"github.com/DimmKirr/devcell/internal/engine"
)

// Example demonstrates looking up a registered engine by name.
// Docker, tart, and winkit register themselves via init(); call For
// to get the implementation.
func Example() {
	_, err := engine.For("nonexistent")
	fmt.Println(err)
	// Output:
	// engine "nonexistent" is not available in this build; see https://github.com/DimmKirr/devcell/issues
}
