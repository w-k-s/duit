package api

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"io"
	"net"
	"os"
	"syscall"
)

func encodeGzippedJSON(w io.Writer, val interface{}) error {
	gz := gzip.NewWriter(w)
	err := json.NewEncoder(gz).Encode(val)
	if err != nil {
		return err
	}

	return gz.Close()
}

func checkError(err error) {
	if err == nil || err == sql.ErrNoRows {
		return
	}

	// Check for a broken connection, as it is not really a
	// condition that warrants a panic stack trace.
	if ne, ok := err.(*net.OpError); ok {
		if se, ok := ne.Err.(*os.SyscallError); ok {
			if se.Err == syscall.EPIPE || se.Err == syscall.ECONNRESET {
				return
			}
		}
	}

	panic(err)
}
