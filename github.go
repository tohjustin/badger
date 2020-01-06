package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/shurcooL/githubv4"
	"github.com/tohjustin/badger/pkg/badge"
	"golang.org/x/oauth2"
)

type RepositoryService interface {
	getForkCount(owner string, repo string) (int, error)
	getIssueCount(owner string, repo string, issueState string) (int, error)
	getPullRequestCount(owner string, repo string, pullRequestState string) (int, error)
	getStargazerCount(owner string, repo string) (int, error)

	Handler(w http.ResponseWriter, r *http.Request)
}

func NewGithubService() RepositoryService {
	// Create new Github GraphQL client
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")})
	httpClient := oauth2.NewClient(context.Background(), tokenSource)

	return &githubService{
		client: githubv4.NewClient(httpClient),
	}
}

type githubService struct {
	client *githubv4.Client
}

func (service *githubService) getForkCount(owner string, repo string) (int, error) {
	var query struct {
		Repository struct {
			Forks struct {
				TotalCount int
			}
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
	}

	err := service.client.Query(context.Background(), &query, variables)
	return query.Repository.Forks.TotalCount, err
}

func (service *githubService) getIssueCount(owner string, repo string, issueState string) (int, error) {
	var issueStates []githubv4.IssueState
	var query struct {
		Repository struct {
			Issues struct {
				TotalCount int
			} `graphql:"issues(states: $states)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	switch issueState {
	case "open":
		issueStates = []githubv4.IssueState{githubv4.IssueStateOpen}
	case "closed":
		issueStates = []githubv4.IssueState{githubv4.IssueStateClosed}
	default:
		issueStates = []githubv4.IssueState{
			githubv4.IssueStateOpen,
			githubv4.IssueStateClosed,
		}
	}
	variables := map[string]interface{}{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"states": issueStates,
	}

	err := service.client.Query(context.Background(), &query, variables)
	return query.Repository.Issues.TotalCount, err
}

func (service *githubService) getPullRequestCount(owner string, repo string, pullRequestState string) (int, error) {
	var pullRequestStates []githubv4.PullRequestState
	var query struct {
		Repository struct {
			PullRequests struct {
				TotalCount int
			} `graphql:"pullRequests(states: $states)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	switch pullRequestState {
	case "open":
		pullRequestStates = []githubv4.PullRequestState{githubv4.PullRequestStateOpen}
	case "closed":
		pullRequestStates = []githubv4.PullRequestState{githubv4.PullRequestStateClosed}
	case "merged":
		pullRequestStates = []githubv4.PullRequestState{githubv4.PullRequestStateMerged}
	default:
		pullRequestStates = []githubv4.PullRequestState{
			githubv4.PullRequestStateOpen,
			githubv4.PullRequestStateClosed,
			githubv4.PullRequestStateMerged,
		}
	}
	variables := map[string]interface{}{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"states": pullRequestStates,
	}

	err := service.client.Query(context.Background(), &query, variables)
	return query.Repository.PullRequests.TotalCount, err
}

func (service *githubService) getStargazerCount(owner string, repo string) (int, error) {
	var query struct {
		Repository struct {
			Stargazers struct {
				TotalCount int
			}
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	variables := map[string]interface{}{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
	}

	err := service.client.Query(context.Background(), &query, variables)
	return query.Repository.Stargazers.TotalCount, err
}

func (service *githubService) Handler(w http.ResponseWriter, r *http.Request) {
	routeVariables := mux.Vars(r)
	owner := routeVariables["owner"]
	repo := routeVariables["repo"]
	requestType := routeVariables["requestType"]

	// Fetch data
	var color, status, subject string
	var value int
	var err error
	switch requestType {
	case "forks":
		subject = "forks"
		value, err = service.getForkCount(owner, repo)
	case "issues":
		state := r.URL.Query().Get("state")
		switch state {
		case "open":
			subject = "open issues"
		case "closed":
			subject = "closed issues"
		default:
			subject = "issues"
		}
		value, err = service.getIssueCount(owner, repo, state)
	case "pull-requests":
		state := r.URL.Query().Get("state")
		switch state {
		case "open":
			subject = "open PRs"
		case "closed":
			subject = "closed PRs"
		case "merged":
			subject = "merged PRs"
		default:
			subject = "PRs"
		}
		value, err = service.getPullRequestCount(owner, repo, state)
	case "stars":
		subject = "stars"
		value, err = service.getStargazerCount(owner, repo)
	}

	// Compute status
	if err != nil {
		status = err.Error()
	} else {
		status = strconv.Itoa(value)
	}

	// Overwrite any badge texts
	if queryColor := r.URL.Query().Get("color"); queryColor != "" {
		color = queryColor
	}
	if queryStatus := r.URL.Query().Get("status"); queryStatus != "" {
		status = queryStatus
	}
	if querySubject := r.URL.Query().Get("subject"); querySubject != "" {
		subject = querySubject
	}
	icon := r.URL.Query().Get("icon")
	style := r.URL.Query().Get("style")

	// Generate badge
	createOptions := badge.Options{Color: color, Icon: icon, Style: badge.Style(style)}
	generatedBadge, err := badge.Create(subject, status, &createOptions)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		fmt.Println(err)
		return
	}

	// cache response in browser for 1 hour (3600), CDN for 1 hour (3600)
	w.Header().Set("Cache-Control", "public, max-age=3600, s-maxage=3600")
	w.Header().Set("Content-Type", "image/svg+xml;utf-8")
	w.Write([]byte(generatedBadge))
}