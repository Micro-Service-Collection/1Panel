package files

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"
)

type FileInfo struct {
	Fs         afero.Fs    `json:"-"`
	Path       string      `json:"path"`
	Name       string      `json:"name"`
	User       string      `json:"user"`
	Group      string      `json:"group"`
	Extension  string      `json:"extension"`
	Content    string      `json:"content"`
	Size       int64       `json:"size"`
	IsDir      bool        `json:"isDir"`
	IsSymlink  bool        `json:"isSymlink"`
	IsHidden   bool        `json:"isHidden"`
	LinkPath   string      `json:"linkPath"`
	Type       string      `json:"type"`
	Mode       string      `json:"mode"`
	MimeType   string      `json:"mimeType"`
	UpdateTime time.Time   `json:"updateTime"`
	ModTime    time.Time   `json:"modTime"`
	FileMode   os.FileMode `json:"-"`
	Items      []*FileInfo `json:"items"`
}

type FileOption struct {
	Path       string `json:"path"`
	Search     string `json:"search"`
	Expand     bool   `json:"expand"`
	Dir        bool   `json:"dir"`
	ShowHidden bool   `json:"showHidden"`
}

func NewFileInfo(op FileOption) (*FileInfo, error) {
	var appFs = afero.NewOsFs()

	info, err := appFs.Stat(op.Path)
	if err != nil {
		return nil, err
	}

	file := &FileInfo{
		Fs:        appFs,
		Path:      op.Path,
		Name:      info.Name(),
		IsDir:     info.IsDir(),
		FileMode:  info.Mode(),
		ModTime:   info.ModTime(),
		Size:      info.Size(),
		IsSymlink: IsSymlink(info.Mode()),
		Extension: filepath.Ext(info.Name()),
		IsHidden:  IsHidden(op.Path),
		Mode:      fmt.Sprintf("%04o", info.Mode().Perm()),
		User:      GetUsername(info.Sys().(*syscall.Stat_t).Uid),
		Group:     GetGroup(info.Sys().(*syscall.Stat_t).Gid),
		MimeType:  GetMimeType(op.Path),
	}
	if file.IsSymlink {
		file.LinkPath = GetSymlink(op.Path)
	}
	if op.Expand {
		if file.IsDir {
			if err := file.listChildren(op.Dir, op.ShowHidden); err != nil {
				return nil, err
			}
			return file, nil
		} else {
			if err := file.getContent(); err != nil {
				return nil, err
			}
		}
	}
	return file, nil
}

func (f *FileInfo) listChildren(dir, showHidden bool) error {
	afs := &afero.Afero{Fs: f.Fs}
	files, err := afs.ReadDir(f.Path)
	if err != nil {
		return err
	}
	var items []*FileInfo
	for _, df := range files {
		if dir && !df.IsDir() {
			continue
		}

		name := df.Name()
		fPath := path.Join(f.Path, df.Name())

		if !showHidden && IsHidden(name) {
			continue
		}

		isSymlink, isInvalidLink := false, false
		if IsSymlink(df.Mode()) {
			isSymlink = true
			info, err := f.Fs.Stat(fPath)
			if err == nil {
				df = info
			} else {
				isInvalidLink = true
			}
		}

		file := &FileInfo{
			Fs:        f.Fs,
			Name:      name,
			Size:      df.Size(),
			ModTime:   df.ModTime(),
			FileMode:  df.Mode(),
			IsDir:     df.IsDir(),
			IsSymlink: isSymlink,
			IsHidden:  IsHidden(fPath),
			Extension: filepath.Ext(name),
			Path:      fPath,
			Mode:      fmt.Sprintf("%04o", df.Mode().Perm()),
			User:      GetUsername(df.Sys().(*syscall.Stat_t).Uid),
			Group:     GetGroup(df.Sys().(*syscall.Stat_t).Gid),
			MimeType:  GetMimeType(fPath),
		}
		if isSymlink {
			file.LinkPath = GetSymlink(fPath)
		}

		if isInvalidLink {
			file.Type = "invalid_link"
		}
		items = append(items, file)
	}
	f.Items = items
	return nil
}

func (f *FileInfo) getContent() error {
	if f.Size <= 10*1024*1024 {
		afs := &afero.Afero{Fs: f.Fs}
		cByte, err := afs.ReadFile(f.Path)
		if err != nil {
			return nil
		}
		f.Content = string(cByte)
		return nil
	} else {
		return errors.New("file is too large!")
	}
}