package executer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Current LinuxExecuter = LinuxExecuter{}

type LinuxExecuter struct {
}

func CreateLinuxExecuter() LinuxExecuter {
	return LinuxExecuter{}
}

func (le LinuxExecuter) EnsureFile(path string, modTime time.Time, hash string, downloadFunc func() (io.ReadCloser, error)) error {
	needDownload := false
	info, err := le.Stat(path)
	if os.IsNotExist(err) {
		needDownload = true
	} else if err != nil {
		return err
	} else {
		if info.IsDir() {
			err := le.RemovePath(path)
			if err != nil {
				return err
			}
			needDownload = true
		}

		if !info.ModTime().Equal(modTime) {
			isHash, err := le.CalculateHash(path)
			if err != nil {
				return err
			}
			if isHash != hash {
				needDownload = true
			} else {
				return le.Chtimes(path, modTime)
			}
		}
	}

	if needDownload {
		log.Info().Msgf("Downloading file to %s", path)
		r, err := downloadFunc()
		if err != nil {
			return fmt.Errorf("download func: %w", err)
		}
		defer func() { _ = r.Close() }()
		return le.downloadToPath(context.Background(), path, modTime, r)
	}

	return nil
}

func (le LinuxExecuter) EnsureDir(path string) error {
	info, err := le.Stat(path)
	if os.IsNotExist(err) {
		zerolog.DefaultContextLogger.Info().Msgf("Creating directory %s", path)
		return le.MkdirAll(path)
	} else if err != nil {
		return err
	}

	if !info.IsDir() {
		zerolog.DefaultContextLogger.Info().Msgf("Removing file %s to create directory", path)
		err := le.RemovePath(path)
		if err != nil {
			return err
		}

		zerolog.DefaultContextLogger.Info().Msgf("Creating directory %s", path)
		return le.MkdirAll(path)
	}

	return nil
}

func (le LinuxExecuter) downloadToPath(ctx context.Context, downloadPath string, modTime time.Time, r io.Reader) error {
	downloadFile := path.Base(downloadPath)
	downloadDir := path.Dir(downloadPath)
	le.MkdirAll(downloadDir)
	// needs to be removed or renamed
	f, err := os.CreateTemp(downloadDir, downloadFile+"-download-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	fName := f.Name()

	_, err = f.ReadFrom(r)
	errClose := f.Close()
	if err != nil {
		_ = le.RemovePath(fName)
		maybeErr := context.Cause(ctx)
		if maybeErr != nil {
			return fmt.Errorf("download file: %w", maybeErr)
		}
		return fmt.Errorf("download file: %w", err)
	}

	if errClose != nil {
		_ = le.RemovePath(fName)
		return fmt.Errorf("close file: %w", errClose)
	}
	// should be okay because the temp file is in the same directory
	err = le.Rename(f.Name(), downloadPath)
	if err != nil {
		_ = le.RemovePath(fName)
		return fmt.Errorf("rename file: %w", err)
	}
	return le.Chtimes(downloadPath, modTime)
}

func (le LinuxExecuter) CalculateHash(path string) (string, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	io.Copy(hasher, f)
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (le LinuxExecuter) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (le LinuxExecuter) Chtimes(path string, mtime time.Time) error {
	return os.Chtimes(path, mtime, mtime)
}

func (le LinuxExecuter) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (le LinuxExecuter) MkdirAll(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

func (le LinuxExecuter) RemovePath(path string) error {
	return os.RemoveAll(path)
}
