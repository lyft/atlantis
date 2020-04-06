package vcs

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v31/github"
)

// GithubCredentials handles creating http.Clients that authenticate
type GithubCredentials interface {
	Client() (*http.Client, error)
	GetToken() (string, error)
	GetUser() string
}

// GithubAnonymousCredentials expose no credentials
type GithubAnonymousCredentials struct{}

// Client returns a client with no credentials
func (c *GithubAnonymousCredentials) Client() (*http.Client, error) {
	tr := http.DefaultTransport
	return &http.Client{Transport: tr}, nil
}

// GetUser returns the username for these credentials
func (c *GithubAnonymousCredentials) GetUser() string {
	return "anonymous"
}

// GetToken returns an empty token
func (c *GithubAnonymousCredentials) GetToken() (string, error) {
	return "", nil
}

// GithubUserCredentials implements GithubCredentials for the personal auth token flow
type GithubUserCredentials struct {
	User  string
	Token string
}

// Client returns a client for basic auth user credentials
func (c *GithubUserCredentials) Client() (*http.Client, error) {
	tr := &github.BasicAuthTransport{
		Username: strings.TrimSpace(c.User),
		Password: strings.TrimSpace(c.Token),
	}
	return tr.Client(), nil
}

// GetUser returns the username for these credentials
func (c *GithubUserCredentials) GetUser() string {
	return c.User
}

// GetToken returns the user token
func (c *GithubUserCredentials) GetToken() (string, error) {
	return c.Token, nil
}

// GithubAppCredentials implements GithubCredentials for github app installation token flow
type GithubAppCredentials struct {
	AppID          int64
	KeyPath        string
	Hostname       string
	apiURL         *url.URL
	installationID int64
	token          *github.InstallationToken
}

// Client returns a github app installation client
func (c *GithubAppCredentials) Client() (*http.Client, error) {
	itr, err := c.transport()
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: itr}, nil
}

// GetUser returns the username for these credentials
func (c *GithubAppCredentials) GetUser() string {
	return ""
}

// GetToken returns a fresh installation token
func (c *GithubAppCredentials) GetToken() (string, error) {
	if c.token != nil {
		log.Println(c.token.GetExpiresAt())
		return c.token.GetToken(), nil
	}

	transport, err := c.Client()
	if err != nil {
		return "", err
	}

	apiURL := c.getAPIURL()
	var client *github.Client

	if apiURL.Hostname() == "github.com" {
		client = github.NewClient(transport)
	} else {
		client, _ = github.NewEnterpriseClient(apiURL.String(), apiURL.String(), transport)
	}

	installationID, err := c.getInstallationID()
	if err != nil {
		return "", err
	}

	token, _, err := client.Apps.CreateInstallationToken(context.Background(), installationID, &github.InstallationTokenOptions{})

	if err != nil {
		return "", err
	}

	c.token = token
	return token.GetToken(), nil
}

func (c *GithubAppCredentials) getInstallationID() (int64, error) {
	if c.installationID != 0 {
		return c.installationID, nil
	}

	tr := http.DefaultTransport
	// A non-installation transport
	t, err := ghinstallation.NewAppsTransportKeyFromFile(tr, c.AppID, c.KeyPath)
	t.BaseURL = c.getAPIURL().String()
	if err != nil {
		return 0, err
	}

	// Query github with the app's JWT
	client := github.NewClient(&http.Client{Transport: t})
	client.BaseURL = c.getAPIURL()
	ctx := context.Background()
	app, _, err := client.Apps.Get(ctx, "")
	if err != nil {
		return 0, err
	}

	c.installationID = app.GetID()
	return c.installationID, nil
}

func (c *GithubAppCredentials) transport() (*ghinstallation.Transport, error) {
	installationID, err := c.getInstallationID()
	if err != nil {
		return nil, err
	}

	tr := http.DefaultTransport
	itr, err := ghinstallation.NewKeyFromFile(tr, c.AppID, installationID, c.KeyPath)
	if err == nil {
		itr.BaseURL = c.getAPIURL().String()
	}
	return itr, err
}

func (c *GithubAppCredentials) getAPIURL() *url.URL {
	if c.apiURL != nil {
		return c.apiURL
	}

	c.apiURL = resolveGithubAPIURL(c.Hostname)
	return c.apiURL
}

func resolveGithubAPIURL(hostname string) *url.URL {
	// If we're using github.com then we don't need to do any additional configuration
	// for the client. It we're using Github Enterprise, then we need to manually
	// set the base url for the API.
	baseURL := &url.URL{
		Scheme: "https",
		Host:   "api.github.com",
		Path:   "/",
	}

	if hostname != "github.com" {
		baseURL.Host = hostname
		baseURL.Path = "/api/v3/"
	}

	return baseURL
}
