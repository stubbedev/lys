package widget

func installHook[F any](hooks *perLoop[F], fn F, none bool) (remove func()) {
	if none {
		hooks.unbindCurrent()
		return func() {}
	}
	id := hooks.bind(&fn)
	return func() { hooks.unbind(id, &fn) }
}
