package raw

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/runatlantis/atlantis/server/config/valid"
)

type RevisionSetter struct {
	URL       string    `yaml:"url" json:"url"`
	BasicAuth BasicAuth `yaml:"basic_auth" json:"basic_auth"`

	DefaultTaskQueue TaskQueue `yaml:"default_task_queue" json:"default_task_queue"`
	SlowTaskQueue    TaskQueue `yaml:"slow_task_queue" json:"slow_task_queue"`
}

func (p RevisionSetter) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.URL, validation.Required, is.URL),
		validation.Field(&p.BasicAuth),
		validation.Field(&p.DefaultTaskQueue, validation.Required),
		validation.Field(&p.SlowTaskQueue, validation.Required),
	)
}

func (p *RevisionSetter) ToValid() valid.RevisionSetter {
	return valid.RevisionSetter{
		BasicAuth:        p.BasicAuth.ToValid(),
		URL:              p.URL,
		DefaultTaskQueue: p.DefaultTaskQueue.ToValid(),
		SlowTaskQueue:    p.SlowTaskQueue.ToValid(),
	}
}

type BasicAuth struct {
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

func (b BasicAuth) Validate() error {
	return validation.ValidateStruct(&b,
		validation.Field(&b.Username, validation.Required),
		validation.Field(&b.Password, validation.Required),
	)
}

func (b BasicAuth) ToValid() valid.BasicAuth {
	return valid.BasicAuth{
		Username: b.Username,
		Password: b.Password,
	}
}

type TaskQueue struct {
	ActivitesPerSecond float64 `yaml:"activities_per_second" json:"activities_per_second"`
}

func (t TaskQueue) Validate() error {
	return validation.ValidateStruct(&t,
		validation.Field(&t.ActivitesPerSecond, validation.Required))
}

func (t TaskQueue) ToValid() valid.TaskQueue {
	return valid.TaskQueue{
		ActivitiesPerSecond: t.ActivitesPerSecond,
	}
}
