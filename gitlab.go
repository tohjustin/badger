package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/tohjustin/badger/pkg/badge"
)

type GitlabFilteredResponse struct {
	Size int `json:"size"`
}

type GitlabProjectsResponse struct {
	ID                int           `json:"id"`
	Description       string        `json:"description"`
	Name              string        `json:"name"`
	NameWithNamespace string        `json:"name_with_namespace"`
	Path              string        `json:"path"`
	PathWithNamespace string        `json:"path_with_namespace"`
	CreatedAt         time.Time     `json:"created_at"`
	DefaultBranch     string        `json:"default_branch"`
	TagList           []interface{} `json:"tag_list"`
	SSHURLToRepo      string        `json:"ssh_url_to_repo"`
	HTTPURLToRepo     string        `json:"http_url_to_repo"`
	WebURL            string        `json:"web_url"`
	ReadmeURL         string        `json:"readme_url"`
	AvatarURL         string        `json:"avatar_url"`
	StarCount         int           `json:"star_count"`
	ForksCount        int           `json:"forks_count"`
	LastActivityAt    time.Time     `json:"last_activity_at"`
	Namespace         struct {
		ID       int         `json:"id"`
		Name     string      `json:"name"`
		Path     string      `json:"path"`
		Kind     string      `json:"kind"`
		FullPath string      `json:"full_path"`
		ParentID interface{} `json:"parent_id"`
	} `json:"namespace"`
}

func NewGitlabService() RepositoryService {
	return &gitlabService{}
}

type gitlabService struct{}

func (service *gitlabService) getForkCount(owner string, repo string) (int, error) {
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s%%2F%s", owner, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("NewRequest: ", err)
		return 0, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Do: ", err)
		return 0, err
	}
	defer resp.Body.Close()

	var project GitlabProjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		log.Println(err)
		return -1, err
	}

	return project.ForksCount, nil
}

func (service *gitlabService) getIssueCount(owner string, repo string, issueState string) (int, error) {
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s%%2F%s/issues", owner, repo)
	switch issueState {
	case "opened":
		url = fmt.Sprintf("%s?state=opened", url)
	case "closed":
		url = fmt.Sprintf("%s?state=closed", url)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("NewRequest: ", err)
		return 0, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Do: ", err)
		return 0, err
	}
	defer resp.Body.Close()

	xTotal := resp.Header.Get("X-Total")
	issueCount, err := strconv.Atoi(xTotal)
	if err != nil {
		log.Println(err)
		return -1, err
	}

	return issueCount, nil
}

func (service *gitlabService) getPullRequestCount(owner string, repo string, pullRequestState string) (int, error) {
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s%%2F%s/merge_requests", owner, repo)
	switch pullRequestState {
	case "opened":
		url = fmt.Sprintf("%s?state=opened", url)
	case "closed":
		url = fmt.Sprintf("%s?state=closed", url)
	case "locked":
		url = fmt.Sprintf("%s?state=locked", url)
	case "merged":
		url = fmt.Sprintf("%s?state=merged", url)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("NewRequest: ", err)
		return 0, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Do: ", err)
		return 0, err
	}
	defer resp.Body.Close()

	xTotal := resp.Header.Get("X-Total")
	issueCount, err := strconv.Atoi(xTotal)
	if err != nil {
		log.Println(err)
		return -1, err
	}

	return issueCount, nil
}

func (service *gitlabService) getStargazerCount(owner string, repo string) (int, error) {
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s%%2F%s", owner, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal("NewRequest: ", err)
		return 0, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Do: ", err)
		return 0, err
	}
	defer resp.Body.Close()

	var project GitlabProjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		log.Println(err)
		return -1, err
	}

	return project.StarCount, nil
}

func (service *gitlabService) Handler(w http.ResponseWriter, r *http.Request) {
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
		case "opened":
			subject = "opened issues"
		case "closed":
			subject = "closed issues"
		default:
			subject = "issues"
		}
		value, err = service.getIssueCount(owner, repo, state)
	case "merge-requests":
		state := r.URL.Query().Get("state")
		switch state {
		case "opened":
			subject = "opened MRs"
		case "closed":
			subject = "closed MRs"
		case "locked":
			subject = "locked MRs"
		case "merged":
			subject = "merged MRs"
		default:
			subject = "MRs"
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