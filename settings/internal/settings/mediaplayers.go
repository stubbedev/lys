package settings

import (
	"context"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/service/mpris"
	"github.com/stubbedev/lys/lib/dbusx"
)

const (
	listMediaPlayers = "media-player"
	mediaPlayersWait = 3 * time.Second
)

func playerPattern(busName string) (pattern, name string, ok bool) {
	rest, ok := strings.CutPrefix(busName, mpris.BusNamePrefix)
	if !ok || rest == "" {
		return "", "", false
	}
	name, _, _ = strings.Cut(rest, ".")
	return "*" + name + "*", name, true
}

func mediaPlayers() []choice {
	ctx, cancel := context.WithTimeout(context.Background(), mediaPlayersWait)
	defer cancel()
	conn, err := dbusx.DialSession(ctx, mediaPlayersWait)
	if err != nil {
		log.Printf("settings: the running media players: %v", err)
		return nil
	}
	defer func() { _ = conn.Close() }()
	var names []string
	if err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		log.Printf("settings: the running media players: %v", err)
		return nil
	}
	return playerChoices(names)
}

func playerChoices(busNames []string) []choice {
	var out []choice
	for _, bus := range busNames {
		if pattern, name, ok := playerPattern(bus); ok && !slices.ContainsFunc(out, func(c choice) bool { return c.value == pattern }) {
			out = append(out, choice{value: pattern, label: name, group: pattern})
		}
	}
	return sortedChoices(out)
}

func runningPlayers(*kit) lister { return systemLists[listMediaPlayers] }

var mediaPlayersRow = choiceListRow(runningPlayers, plainChoices)
