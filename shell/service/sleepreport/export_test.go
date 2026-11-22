package sleepreport

import "context"

func Announcing(m *Monitor) func(context.Context, Notifier) { return announcing(m).run }
