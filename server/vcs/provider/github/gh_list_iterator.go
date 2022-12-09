package github

import (
	"context"
	"fmt"
	gh "github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"net/http"
)

type ListIterator struct {
	ClientCreator githubapp.ClientCreator
}

func (i *ListIterator) Iterate(ctx context.Context, installationToken int64, runFunc func(ctx context.Context, client *gh.Client, nextPage int) (interface{}, *gh.Response, error), parseFunc func(interface{}) []string) ([]string, error) {
	client, err := i.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return nil, errors.Wrap(err, "creating installation client")
	}

	var output []string
	nextPage := 0
	for {
		results, resp, err := runFunc(ctx, client, nextPage)
		if err != nil {
			return nil, errors.Wrap(err, "error running gh api call")
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("not ok status running gh api call: %s", resp.Status)
		}
		output = append(output, parseFunc(results)...)
		if resp.NextPage == 0 {
			break
		}
		nextPage = resp.NextPage
	}
	return output, nil
}
