package storage

import (
	"fmt"
	"go.uber.org/zap"
	"os"
	"path/filepath"
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
	itemPath := l.CommitPath(repository, commitish) + ".tar.br"
	shaPath := itemPath + ".shasum"
	if err := validateSha(shaPath, itemPath); err != nil {
		return err
	}
	return os.Link(itemPath, into)
}
