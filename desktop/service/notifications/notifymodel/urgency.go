// Package notifymodel is what a notification is beyond its server: the
// parts lys-shell's notification service records and anything drawing a
// notification reads, without a bus.
package notifymodel

// Urgency is the spec's urgency hint.
type Urgency uint8

// Urgency levels.
const (
	UrgencyLow Urgency = iota
	UrgencyNormal
	UrgencyCritical
)

// UrgencyFromByte reads the urgency hint: 0 low, 2 critical, anything else normal.
func UrgencyFromByte(v byte) Urgency {
	switch v {
	case 0:
		return UrgencyLow
	case 2:
		return UrgencyCritical
	}
	return UrgencyNormal
}
