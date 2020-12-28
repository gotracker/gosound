// +build windows

package gosound

import (
	"context"
	"errors"
	"time"

	"github.com/gotracker/gomixing/mixing"

	"github.com/gotracker/gosound/internal/win32/winmm"
)

const winmmName = "winmm"

type winmmDevice struct {
	device
	mix     mixing.Mixer
	waveout *winmm.WaveOut
}

func newWinMMDevice(settings Settings) (Device, error) {
	d := winmmDevice{
		device: device{
			onRowOutput: settings.OnRowOutput,
		},
		mix: mixing.Mixer{
			Channels:      settings.Channels,
			BitsPerSample: settings.BitsPerSample,
		},
	}
	var err error
	d.waveout, err = winmm.New(settings.Channels, settings.SamplesPerSecond, settings.BitsPerSample)
	if err != nil {
		return nil, err
	}
	if d.waveout == nil {
		return nil, errors.New("could not create winmm device")
	}
	return &d, nil
}

// Name returns the device name
func (d *winmmDevice) Name() string {
	return winmmName
}

// Play starts the wave output device playing
func (d *winmmDevice) Play(in <-chan *PremixData) error {
	return d.PlayWithCtx(context.Background(), in)
}

// PlayWithCtx starts the wave output device playing
func (d *winmmDevice) PlayWithCtx(ctx context.Context, in <-chan *PremixData) error {
	type RowWave struct {
		Wave *winmm.WaveOutData
		Row  *PremixData
	}

	panmixer := mixing.GetPanMixer(d.mix.Channels)
	if panmixer == nil {
		return errors.New("invalid pan mixer - check channel count")
	}

	myCtx, cancel := context.WithCancel(ctx)

	out := make(chan RowWave, 3)

	go func() {
		defer cancel()
		defer close(out)
		for {
			select {
			case <-myCtx.Done():
				return
			case row, ok := <-in:
				if !ok {
					return
				}
				mixedData := d.mix.Flatten(panmixer, row.SamplesLen, row.Data, row.MixerVolume)
				rowWave := RowWave{
					Wave: d.waveout.Write(mixedData),
					Row:  row,
				}
				out <- rowWave
			}
		}
	}()

	for {
		select {
		case <-myCtx.Done():
			return myCtx.Err()
		case rowWave, ok := <-out:
			if !ok {
				// done!
				return nil
			}
			if d.onRowOutput != nil {
				d.onRowOutput(KindSoundCard, rowWave.Row)
			}
			for !d.waveout.IsHeaderFinished(rowWave.Wave) {
				time.Sleep(time.Microsecond * 1)
			}
		}
	}
}

// Close closes the wave output device
func (d *winmmDevice) Close() {
	if d.waveout != nil {
		d.waveout.Close()
	}
}

func init() {
	Map[winmmName] = deviceDetails{
		create: newWinMMDevice,
		kind:   KindSoundCard,
	}
}
