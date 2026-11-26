// Package opustoc reads how long an Opus packet plays from its
// table-of-contents byte and frame count code (RFC 6716 sections 3.1
// and 3.2), what the decoder and the demuxers that time packets share.
package opustoc

const (
	frame20ms = 960
	frame10ms = frame20ms / 2
	frame2p5  = frame10ms / 4
	// maxFrames is the frame count code's largest count.
	maxFrames = 0x3f
)

// FrameSize is each frame's duration in 48 kHz samples for a packet
// whose TOC byte is toc.
func FrameSize(toc byte) int {
	config := int(toc >> 3)
	switch {
	case config < 12:
		return [4]int{frame10ms, frame20ms, 2 * frame20ms, 3 * frame20ms}[config&3]
	case config < 16:
		return [2]int{frame10ms, frame20ms}[config&1]
	}
	return frame2p5 << (config & 3)
}

// Samples is the packet's duration in 48 kHz samples; ok is false for
// a packet without a TOC byte or, coded with an arbitrary frame count,
// without a valid count.
func Samples(packet []byte) (samples int, ok bool) {
	if len(packet) == 0 {
		return 0, false
	}
	frames := 1
	switch packet[0] & 3 {
	case 1, 2:
		frames = 2
	case 3:
		if len(packet) < 2 || packet[1]&maxFrames == 0 {
			return 0, false
		}
		frames = int(packet[1] & maxFrames)
	}
	return frames * FrameSize(packet[0]), true
}
