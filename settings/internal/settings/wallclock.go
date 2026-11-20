package settings

import "time"

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

func (wallClock) AfterFunc(d time.Duration, fn func()) func() bool { return time.AfterFunc(d, fn).Stop }
