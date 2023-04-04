package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

func NewLocal(basePath string) (Base, error) {
	var err error
	basePath, err = filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(basePath)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("%s: is not a directory", basePath)
	}

	l := LocalStorage{basePath: basePath, log: zap.L().With(zap.String("facility", "local-storage"))}
	l.log.Info("Initialized Local Storage adapter", zap.String("base_path", basePath))
	return l, nil
}

type LocalStorage struct {
	log      *zap.Logger
	basePath string
}

func (l LocalStorage) RepositoryPath(repository string) string {
	return repositoryPath(l.basePath, repository)
}

func (l LocalStorage) CommitPath(repository, commitish string) string {
	return commitPath(l.basePath, repository, commitish)
}

func (l LocalStorage) DownloadCommit(repository, commitish, into string) error {
	itemPath := l.CommitPath(repository, commitish) + ".tar.zst"
	shaPath := itemPath + ".shasum"
	if err := validateSha(shaPath, itemPath); err != nil {
		return err
	}
	return fsCopy(itemPath, into)
}

func fsCopy(from, to string) error {
	original, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = original.Close() }()

	// Create new file
	dst, err := os.Create(to)
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()

	_, err = io.Copy(dst, original)
	if err != nil {
		return err
	}

	if err = dst.Sync(); err != nil {
		_ = os.Remove(to)
		return err
	}

	return nil
}
