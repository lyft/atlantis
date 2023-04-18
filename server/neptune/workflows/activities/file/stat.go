package file

import "os"

type Validator struct{}

func (s *Validator) Stat(name string) error {
	_, err := os.Stat(name)
	return err
}
