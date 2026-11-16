package widget

import (
	"slices"
	"sync"

	"github.com/stubbedev/lys/lib/assetfile"
)

const maxAssetQueue = 1024

type assetJob struct {
	key     any
	load    func()
	waiters []assetWaiter
}

type assetWaiter struct {
	loop *eventLoop
	done func()
}

const maxAssetWorkers = 8

var assets = newAssetQueue()

var readAssetFile = assetfile.Read

type assetQueue struct {
	mu      sync.Mutex
	idle    sync.Cond
	queue   []*assetJob
	jobs    map[any]*assetJob
	running int
}

func newAssetQueue() *assetQueue {
	q := &assetQueue{jobs: map[any]*assetJob{}}
	q.idle.L = &q.mu
	return q
}

func loadAsset(key any, l *eventLoop, load, done func()) {
	if l == nil {
		load()
		done()
		return
	}
	if dropped := assets.add(key, load, assetWaiter{loop: l, done: done}); dropped != nil {
		answer(dropped)
	}
}

func (q *assetQueue) add(key any, load func(), w assetWaiter) (dropped *assetJob) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if j, ok := q.jobs[key]; ok {
		j.waiters = append(j.waiters, w)
		return nil
	}
	if len(q.queue) >= maxAssetQueue {
		oldest := q.queue[0]
		q.queue = slices.Delete(q.queue, 0, 1)
		delete(q.jobs, oldest.key)
		dropped = oldest
	}
	j := &assetJob{key: key, load: load, waiters: []assetWaiter{w}}
	q.jobs[key] = j
	q.queue = append(q.queue, j)
	if q.running < maxAssetWorkers {
		q.running++
		go q.run()
	}
	return dropped
}

func (q *assetQueue) run() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.queue) > 0 {
		j := q.queue[0]
		q.queue = slices.Delete(q.queue, 0, 1)
		q.mu.Unlock()
		j.load()
		q.mu.Lock()
		delete(q.jobs, j.key)
		q.mu.Unlock()
		answer(j)
		q.mu.Lock()
	}
	q.running--
	if q.running == 0 {
		q.idle.Broadcast()
	}
}

func answer(j *assetJob) {
	for i, w := range j.waiters {
		if slices.ContainsFunc(j.waiters[:i], func(o assetWaiter) bool { return o.loop == w.loop }) {
			continue
		}
		mine := j.waiters[i:]
		w.loop.invoke(func() {
			for _, o := range mine {
				if o.loop == w.loop {
					o.done()
				}
			}
		})
	}
}

func (q *assetQueue) forget(l *eventLoop) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queue = slices.DeleteFunc(q.queue, func(j *assetJob) bool {
		j.waiters = slices.DeleteFunc(j.waiters, func(w assetWaiter) bool { return w.loop == l })
		if len(j.waiters) > 0 {
			return false
		}
		delete(q.jobs, j.key)
		return true
	})
}

func (q *assetQueue) wait() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.running > 0 {
		q.idle.Wait()
	}
}

// WaitAssetLoads blocks until every icon, stylesheet image and font load
// asked for so far has finished and queued its answer on the loop, whose
// next pass then applies it. It is what a test that needs a settled frame
// waits on; a UI loop must never call it.
func WaitAssetLoads() { assets.wait() }

func owe(root *node) (settle func()) {
	root.owedAssets++
	return func() { root.owedAssets-- }
}

// TreeAssetsOwed reports how many icons and images widgets in root's
// tree have asked for whose answers have yet to land on the loop.
func TreeAssetsOwed(root Widget) int {
	n := nodeOf(root)
	if n == nil {
		return 0
	}
	return n.owedAssets
}
