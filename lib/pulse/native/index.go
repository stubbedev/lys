package native

// SinkIndex numbers a sink. Each kind of object the server numbers has
// its own index type, so one kind's index is never passed where
// another's is meant.
type SinkIndex uint32

// SourceIndex numbers a source.
type SourceIndex uint32

// SinkInputIndex numbers a playback stream.
type SinkInputIndex uint32

// SourceOutputIndex numbers a recording stream.
type SourceOutputIndex uint32

// ModuleIndex numbers a loaded module.
type ModuleIndex uint32

// ClientIndex numbers a connected client.
type ClientIndex uint32

// CardIndex numbers a card.
type CardIndex uint32

type objectIndex interface {
	~uint32
}
