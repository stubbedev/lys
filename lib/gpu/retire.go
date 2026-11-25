package gpu

import "math"

type destroyer interface{ destroy() }

type batchUser interface{ batchedIn() uint64 }

type grave struct {
	after uint64
	item  destroyer
}

func (d *Device) retire(item destroyer) {
	after := d.submitted
	if d.framing || d.batchUses(item) {
		after++
	}
	if after <= d.completed {
		item.destroy()
		return
	}
	d.graves = append(d.graves, grave{after: after, item: item})
}

func (d *Device) batchUses(item destroyer) bool {
	if d.batch == 0 {
		return false
	}
	u, tracked := item.(batchUser)
	return !tracked || u.batchedIn() > d.submitted
}

func (d *Device) nextSubmission() uint64 { return d.submitted + 1 }

func (d *Device) discard(item destroyer) {
	if !d.framing && d.active == nil {
		d.reap(false)
	}
	d.retire(item)
}

func (d *Device) buryFinished(through uint64) {
	n := 0
	for n < len(d.graves) && d.graves[n].after <= through {
		d.graves[n].item.destroy()
		d.graves[n] = grave{}
		n++
	}
	if n == 0 {
		return
	}
	left := copy(d.graves, d.graves[n:])
	clear(d.graves[left:])
	d.graves = d.graves[:left]
}

func (d *Device) endFraming() { d.framing = false }

func (d *Device) buryAll() { d.buryFinished(math.MaxUint64) }

func (s *surface) destroy() { s.close() }

func (b *buffer) destroy() { b.close() }

func (t *Texture) destroy() { t.release() }

func (l *Linear) destroy() { l.release() }

func (b *Buffer) destroy() {
	if b.sampled != 0 {
		b.d.cmds.DestroyImageView(b.d.device, b.sampled, nil)
		b.sampled = 0
	}
	b.e.close(b.d)
}

func (t *Target) destroy() {
	d := t.s.d
	if t.tex != nil {
		t.tex.release()
	}
	t.s.close()
	if t.offscreen {
		return
	}
	d.cmds.DestroyImage(d.device, t.s.image, nil)
	d.freeMemory(t.memory)
	t.s.image, t.memory = 0, nil
}

func (r *Readback) destroy() {
	d := r.d
	if r.copied {
		_ = d.awaitWork(r.waitRound)
	}
	d.cmds.DestroyFence(d.device, r.fence, nil)
	r.buf.close()
}

// BlockingWaits counts the times this device's goroutine has waited for
// the GPU to finish: a frame that adds to it could have missed its
// deadline for the work ahead of it.
func (d *Device) BlockingWaits() uint64 { return d.blockingWaits }

// OpaqueRuns counts the runs of draws this device recorded with blending
// off: opaque fills, opaque textures at full alpha, unmasked, and Kawase
// levels.
func (d *Device) OpaqueRuns() uint64 { return d.opaqueRuns }

// Submissions counts the queue submissions this device made: a frame of
// passes followed by SyncFile is one.
func (d *Device) Submissions() uint64 { return d.submitted }
