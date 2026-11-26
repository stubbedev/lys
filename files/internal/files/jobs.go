package files

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"slices"
	"sync"

	"github.com/stubbedev/lys/desktop/fileops"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/files/internal/locale"
)

// kind is what an operation does, as its progress and its failure
// say.
type kind uint8

const (
	copying kind = iota + 1
	moving
	deleting
	trashing
	emptying
	undoing
)

// progress is the message a running operation shows.
func (k kind) progress() string {
	return [...]string{
		copying: "files-progress-copy", moving: "files-progress-move", deleting: "files-progress-delete",
		trashing: "files-progress-trash", emptying: "files-progress-empty",
		undoing: "files-progress-undo",
	}[k]
}

// failed is the message an operation that stopped on an error shows.
func (k kind) failed() string {
	return [...]string{
		copying: "files-copy-failed", moving: "files-move-failed", deleting: "files-delete-failed",
		trashing: "files-trash-failed", emptying: "files-empty-failed",
		undoing: "files-undo-failed",
	}[k]
}

// The responses of the conflict dialog.
const (
	answerSkip     = "skip"
	answerKeepBoth = "keep-both"
	answerReplace  = "replace"
	answerCancel   = "cancel"
)

// actionOf is the action a conflict response takes; Cancel skips, and
// cancels the operation.
var actionOf = map[string]fileops.Action{
	answerSkip: fileops.Skip, answerKeepBoth: fileops.KeepBoth, answerReplace: fileops.Replace, answerCancel: fileops.Skip,
}

// jobs are the operations running and the panel that shows them.
type jobs struct {
	f       *Files
	panel   *widget.Box
	running []*job
	wg      sync.WaitGroup
}

func newJobs(f *Files) *jobs {
	panel := widget.NewBox(widget.Column, 0, 0)
	panel.AddClass("files-jobs")
	panel.SetVisible(false)
	return &jobs{f: f, panel: panel}
}

// job is one operation running: its row in the panel, and what it
// last reported off the loop.
type job struct {
	js       *jobs
	kind     kind
	j        *fileops.Job
	row      *widget.Box
	label    *widget.Label
	bar      *widget.ProgressBar
	stop     chan struct{}
	stopOnce sync.Once
	mu       sync.Mutex
	latest   fileops.Progress
	queued   bool
	// dismiss takes down the conflict dialog showing, nil for none.
	dismiss func()
}

// start runs op as k, showing its progress until it ends; done hears
// what undoes it once it succeeded, or stopped after doing some of it.
func (js *jobs) start(k kind, op fileops.Op, done func(*fileops.Undo)) {
	f := js.f
	jb := &job{js: js, kind: k, stop: make(chan struct{})}
	jb.row = filebrowser.Section(widget.Row, "files-job")
	jb.label = widget.NewLabel(f.cfg.Browser.Font, widgets.TextPx(styling.Body, 1), progressText(k, fileops.Progress{}), 0)
	jb.label.SetEllipsize(widget.EllipsizeEnd)
	jb.bar = widget.NewProgressBar(0)
	cancel := widget.NewButton(widget.NewLabel(f.cfg.Browser.Font, widgets.TextPx(styling.Body, 1), locale.T().Get("files-cancel"), 0), 0, 0)
	cancel.AddClass("files-job-cancel", css.Ghost)
	cancel.OnClick = func() {
		jb.closeDialog()
		jb.cancel()
	}
	jb.row.AppendAligned(jb.label, true, widget.AlignCenter)
	jb.row.AppendAligned(jb.bar, true, widget.AlignCenter)
	jb.row.AppendAligned(cancel, false, widget.AlignCenter)
	js.panel.Append(jb.row, false)
	js.panel.SetVisible(true)
	js.running = append(js.running, jb)
	jb.j = fileops.Start(op, jb)
	js.wg.Go(func() {
		undo, err := jb.j.Wait()
		f.cfg.Host.Invoke(func() { js.finish(jb, undo, err, done) })
	})
}

// finish takes a job's row down and tells how it ended: an error that
// was no cancel says so, and what can be undone is kept.
func (js *jobs) finish(jb *job, undo *fileops.Undo, err error, done func(*fileops.Undo)) {
	jb.closeDialog()
	js.panel.Remove(jb.row)
	js.running = slices.DeleteFunc(js.running, func(r *job) bool { return r == jb })
	js.panel.SetVisible(len(js.running) > 0)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("files: %v", err)
		js.f.cfg.Host.Toast(locale.T().Get(jb.kind.failed()), nil)
	}
	if done != nil && (err == nil || undo != nil) {
		done(undo)
	}
}

// cancelAll cancels every operation running.
func (js *jobs) cancelAll() {
	for _, jb := range js.running {
		jb.cancel()
	}
}

// wait waits for every operation started to end.
func (js *jobs) wait() { js.wg.Wait() }

// cancel stops the job, answering a conflict it waits on.
func (jb *job) cancel() {
	jb.stopOnce.Do(func() { close(jb.stop) })
	jb.j.Cancel()
}

// Progress implements fileops.Observer: the row shows the latest
// report once the loop gets to it.
func (jb *job) Progress(p fileops.Progress) {
	jb.mu.Lock()
	jb.latest = p
	queue := !jb.queued
	jb.queued = true
	jb.mu.Unlock()
	if queue {
		jb.js.f.cfg.Host.Invoke(jb.show)
	}
}

// show puts the latest report in the row.
func (jb *job) show() {
	jb.mu.Lock()
	p := jb.latest
	jb.queued = false
	jb.mu.Unlock()
	jb.label.SetText(progressText(jb.kind, p))
	jb.bar.SetValue(fraction(p))
}

// progressText says how far an operation of kind k is.
func progressText(k kind, p fileops.Progress) string {
	tr := locale.T()
	return tr.Get(k.progress(), i18n.Str("done", tr.Integer(p.Files)), i18n.Str("of", tr.Integer(p.TotalFiles)))
}

// fraction is how far p is: by bytes when it moves any, else by files.
func fraction(p fileops.Progress) float64 {
	switch {
	case p.TotalBytes > 0:
		return float64(p.Bytes) / float64(p.TotalBytes)
	case p.TotalFiles > 0:
		return float64(p.Files) / float64(p.TotalFiles)
	}
	return 0
}

// Resolve implements fileops.Observer: it asks on the loop, and waits
// for the answer or the job's cancel.
func (jb *job) Resolve(c fileops.Conflict) fileops.Resolution {
	answer := make(chan fileops.Resolution, 1)
	jb.js.f.cfg.Host.Invoke(func() { jb.ask(c, answer) })
	select {
	case r := <-answer:
		return r
	case <-jb.stop:
		return fileops.Resolution{Action: fileops.Skip}
	}
}

// ask shows the conflict dialog: skip, keep both or replace, for this
// conflict or every later one of the job; Cancel cancels the job.
func (jb *job) ask(c fileops.Conflict, answer chan<- fileops.Resolution) {
	select {
	case <-jb.stop:
		return
	default:
	}
	f := jb.js.f
	all := widget.NewCheckButton(false)
	all.SetChild(widget.NewLabel(f.cfg.Browser.Font, widgets.TextPx(styling.Body, 1), locale.T().Get("files-conflict-all"), 0))
	all.AddClass("files-conflict-all")
	body := "files-conflict-file"
	if c.TargetIsDir {
		body = "files-conflict-folder"
	}
	jb.dismiss = f.cfg.Host.Ask(Question{
		Title:   locale.T().Get("files-conflict-title", i18n.Str("name", filepath.Base(c.Target)), i18n.Str("folder", filepath.Base(filepath.Dir(c.Target)))),
		Body:    locale.T().Get(body),
		Content: all,
		Buttons: []app.DialogButton{
			{Label: locale.T().Get("files-cancel"), Response: answerCancel, Role: app.ButtonRoleCancel},
			{Label: locale.T().Get("files-skip"), Response: answerSkip},
			{Label: locale.T().Get("files-keep-both"), Response: answerKeepBoth},
			{Label: locale.T().Get("files-replace"), Response: answerReplace, Role: app.ButtonRoleDefault, Appearance: app.AppearanceDestructive},
		},
		Default: answerReplace, Cancel: answerCancel,
		Answer: func(response string) {
			jb.dismiss = nil
			a, ok := actionOf[response]
			if !ok || response == answerCancel {
				jb.cancel()
			}
			if !ok {
				a = fileops.Skip
			}
			answer <- fileops.Resolution{Action: a, All: all.Checked()}
		},
	})
}

// closeDialog takes the conflict dialog down.
func (jb *job) closeDialog() {
	if jb.dismiss != nil {
		jb.dismiss()
		jb.dismiss = nil
	}
}

// pushUndo keeps u as the last thing to undo.
func (f *Files) pushUndo(u *fileops.Undo) {
	if u != nil {
		f.undos = append(f.undos, u)
	}
}

// undo reverses the last move, rename or trash (Ctrl+Z); with none it
// says so.
func (f *Files) undo() {
	if len(f.undos) == 0 {
		f.cfg.Host.Toast(locale.T().Get("files-nothing-to-undo"), nil)
		return
	}
	f.runUndo(f.undos[len(f.undos)-1])
}

// runUndo reverses u and drops it from what can be undone.
func (f *Files) runUndo(u *fileops.Undo) {
	i := slices.Index(f.undos, u)
	if i < 0 {
		return
	}
	f.undos = slices.Delete(f.undos, i, i+1)
	f.jobs.start(undoing, u, nil)
}
