package app

import (
	"errors"

	"github.com/ncruces/zenity"
)

// fileRequest is what a dialog asks the user for.
type fileRequest struct {
	title string

	// start is the file or folder the dialog opens at.
	start string

	// folder asks for a folder rather than a file.
	folder bool
}

// fileDialogs asks the user for somewhere to read or write. It returns an
// empty path and no error when the user cancels.
//
// The application shows the system's own dialogs. Tests answer with paths of
// their own, since a test cannot click through a real one.
type fileDialogs interface {
	choose(request fileRequest) (string, error)
}

// systemDialogs shows the native dialogs: the common dialogs on Windows, and
// zenity or kdialog on Linux, whichever the desktop has.
type systemDialogs struct {
	// parent is the window the dialogs belong to, so that they open over it
	// and keep it from being used meanwhile.
	parent zenity.Option
}

func (d systemDialogs) choose(request fileRequest) (string, error) {
	options := []zenity.Option{zenity.Title(request.title), d.parent}

	if request.start != "" {
		options = append(options, zenity.Filename(request.start))
	}

	if request.folder {
		options = append(options, zenity.Directory())
	}

	path, err := zenity.SelectFile(options...)
	if errors.Is(err, zenity.ErrCanceled) {
		return "", nil
	}

	return path, err
}

// pendingDialog is a dialog that is open, and what to do with its answer.
type pendingDialog struct {
	answer chan dialogAnswer
	then   func(path string)
}

type dialogAnswer struct {
	path string
	err  error
}

// ask shows a dialog without holding up the window: it runs on a goroutine of
// its own, and then runs on the interface goroutine once the user answered.
// Only one dialog is open at a time; asking while one is open does nothing.
func (a *App) ask(request fileRequest, then func(path string)) {
	if a.state.dialog != nil {
		return
	}

	answer := make(chan dialogAnswer, 1)
	dialogs := a.dialogs

	go func() {
		path, err := dialogs.choose(request)
		answer <- dialogAnswer{path: path, err: err}
	}()

	a.state.dialog = &pendingDialog{answer: answer, then: then}
}

// pollDialog picks up the answer of the open dialog, if it has arrived.
func (a *App) pollDialog() {
	pending := a.state.dialog
	if pending == nil {
		return
	}

	select {
	case answer := <-pending.answer:
		a.state.dialog = nil

		switch {
		case answer.err != nil:
			warn(answer.err)
			a.setStatus("The file dialog could not be shown: %v", answer.err)

		case answer.path != "":
			pending.then(answer.path)
		}

	default:
	}
}
