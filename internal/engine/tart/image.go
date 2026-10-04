package tart

import "github.com/DimmKirr/devcell/internal/cell"

// BaseTemplateName is the tart VM name for a stack-agnostic base template
// (nix installed, store on external disk, but no nix-darwin activation).
const BaseTemplateName = "devcell-tart-base"

// TemplateVMName returns the tart VM name for a built template image.
func TemplateVMName(stack string, modules []string) string {
	return "devcell-tart-" + cell.StackTag(stack, modules)
}

// InstanceVMName returns the tart VM name for a per-cell running instance.
func InstanceVMName(cellName string) string {
	return cellName + "-tart"
}
