// +build flac

package gosound

import (
	"bufio"
	"context"
	"errors"
	"os"

	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/frame"
	"github.com/mewkiz/flac/meta"

	"github.com/gotracker/gomixing/mixing"
)

type fileDeviceFlac struct {
	fileDevice
	mix              mixing.Mixer
	samplesPerSecond int

	f *os.File
	w *bufio.Writer
}

func newFileFlacDevice(settings Settings) (Device, error) {
	fd := fileDeviceFlac{
		fileDevice: fileDevice{
			device: device{
				onRowOutput: settings.OnRowOutput,
			},
		},
		mix: mixing.Mixer{
			Channels:      settings.Channels,
			BitsPerSample: settings.BitsPerSample,
		},
		samplesPerSecond: settings.SamplesPerSecond,
	}
	f, err := os.OpenFile(settings.Filepath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}

	if f == nil {
		return nil, errors.New("unexpected file error")
	}

	fd.f = f

	return &fd, nil
}

// Play starts the wave output device playing
func (d *fileDeviceFlac) Play(in <-chan *PremixData) error {
	return d.PlayWithCtx(context.Background(), in)
}

// PlayWithCtx starts the wave output device playing
func (d *fileDeviceFlac) PlayWithCtx(ctx context.Context, in <-chan *PremixData) error {
	w := bufio.NewWriter(d.f)
	d.w = w
	// Encode FLAC stream.
	si := &meta.StreamInfo{
		BlockSizeMin:  16,
		BlockSizeMax:  65535,
		SampleRate:    uint32(d.samplesPerSecond),
		NChannels:     uint8(d.mix.Channels),
		BitsPerSample: uint8(d.mix.BitsPerSample),
	}
	enc, err := flac.NewEncoder(w, si)
	if err != nil {
		return err
	}
	defer enc.Close()

	panmixer := mixing.GetPanMixer(d.mix.Channels)
	if panmixer == nil {
		return errors.New("invalid pan mixer - check channel count")
	}

	var channels frame.Channels
	switch d.mix.Channels {
	case 1:
		channels = frame.ChannelsMono
	case 2:
		channels = frame.ChannelsLR
	case 4:
		channels = frame.ChannelsLRLsRs
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
			mixedData := d.mix.FlattenToInts(panmixer, row.SamplesLen, row.Data)
			subframes := make([]*frame.Subframe, d.mix.Channels)
			for i := range subframes {
				subframe := &frame.Subframe{
					SubHeader: frame.SubHeader{
						Pred: frame.PredVerbatim,
					},
					Samples:  mixedData[i],
					NSamples: row.SamplesLen,
				}
				subframes[i] = subframe
			}
			for _, subframe := range subframes {
				sample := subframe.Samples[0]
				constant := true
				for _, s := range subframe.Samples[1:] {
					if sample != s {
						constant = false
					}
				}
				if constant {
					subframe.SubHeader.Pred = frame.PredConstant
				}
			}

			fr := &frame.Frame{
				Header: frame.Header{
					HasFixedBlockSize: false,
					BlockSize:         uint16(row.SamplesLen),
					SampleRate:        uint32(d.samplesPerSecond),
					Channels:          channels,
					BitsPerSample:     uint8(d.mix.BitsPerSample),
				},
				Subframes: subframes,
			}
			if err := enc.WriteFrame(fr); err != nil {
				return err
			}
			if d.onRowOutput != nil {
				d.onRowOutput(KindFile, row)
			}
		}
	}
}

// Close closes the wave output device
func (d *fileDeviceFlac) Close() {
	d.w.Flush()
	d.w = nil
	d.f.Close()
}

func init() {
	fileDeviceMap[".flac"] = newFileFlacDevice
}
