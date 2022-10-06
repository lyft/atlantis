package stow

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/graymeta/stow"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
)

type CloserFn func()

func NewClient(storeConfig valid.Store) (*Client, error) {
	location, err := stow.Dial(string(storeConfig.BackendType), storeConfig.Config)
	if err != nil {
		return nil, err
	}

	return &Client{
		location:      location,
		containerName: storeConfig.ContainerName,
	}, nil
}

type Client struct {
	location      stow.Location
	containerName string
	prefix        string
}

func (c *Client) Get(ctx context.Context, key string) (io.ReadCloser, CloserFn, error) {
	container, err := c.location.Container(c.containerName)
	if err != nil {
		return nil, nil, errors.Wrap(err, "resolving container")
	}

	key = c.addPrefix(key)
	item, err := container.Item(key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "getting item")
	}

	r, err := item.Open()
	if err != nil {
		return nil, nil, errors.Wrap(err, "reading item")
	}

	closerFn := func() {
		r.Close()
	}

	return r, closerFn, nil
}

func (c *Client) Set(ctx context.Context, key string, object []byte) error {
	container, err := c.location.Container(c.containerName)
	if err != nil {
		return errors.Wrap(err, "resolving container")
	}

	key = c.addPrefix(key)
	_, err = container.Put(key, bytes.NewReader(object), int64(len(object)), nil)
	if err != nil {
		return errors.Wrap(err, "writing to container")
	}
	return nil
}

func (c *Client) addPrefix(key string) string {
	return fmt.Sprintf("%s/%s", c.prefix, key)
}
