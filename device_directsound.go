// +build windows,directsound

package gosound

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/gotracker/gomixing/mixing"

	"github.com/gotracker/gosound/internal/win32"
	"github.com/gotracker/gosound/internal/win32/directsound"
)

const dsoundName = "directsound"

type dsoundDevice struct {
	device

	ds           *directsound.DirectSound
	lpdsbPrimary *directsound.Buffer
	wfx          *win32.WAVEFORMATEX

	mix mixing.Mixer
}

func newDSoundDevice(settings Settings) (Device, error) {
	d := dsoundDevice{
		device: device{
			onRowOutput: settings.OnRowOutput,
		},
		mix: mixing.Mixer{
			Channels:      settings.Channels,
			BitsPerSample: settings.BitsPerSample,
		},
	}
	preferredDeviceName := ""

	ds, err := directsound.NewDSound(preferredDeviceName)
	if err != nil {
		return nil, err
	}
	d.ds = ds
	if d.ds == nil {
		return nil, errors.New("could not create directsound device")
	}

	lpdsbPrimary, wfx, err := ds.CreateSoundBufferPrimary(settings.Channels, settings.SamplesPerSecond, settings.BitsPerSample)
	if err != nil {
		ds.Close()
		return nil, err
	}
	d.lpdsbPrimary = lpdsbPrimary
	d.wfx = wfx

	return &d, nil
}

// Name returns the device name
func (d *dsoundDevice) Name() string {
	return dsoundName
}

// Play starts the wave output device playing
func (d *dsoundDevice) Play(in <-chan *PremixData) error {
	return d.PlayWithCtx(context.Background(), in)
}

// PlayWithCtx starts the wave output device playing
func (d *dsoundDevice) PlayWithCtx(ctx context.Context, in <-chan *PremixData) error {
	type RowWave struct {
		PlayOffset uint32
		Row        *PremixData
	}

	event, err := win32.CreateEvent()
	if err != nil {
		return err
	}
	defer win32.CloseHandle(event)

	panmixer := mixing.GetPanMixer(d.mix.Channels)
	if panmixer == nil {
		return errors.New("invalid pan mixer - check channel count")
	}

	playbackSize := int(d.wfx.NAvgBytesPerSec * 2)
	lpdsb, err := d.ds.CreateSoundBufferSecondary(d.wfx, playbackSize)
	if err != nil {
		return err
	}
	defer lpdsb.Release()

	notify, err := lpdsb.GetNotify()
	if err != nil {
		return err
	}
	defer notify.Release()

	pn := []directsound.PositionNotify{
		{
			Offset:      uint32(playbackSize - int(d.wfx.NBlockAlign)),
			EventNotify: event,
		},
	}

	if err := notify.SetNotificationPositions(pn); err != nil {
		return err
	}

	// play (looping)
	if err := lpdsb.Play(true); err != nil {
		return err
	}

	done := make(chan struct{}, 1)
	defer close(done)

	myCtx, cancel := context.WithCancel(ctx)

	out := make(chan RowWave, 3)
	go func() {
		defer close(out)
		defer cancel()
		writePos := 0
		for {
			select {
			case <-myCtx.Done():
				return
			case row, ok := <-in:
				if !ok {
					return
				}
				var rowWave RowWave
				//_, writePos, err := lpdsb.GetCurrentPosition()
				numBytes := row.SamplesLen * int(d.wfx.NBlockAlign)
				segments, err := lpdsb.Lock(writePos%playbackSize, numBytes)
				if err != nil {
					continue
				}
				d.mix.FlattenTo(segments, panmixer, row.SamplesLen, row.Data)
				if err := lpdsb.Unlock(segments); err != nil {
					continue
				}
				rowWave.Row = row
				writePos += numBytes
				rowWave.PlayOffset = uint32(writePos)
				out <- rowWave
			}
		}
	}()
	playBase := uint32(0)
	go func() {
		eventCh, closeFunc := win32.EventToChannel(event)
		defer closeFunc()
		for {
			select {
			case <-eventCh:
				atomic.AddUint32(&playBase, uint32(playbackSize))
			case <-done:
				return
			}
		}
	}()
	for rowWave := range out {
		for {
			playPos, _, _ := lpdsb.GetCurrentPosition()
			base := atomic.LoadUint32(&playBase)
			if playPos+base >= rowWave.PlayOffset {
				if d.onRowOutput != nil {
					d.onRowOutput(KindSoundCard, rowWave.Row)
				}
				break
			}
			time.Sleep(time.Millisecond * 1)
		}
	}
	done <- struct{}{}
	return nil
}

// Close closes the wave output device
func (d *dsoundDevice) Close() {
	if d.lpdsbPrimary != nil {
		d.lpdsbPrimary.Release()
	}
	if d.ds != nil {
		d.ds.Close()
	}
}

func init() {
	Map[dsoundName] = deviceDetails{
		create: newDSoundDevice,
		kind:   KindSoundCard,
	}
}
