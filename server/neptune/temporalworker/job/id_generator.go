package job

import "github.com/google/uuid"

type IdGenerator struct{}

func (i *IdGenerator) GenerateID() string {
	return uuid.New().String()
}
