package notifymodel

// Progress is the value hint: a share of 0-100 percent, or none.
type Progress struct {
	percent uint8
	set     bool
}

// NewProgress is percent, clamped to 0-100 as every server draws it.
func NewProgress(percent int64) Progress {
	return Progress{percent: uint8(min(max(percent, 0), 100)), set: true}
}

// Percent is the progress; false when the notification has none.
func (p Progress) Percent() (uint8, bool) { return p.percent, p.set }
