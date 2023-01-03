package execute

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func mergeEnvs(newEnvs map[string]string) []string {
	if newEnvs == nil {
		newEnvs = map[string]string{}
	}
	envs := make([]string, 0, len(newEnvs))
	for k, v := range newEnvs {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	for _, v := range os.Environ() {
		exists := false
		prefixIndex := strings.Index(v, "=")
		prefix := v[0:prefixIndex]
		for _, cv := range envs {
			if strings.HasPrefix(cv, prefix) {
				exists = true
				break
			}
		}
		if !exists {
			envs = append(envs, v)
		}
	}

	return envs
}

type Opts struct {
	Cwd  string
	Envs map[string]string
}

func Exec2(args []string, opts *Opts) (stdout, stderr *bytes.Buffer, err error) {
	if len(args) == 0 {
		panic("exec2 called without arguments")
	}

	if opts == nil {
		opts = &Opts{}
	}

	cmdName := args[0]
	args = args[1:]

	var binPath string
	binPath, err = exec.LookPath(cmdName)
	if err != nil {
		return nil, nil, fmt.Errorf("ExecCwd: Cannot locate %s: %s", cmdName, err)
	}
	execArgs := append([]string{binPath}, args...)
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Cmd{
		Path:   binPath,
		Args:   execArgs,
		Dir:    opts.Cwd,
		Env:    mergeEnvs(opts.Envs),
		Stdin:  os.Stdin,
		Stdout: &outBuf,
		Stderr: &errBuf,
	}

	if err = cmd.Start(); err != nil {
		return
	}

	return &outBuf, &errBuf, cmd.Wait()
}

func Exec(args []string, opts *Opts) (*bytes.Buffer, error) {
	stdOut, _, err := Exec2(args, opts)
	return stdOut, err
}
