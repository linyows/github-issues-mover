package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/go-github/v32/github"
)

const (
	mediaTypeIssueImportAPI = "application/vnd.github.golden-comet-preview+json"
)

type IssueImportRequest struct {
	IssueImport IssueImport           `json:"issue"`
	Comments    []*IssueImportComment `json:"comments,omitempty"`
}

type IssueImport struct {
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Assignee  *string    `json:"assignee,omitempty"`
	Milestone *int       `json:"milestone,omitempty"`
	Closed    *bool      `json:"closed,omitempty"`
	Labels    []string   `json:"labels,omitempty"`
}

type IssueImportComment struct {
	CreatedAt *time.Time `json:"created_at,omitempty"`
	Body      string     `json:"body"`
}

type IssueImportResponse struct {
	ID               *int                `json:"id,omitempty"`
	Status           *string             `json:"status,omitempty"`
	URL              *string             `json:"url,omitempty"`
	ImportIssuesURL  *string             `json:"import_issues_url,omitempty"`
	RepositoryURL    *string             `json:"repository_url,omitempty"`
	CreatedAt        *time.Time          `json:"created_at,omitempty"`
	UpdatedAt        *time.Time          `json:"updated_at,omitempty"`
	Message          *string             `json:"message,omitempty"`
	DocumentationURL *string             `json:"documentation_url,omitempty"`
	Errors           []*IssueImportError `json:"errors,omitempty"`
}

type IssueImportError struct {
	Location *string `json:"location,omitempty"`
	Resource *string `json:"resource,omitempty"`
	Field    *string `json:"field,omitempty"`
	Value    *string `json:"value,omitempty"`
	Code     *string `json:"code,omitempty"`
}

func ImportIssue(client *github.Client, ctx context.Context, owner, repo string, issue *IssueImportRequest) (*IssueImportResponse, *github.Response, error) {
	u := fmt.Sprintf("repos/%v/%v/import/issues", owner, repo)
	req, err := client.NewRequest("POST", u, issue)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", mediaTypeIssueImportAPI)
	i := new(IssueImportResponse)
	resp, err := client.Do(ctx, req, i)
	if err != nil {
		aerr, ok := err.(*github.AcceptedError)
		if ok {
			decErr := json.Unmarshal(aerr.Raw, i)
			if decErr != nil {
				err = decErr
			}
			return i, resp, nil
		}
		return nil, resp, err
	}
	return i, resp, nil
}

func CheckImportIssueStatus(client *github.Client, ctx context.Context, owner, repo string, issueID int64) (*IssueImportResponse, *github.Response, error) {
	u := fmt.Sprintf("repos/%v/%v/import/issues/%v", owner, repo, issueID)
	req, err := client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", mediaTypeIssueImportAPI)
	i := new(IssueImportResponse)
	resp, err := client.Do(ctx, req, i)
	if err != nil {
		return nil, resp, err
	}
	return i, resp, nil
}
