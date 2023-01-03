package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/cocov-ci/worker/execute"
	"os"
	"os/exec"
	"strings"
)

type Base interface {
	RepositoryPath(repository string) string
	CommitPath(repository, commitish string) string
	DownloadCommit(repository, commitish, into string) error
}

type SHASumError struct {
	Output string
	Status int
}

func (s SHASumError) Error() string {
	return fmt.Sprintf("shasum failed: %s (status %d)", s.Output, s.Status)
}

func shasum(data string) string {
	h := sha1.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func repositoryPath(base, repository string) string {
	return strings.Join([]string{base, shasum(repository)}, "/")
}

func commitPath(base, repository, commitish string) string {
	return strings.Join([]string{repositoryPath(base, repository), commitish}, "/")
}

func validateSha(sumPath, itemPath string) error {

	// Test for br file
	stat, err := os.Stat(itemPath)
	if err != nil {
		return err
	}
	if stat.IsDir() {
		return fmt.Errorf("%s: is a directory", itemPath)
	}

	// Test for shasum file
	stat, err = os.Stat(sumPath)
	if err != nil {
		return err
	}
	if stat.IsDir() {
		return fmt.Errorf("%s: is a directory", sumPath)
	}

	shasumData, err := os.ReadFile(sumPath)
	if err != nil {
		return err
	}
	shaComponents := strings.SplitN(string(shasumData), ":", 2)
	shaAlgo, shaDigest := strings.TrimPrefix(shaComponents[0], "sha"), strings.TrimSpace(shaComponents[1])

	rawSha, err := execute.Exec([]string{"shasum", "-a" + shaAlgo, itemPath}, nil)
	if execErr, ok := err.(*exec.ExitError); ok {
		return SHASumError{
			Output: string(execErr.Stderr),
			Status: execErr.ExitCode(),
		}
	} else if err != nil {
		return err
	}

	sha := strings.TrimSpace(strings.SplitN(rawSha.String(), " ", 2)[0])

	if sha != shaDigest {
		return SHASumError{
			Output: fmt.Sprintf("Local copy with digest %s does not match expected digest %s",
				sha, shaDigest),
			Status: 1,
		}
	}

	return nil
}

func inflateBrotli(source string, then func(string) error) error {
	rawTmpFile, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	if err = rawTmpFile.Close(); err != nil {
		return err
	}
	tmpPath := rawTmpFile.Name()
	if err = os.Remove(tmpPath); err != nil {
		return err
	}
	tmpPath = tmpPath + ".tar"

	if _, err := execute.Exec([]string{"brotli", "-d", source, "-o", tmpPath}, nil); err != nil {
		return err
	}

	defer func() { _ = os.Remove(tmpPath) }()

	return then(tmpPath)
}

func untar(source, dest string) error {
	if err := os.MkdirAll(dest, 0750); err != nil {
		return err
	}

	_, err := execute.Exec([]string{"tar", "--strip-components=1", "-xf", source}, &execute.Opts{Cwd: dest})
	return err
}
