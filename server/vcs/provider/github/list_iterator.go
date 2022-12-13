package github

import (
	"context"
	"fmt"
	gh "github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"net/http"
)

// Add supported types here with each GH api fxn introduced

func Iterate[T []*gh.CommitFile | []*gh.PullRequestReview | []*gh.CheckRun, R []string](
	ctx context.Context,
	runFunc func(ctx context.Context, nextPage int) (T, *gh.Response, error),
	processFunc func(T) R) (R, error) {

	var output R
	nextPage := 0
	for {
		results, resp, err := runFunc(ctx, nextPage)
		if err != nil {
			return nil, errors.Wrap(err, "error running gh api call")
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("not ok status running gh api call: %s", resp.Status)
		}
		output = append(output, processFunc(results)...)
		if resp.NextPage == 0 {
			break
		}
		nextPage = resp.NextPage
	}
	return output, nil
}
