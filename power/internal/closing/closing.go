// Package closing joins the error of closing a resource into the error
// its user returns.
package closing

import (
	"errors"
	"fmt"
	"io"
)

// Join closes c and joins a failure, naming what was closed, into err.
func Join(err *error, what string, c io.Closer) {
	if cerr := c.Close(); cerr != nil {
		*err = errors.Join(*err, fmt.Errorf("close the %s: %w", what, cerr))
	}
}
