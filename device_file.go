package gosound

import (
	"errors"
	"path"
	"strings"
)

const fileName = "file"

var (
	fileDeviceMap = make(map[string]createOutputDeviceFunc)
)

type fileDevice struct {
	device
}

func (d *fileDevice) GetKind() Kind {
	return KindFile
}

// Name returns the device name
func (d *fileDevice) Name() string {
	return fileName
}

func newFileDevice(settings Settings) (Device, error) {
	ext := strings.ToLower(path.Ext(settings.Filepath))
	if create, ok := fileDeviceMap[ext]; ok && create != nil {
		return create(settings)
	}

	return nil, errors.New("unsupported output format")
}

func init() {
	Map[fileName] = deviceDetails{
		create: newFileDevice,
		kind:   KindFile,
	}
}
