// Package secretbus declares the Secret Service's D-Bus interfaces, events
// and signals once, for the service that emits them and the clients that
// follow them.
package secretbus

import (
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// BusName is the Secret Service's well-known name, ServicePath its
// service object and NoObject the path that names no prompt, collection
// or item.
const (
	BusName     = "org.freedesktop.secrets"
	ServicePath = dbus.ObjectPath("/org/freedesktop/secrets")
	NoObject    = dbus.ObjectPath("/")
)

const (
	ServiceInterface    = "org.freedesktop.Secret.Service"
	CollectionInterface = "org.freedesktop.Secret.Collection"
	ItemInterface       = "org.freedesktop.Secret.Item"
	SessionInterface    = "org.freedesktop.Secret.Session"
	PromptInterface     = "org.freedesktop.Secret.Prompt"
)

// The qualified properties CreateCollection and CreateItem take.
const (
	CollectionLabel = CollectionInterface + ".Label"
	ItemLabel       = ItemInterface + ".Label"
	ItemAttributes  = ItemInterface + ".Attributes"
)

// The errors the specification defines.
const (
	ErrIsLocked     = "org.freedesktop.Secret.Error.IsLocked"
	ErrNoSession    = "org.freedesktop.Secret.Error.NoSession"
	ErrNoSuchObject = "org.freedesktop.Secret.Error.NoSuchObject"
)

type CollectionEvent struct {
	Collection dbus.ObjectPath `dbus:"collection"`
}

type ItemEvent struct {
	Item dbus.ObjectPath `dbus:"item"`
}

type Completion struct {
	Dismissed bool         `dbus:"dismissed"`
	Result    dbus.Variant `dbus:"result"`
}

var (
	CollectionCreated = dbusx.NewSignal[CollectionEvent](ServiceInterface, "CollectionCreated")
	CollectionDeleted = dbusx.NewSignal[CollectionEvent](ServiceInterface, "CollectionDeleted")
	CollectionChanged = dbusx.NewSignal[CollectionEvent](ServiceInterface, "CollectionChanged")

	ItemCreated = dbusx.NewSignal[ItemEvent](CollectionInterface, "ItemCreated")
	ItemDeleted = dbusx.NewSignal[ItemEvent](CollectionInterface, "ItemDeleted")
	ItemChanged = dbusx.NewSignal[ItemEvent](CollectionInterface, "ItemChanged")

	PromptCompleted = dbusx.NewSignal[Completion](PromptInterface, "Completed")
)
