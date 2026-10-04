package winkit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DimmKirr/devcell/internal/cell"
)

func TestTemplateDir(t *testing.T) {
	dir := templateDir("/home/user", guestPE, "base", nil)
	assert.Equal(t, "/home/user/.devcell/windows/base", dir)
}

func TestTemplateDir_UsesStackTag(t *testing.T) {
	mods := []string{"wine", "social"}
	dir := templateDir("/home/user", guestPE, "dev", mods)
	assert.Equal(t, "/home/user/.devcell/windows/"+cell.StackTag("dev", mods), dir)
}

func TestInstanceDir(t *testing.T) {
	dir := InstanceDir("/home/user", "main")
	assert.Equal(t, "/home/user/.devcell/main/windows", dir)
}
