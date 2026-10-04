package cell

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStackTag_StackOnly(t *testing.T) {
	assert.Equal(t, "ultimate", StackTag("ultimate", nil))
}

func TestStackTag_WithModules(t *testing.T) {
	got := StackTag("dev", []string{"plex", "linear"})
	assert.Regexp(t, `^dev-linear-plex-[0-9a-f]{8}$`, got, "stack, sorted modules, 8-char sha suffix")
}

func TestStackTag_ModuleOrderDoesNotMatter(t *testing.T) {
	assert.Equal(t, StackTag("dev", []string{"b", "a"}), StackTag("dev", []string{"a", "b"}))
}

func TestStackTag_DoesNotMutateInput(t *testing.T) {
	mods := []string{"zed", "alpha"}
	StackTag("dev", mods)
	assert.Equal(t, []string{"zed", "alpha"}, mods)
}
