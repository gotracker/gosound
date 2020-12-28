// +build linux pulseaudio

package gosound

import (
	"context"

	"github.com/gotracker/gomixing/mixing"
	"github.com/pkg/errors"

	"github.com/gotracker/gosound/internal/pulseaudio"
)

const pulseaudioName = "pulseaudio"

type pulseaudioDevice struct {
	device
	mix mixing.Mixer
	pa  *pulseaudio.Client
}

func newPulseAudioDevice(settings Settings) (Device, error) {
	d := pulseaudioDevice{
		device: device{
			onRowOutput: settings.OnRowOutput,
		},
		mix: mixing.Mixer{
			Channels:      settings.Channels,
			BitsPerSample: settings.BitsPerSample,
		},
	}

	play, err := pulseaudio.New("Music", settings.SamplesPerSecond, settings.Channels, settings.BitsPerSample)
	if err != nil {
		return nil, err
	}

	d.pa = play
	return &d, nil
}

// Name returns the device name
func (d *pulseaudioDevice) Name() string {
	return pulseaudioName
}

// Play starts the wave output device playing
func (d *pulseaudioDevice) Play(in <-chan *PremixData) error {
	return d.PlayWithCtx(context.Background(), in)
}

// PlayWithCtx starts the wave output device playing
func (d *pulseaudioDevice) PlayWithCtx(ctx context.Context, in <-chan *PremixData) error {
	panmixer := mixing.GetPanMixer(d.mix.Channels)
	if panmixer == nil {
		return errors.New("invalid pan mixer - check channel count")
	}

	myCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		select {
		case <-myCtx.Done():
			return myCtx.Err()
		case row, ok := <-in:
			if !ok {
				return nil
			}
			mixedData := d.mix.Flatten(panmixer, row.SamplesLen, row.Data, row.MixerVolume)
			d.pa.Output(mixedData)
			if d.onRowOutput != nil {
				d.onRowOutput(KindSoundCard, row)
			}
		}
	}
}

// Close closes the wave output device
func (d *pulseaudioDevice) Close() {
	if d.pa != nil {
		d.pa.Close()
	}
}

func init() {
	Map[pulseaudioName] = deviceDetails{
		create: newPulseAudioDevice,
		kind:   KindSoundCard,
	}
}
