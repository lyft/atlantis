package file

import "os"

type Writer struct{}

func (f *Writer) Write(name string, data []byte) error {
	return os.WriteFile(name, data, os.ModePerm) //nolint:gosec
}
