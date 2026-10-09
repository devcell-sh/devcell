package docker

// ExecSpec holds the parameters for building a docker exec argv.
type ExecSpec struct {
	ContainerName string
	User          string
	Binary        string
	Args          []string
	TTY           bool
	Env           map[string]string
}

// BuildExecArgv constructs a docker exec argv for attaching to a running container.
func BuildExecArgv(spec ExecSpec) []string {
	argv := []string{"docker", "exec"}
	if spec.TTY {
		argv = append(argv, "-it")
	}
	if spec.User != "" {
		argv = append(argv, "-u", spec.User)
	}
	for k, v := range spec.Env {
		argv = append(argv, "-e", k+"="+v)
	}
	argv = append(argv, spec.ContainerName, spec.Binary)
	argv = append(argv, spec.Args...)
	return argv
}
