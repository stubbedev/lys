package dm

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"

	"github.com/stubbedev/lys/lib/drm"
	"golang.org/x/sys/unix"
)

const cardGlob = "/dev/dri/card[0-9]*"

type openCard interface {
	FD() int
	DropMaster() error
	SetClientCap(capability, value uint64) error
	Planes() ([]uint32, error)
	Plane(id uint32) (drm.Plane, error)
	Properties(objectID, objectType uint32) (map[string]drm.Property, error)
	PropertyInfo(id uint32) (drm.PropertyInfo, error)
	Close() error
}

type heldCards struct {
	glob   string
	open   func(path string) (openCard, error)
	fromFD func(fd int, path string) openCard
	cards  map[string]openCard
}

func newHeldCards() *heldCards {
	return &heldCards{
		glob:   cardGlob,
		open:   func(path string) (openCard, error) { return drm.Open(path) },
		fromFD: func(fd int, path string) openCard { return drm.FromFD(fd, path) },
		cards:  map[string]openCard{},
	}
}

func (h *heldCards) holdAll() error {
	paths, err := filepath.Glob(h.glob)
	if err != nil {
		return fmt.Errorf("list the DRM cards %s: %w", h.glob, err)
	}
	var errs []error
	for _, path := range paths {
		if _, held := h.cards[path]; held {
			continue
		}
		card, err := h.open(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("hold %s open across compositors: %w", path, err))
			continue
		}
		if err := card.DropMaster(); err != nil && !errors.Is(err, unix.EINVAL) {
			errs = append(errs, fmt.Errorf("give up the DRM master of %s: %w", path, err), card.Close())
			continue
		}
		if err := listPrimaryPlanes(card, path); err != nil {
			errs = append(errs, err, card.Close())
			continue
		}
		h.cards[path] = card
	}
	return errors.Join(errs...)
}

func (h *heldCards) held() []handedCard {
	held := make([]handedCard, 0, len(h.cards))
	for _, path := range slices.Sorted(maps.Keys(h.cards)) {
		held = append(held, handedCard{Path: path, FD: h.cards[path].FD()})
	}
	return held
}

func (h *heldCards) adopt(cards []handedCard, track func(io.Closer)) error {
	for _, card := range cards {
		dup, err := unix.FcntlInt(uintptr(card.FD), unix.F_DUPFD_CLOEXEC, firstHandoverFD)
		if err != nil {
			return fmt.Errorf("%s: duplicate descriptor %d: %w", card.Path, card.FD, err)
		}
		c := h.fromFD(dup, card.Path)
		track(c)
		if err := listPrimaryPlanes(c, card.Path); err != nil {
			return err
		}
		h.cards[card.Path] = c
	}
	return nil
}

func listPrimaryPlanes(card openCard, path string) error {
	if err := card.SetClientCap(drm.ClientCapUniversal, 1); err != nil {
		return fmt.Errorf("list the primary planes of %s: %w", path, err)
	}
	return nil
}

type scanouts map[string]map[uint32]uint32

func (h *heldCards) scanouts() func() (scanouts, error) {
	cards := maps.Clone(h.cards)
	primaries := make(map[string][]uint32, len(cards))
	for path, card := range cards {
		ids, err := primaryPlanes(card)
		if err != nil {
			return func() (scanouts, error) { return nil, fmt.Errorf("list the primary planes of %s: %w", path, err) }
		}
		primaries[path] = ids
	}
	return func() (scanouts, error) {
		shown := make(scanouts, len(cards))
		for path, card := range cards {
			fbs := make(map[uint32]uint32, len(primaries[path]))
			for _, id := range primaries[path] {
				p, err := card.Plane(id)
				if err != nil {
					return nil, fmt.Errorf("read plane %d of %s: %w", id, path, err)
				}
				if p.FB != 0 {
					fbs[id] = p.FB
				}
			}
			shown[path] = fbs
		}
		return shown, nil
	}
}

func primaryPlanes(card openCard) ([]uint32, error) {
	planes, err := card.Planes()
	if err != nil {
		return nil, err
	}
	primary := make([]uint32, 0, len(planes))
	for _, id := range planes {
		props, err := card.Properties(id, drm.ObjectPlane)
		if err != nil {
			return nil, fmt.Errorf("read the properties of plane %d: %w", id, err)
		}
		kind, ok := props["type"]
		if !ok {
			return nil, fmt.Errorf("plane %d has no type property", id)
		}
		info, err := card.PropertyInfo(kind.ID)
		if err != nil {
			return nil, fmt.Errorf("read the type property of plane %d: %w", id, err)
		}
		if want, ok := info.Enums["Primary"]; ok && kind.Value == want {
			primary = append(primary, id)
		}
	}
	return primary, nil
}

func (s scanouts) replacedIn(now scanouts) bool {
	for path, fbs := range s {
		for plane, fb := range fbs {
			if now[path][plane] != fb {
				return true
			}
		}
	}
	return false
}

func (h *heldCards) Close() error {
	var errs []error
	for _, path := range slices.Sorted(maps.Keys(h.cards)) {
		errs = append(errs, h.cards[path].Close())
		delete(h.cards, path)
	}
	return errors.Join(errs...)
}
