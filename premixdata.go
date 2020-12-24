package gosound

import "github.com/gotracker/gomixing/mixing"

// PremixData is a structure containing the audio pre-mix data for a specific row or buffer
type PremixData struct {
	SamplesLen int
	Data       []mixing.ChannelData
	Userdata   interface{}
}
