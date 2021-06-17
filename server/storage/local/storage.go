package localstorage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	charm "github.com/charmbracelet/charm/proto"
	"github.com/charmbracelet/charm/server/storage"
)

// LocalFileStore is a FileStore implementation that stores files locally in a
// folder.
type LocalFileStore struct {
	Path string
}

// DirFile is a fs.File that represents a directory entry.
type DirFile struct {
	buffer   *bytes.Buffer
	fileInfo fs.FileInfo
}

// Stat returns a fs.FileInfo.
func (df *DirFile) Stat() (fs.FileInfo, error) {
	if df.fileInfo == nil {
		return nil, fmt.Errorf("missing file info")
	}
	return df.fileInfo, nil
}

// Read reads from the DirFile and satisfies fs.FS
func (df *DirFile) Read(buf []byte) (int, error) {
	return df.buffer.Read(buf)
}

// Close is a no-op but satisfies fs.FS
func (df *DirFile) Close() error {
	return nil
}

// NewLocalFileStore creates a FileStore locally in the provided path. Files
// will be encrypted client-side and stored as regular file system files and
// folders.
func NewLocalFileStore(path string) (*LocalFileStore, error) {
	err := storage.EnsureDir(path, 0700)
	if err != nil {
		return nil, err
	}
	return &LocalFileStore{path}, nil
}

// Get returns an fs.File for the given Charm ID and path.
func (lfs *LocalFileStore) Get(charmID string, path string) (fs.File, error) {
	fp := filepath.Join(lfs.Path, charmID, path)
	info, err := os.Stat(fp)
	if os.IsNotExist(err) {
		return nil, fs.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	f, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	// write a directory listing if path is a dir
	if info.IsDir() {
		rds, err := f.ReadDir(0)
		if err != nil {
			return nil, err
		}
		fis := make([]*charm.FileInfo, 0)
		for _, v := range rds {
			fi, err := v.Info()
			if err != nil {
				return nil, err
			}
			fin := &charm.FileInfo{
				Name:    v.Name(),
				IsDir:   v.IsDir(),
				Size:    fi.Size(),
				ModTime: fi.ModTime(),
				Mode:    fi.Mode(),
			}
			fis = append(fis, fin)
		}
		buf := bytes.NewBuffer(nil)
		enc := json.NewEncoder(buf)
		err = enc.Encode(fis)
		if err != nil {
			return nil, err
		}
		return &DirFile{buf, info}, nil
	}
	return f, nil
}

// Put reads from the provided io.Reader and stores the data with the Charm ID
// and path.
func (lfs *LocalFileStore) Put(charmID string, path string, r io.Reader, mode fs.FileMode) error {
	fp := filepath.Join(lfs.Path, charmID, path)
	if mode.IsDir() {
		return storage.EnsureDir(fp, mode)
	}
	err := storage.EnsureDir(filepath.Dir(fp), mode)
	if err != nil {
		return err
	}
	f, err := os.Create(fp)
	defer f.Close()
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}
	if mode != 0 {
		return f.Chmod(mode)
	}
	return nil
}

// Delete deletes the file at the given path for the provided Charm ID.
func (lfs *LocalFileStore) Delete(charmID string, path string) error {
	fp := filepath.Join(lfs.Path, charmID, path)
	err := os.RemoveAll(fp)
	if err != nil {
		return err
	}
	return nil
}
