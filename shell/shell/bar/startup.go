package bar

type startupPart string

const (
	startupDropdowns startupPart = "dropdowns"
	startupTray      startupPart = "tray"
)

func startupBar(connector string) startupPart { return startupPart("bar " + connector) }

func startupParts(bars []string, tray bool) []startupPart {
	parts := make([]startupPart, 0, len(bars)+2)
	for _, connector := range bars {
		parts = append(parts, startupBar(connector))
	}
	if len(bars) > 0 {
		parts = append(parts, startupDropdowns)
	}
	if tray {
		parts = append(parts, startupTray)
	}
	return parts
}

type startup struct {
	pending  map[startupPart]bool
	finished map[startupPart]bool
	started  func()
}

func (s *startup) await(parts []startupPart, started func()) {
	s.pending = make(map[startupPart]bool, len(parts))
	for _, part := range parts {
		if !s.finished[part] {
			s.pending[part] = true
		}
	}
	s.started = started
	s.check()
}

func (s *startup) done(part startupPart) {
	if s.finished == nil {
		s.finished = make(map[startupPart]bool)
	}
	s.finished[part] = true
	delete(s.pending, part)
	s.check()
}

func (s *startup) check() {
	if s.started == nil || len(s.pending) > 0 {
		return
	}
	started := s.started
	s.started = nil
	started()
}
