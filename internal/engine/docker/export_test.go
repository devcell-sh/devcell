package docker

// Test seams that let the external docker_test package drive unexported
// pure helpers without docker.
var (
	ImageMetadataFromInspectExport = imageMetadataFromInspect
	FormatImageVersionUserExport   = formatImageVersionUser
)
