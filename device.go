package gosound

import (
	"context"

	"github.com/pkg/errors"
)

var (
	// ErrDeviceNotSupported is returned when the requested device is not supported
	ErrDeviceNotSupported = errors.New("device not supported")
)

// DisplayFunc defines the callback for when a premix buffer is mixed/rendered and output on the device
type DisplayFunc func(deviceKind Kind, premix *PremixData)

// Device is an interface to output device operations
type Device interface {
	Name() string
	Play(in <-chan *PremixData) error
	PlayWithCtx(ctx context.Context, in <-chan *PremixData) error
	Close()
}

type createOutputDeviceFunc func(settings Settings) (Device, error)

type deviceDetails struct {
	Device
	create createOutputDeviceFunc
	kind   Kind
}

// GetKind returns the kind for the passed in device
func GetKind(d Device) Kind {
	if dev, ok := d.(*deviceDetails); ok {
		return dev.kind
	}
	return KindNone
}

var (
	// Map is the mapping of device name to device details
	Map = make(map[string]deviceDetails)
)

// CreateOutputDevice creates an output device based on the provided settings
func CreateOutputDevice(settings Settings) (Device, error) {
	if details, ok := Map[settings.Name]; ok && details.create != nil {
		dev, err := details.create(settings)
		if err != nil {
			return nil, err
		}
		dev.(*device).deviceDetails = details
		return dev, nil
	}

	return nil, errors.Wrap(ErrDeviceNotSupported, settings.Name)
}

type device struct {
	deviceDetails

	onRowOutput DisplayFunc
}

// Settings is the settings for configuring an output device
type Settings struct {
	Name             string
	Channels         int
	SamplesPerSecond int
	BitsPerSample    int
	Filepath         string
	OnRowOutput      DisplayFunc
}
