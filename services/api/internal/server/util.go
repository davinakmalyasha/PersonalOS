package server

import (
	"bytes"
	"strconv"
)

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

func atoi(s string) (int, error) { return strconv.Atoi(s) }
