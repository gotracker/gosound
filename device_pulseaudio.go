// +build linux pulseaudio

package gosound

import (
	"github.com/heucuva/gomixing/mixing"

	"github.com/heucuva/gosound/internal/pulseaudio"
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
func (d *pulseaudioDevice) Play(in <-chan *PremixData) {
	panmixer := mixing.GetPanMixer(d.mix.Channels)
	for row := range in {
		mixedData := d.mix.Flatten(panmixer, row.SamplesLen, row.Data)
		d.pa.Output(mixedData)
		if d.onRowOutput != nil {
			d.onRowOutput(KindSoundCard, row)
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
