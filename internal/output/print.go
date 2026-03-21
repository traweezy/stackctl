package output

import (
	"fmt"
	"io"
)

const (
	StatusOK   = "OK"
	StatusMiss = "MISS"
	StatusWarn = "WARN"
	StatusFail = "FAIL"
)

func StatusLine(w io.Writer, status, message string) error {
	_, err := fmt.Fprintf(w, "[%-4s] %s\n", status, message)
	return err
}
