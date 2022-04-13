package converter

import (
	"fmt"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/vcs/types/event"
)

type CommentEventConverter struct {
	RepoConverter    RepoConverter
	GithubPullGetter events.GithubPullGetter
	EventParser      events.EventParsing
}

// Convert converts github issue comment events to our internal representation
// TODO: remove named methods and return CommentEvent directly /techdebt
func (e CommentEventConverter) Convert(comment *github.IssueCommentEvent) (event.Comment, error) {
	baseRepo, err := e.RepoConverter.Convert(comment.Repo)
	if err != nil {
		return event.Comment{}, err
	}
	if comment.Comment == nil || comment.Comment.User.GetLogin() == "" {
		return event.Comment{}, fmt.Errorf("comment.user.login is null")
	}
	commenterUsername := comment.Comment.User.GetLogin()
	user := models.User{
		Username: commenterUsername,
	}
	pullNum := comment.Issue.GetNumber()
	if pullNum == 0 {
		return event.Comment{}, fmt.Errorf("issue.number is null")
	}

	eventTimestamp := time.Now()
	if comment.Comment.CreatedAt != nil {
		eventTimestamp = *comment.Comment.CreatedAt
	}

	// Get the pull req using pull num and base repo
	pull, headRepo, err := e.getGithubData(baseRepo, pullNum)
	if err != nil {
		return event.Comment{}, errors.Wrap(err, "getting pull from github")
	}

	return event.Comment{
		BaseRepo:  baseRepo,
		HeadRepo:  &headRepo,
		Pull:      &pull,
		User:      user,
		PullNum:   pullNum,
		VCSHost:   models.Github,
		Timestamp: eventTimestamp,
		Comment:   comment.GetComment().GetBody(),
	}, nil
}

func (c *CommentEventConverter) getGithubData(baseRepo models.Repo, pullNum int) (models.PullRequest, models.Repo, error) {
	ghPull, err := c.GithubPullGetter.GetPullRequest(baseRepo, pullNum)
	if err != nil {
		return models.PullRequest{}, models.Repo{}, errors.Wrap(err, "making pull request API call to GitHub")
	}
	pull, _, headRepo, err := c.EventParser.ParseGithubPull(ghPull)
	if err != nil {
		return pull, headRepo, errors.Wrap(err, "extracting required fields from comment data")
	}
	return pull, headRepo, nil
}

/*

func TestRunCommentCommand_GithubPullErrorf(t *testing.T) {
	t.Log("if getting the github pull request fails an error should be logged")
	vcsClient := setup(t)
	ctx := context.Background()
	When(githubGetter.GetPullRequest(fixtures.GithubRepo, fixtures.Pull.Num)).ThenReturn(nil, errors.New("err"))
	ch.RunCommentCommand(ctx, fixtures.GithubRepo, &fixtures.GithubRepo, nil, fixtures.User, fixtures.Pull.Num, nil, time.Now())
	vcsClient.VerifyWasCalledOnce().CreateComment(fixtures.GithubRepo, fixtures.Pull.Num, "`Error: making pull request API call to GitHub: err`", "")
}

func TestRunCommentCommand_GitlabMergeRequestErrorf(t *testing.T) {
	t.Log("if getting the gitlab merge request fails an error should be logged")
	vcsClient := setup(t)
	ctx := context.Background()
	When(gitlabGetter.GetMergeRequest(fixtures.GitlabRepo.FullName, fixtures.Pull.Num)).ThenReturn(nil, errors.New("err"))
	ch.RunCommentCommand(ctx, fixtures.GitlabRepo, &fixtures.GitlabRepo, nil, fixtures.User, fixtures.Pull.Num, nil, time.Now())
	vcsClient.VerifyWasCalledOnce().CreateComment(fixtures.GitlabRepo, fixtures.Pull.Num, "`Error: making merge request API call to GitLab: err`", "")
}

func TestRunCommentCommand_GithubPullParseErrorf(t *testing.T) {
	t.Log("if parsing the returned github pull request fails an error should be logged")
	vcsClient := setup(t)
	ctx := context.Background()
	var pull github.PullRequest
	When(githubGetter.GetPullRequest(fixtures.GithubRepo, fixtures.Pull.Num)).ThenReturn(&pull, nil)
	When(eventParsing.ParseGithubPull(&pull)).ThenReturn(fixtures.Pull, fixtures.GithubRepo, fixtures.GitlabRepo, errors.New("err"))

	ch.RunCommentCommand(ctx, fixtures.GithubRepo, &fixtures.GithubRepo, nil, fixtures.User, fixtures.Pull.Num, nil, time.Now())
	vcsClient.VerifyWasCalledOnce().CreateComment(fixtures.GithubRepo, fixtures.Pull.Num, "`Error: extracting required fields from comment data: err`", "")
}

*/
