package main

import (
	"bytes"
	"compress/gzip"
)

func gzipped(data []byte) (int, error) {
	var out bytes.Buffer
	w, err := gzip.NewWriterLevel(&out, gzip.BestCompression)
	if err != nil {
		return 0, err
	}
	if _, err := w.Write(data); err != nil {
		return 0, err
	}
	if err := w.Close(); err != nil {
		return 0, err
	}
	return out.Len(), nil
}
