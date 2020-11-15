package main

import (
	"time"

	"github.com/shurcooL/githubv4"
)

type Label struct {
	Name        string
	Color       string
	Description string
}

type Milestone struct {
	Number      int
	Title       string
	Description string
	State       string
	Closed      bool
	DueOn       time.Time
}

type Issue struct {
	Title     string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
	ClosedAt  time.Time
	State     string
	Number    int
	Closed    bool
	Milestone struct {
		Number int
	}
	Author struct {
		Login     string
		AvatarURL string `graphql:"avatarUrl(size: 100)"`
	}
	Assignees struct {
		Nodes      []struct{ Login string }
		TotalCount githubv4.Int
	} `graphql:"assignees(first: 100, after: null)"`
	Labels struct {
		Nodes      []struct{ Name string }
		TotalCount githubv4.Int
	} `graphql:"labels(first: 100, after: null)"`
	Comments struct {
		Nodes []struct {
			Author struct {
				Login     string
				AvatarURL string `graphql:"avatarUrl(size: 100)"`
			}
			Body      string
			CreatedAt time.Time
		}
		TotalCount githubv4.Int
	} `graphql:"comments(first: 100, after: null)"`
}

type LabelsQuery struct {
	Repository struct {
		Labels struct {
			Nodes    []Label
			PageInfo struct {
				EndCursor   githubv4.String
				HasNextPage bool
			}
		} `graphql:"labels(first: 100, after: $cursor)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type MilestonesQuery struct {
	Repository struct {
		Milestones struct {
			Nodes    []Milestone
			PageInfo struct {
				EndCursor   githubv4.String
				HasNextPage bool
			}
		} `graphql:"milestones(first: 100, after: $cursor)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type IssuesQuery struct {
	Repository struct {
		Issues struct {
			Nodes    []Issue
			PageInfo struct {
				EndCursor   githubv4.String
				HasNextPage bool
			}
		} `graphql:"issues(first: 100, after: $cursor, orderBy: {field: CREATED_AT, direction: ASC})"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type PullReqeustsQuery struct {
	Repository struct {
		PullReqeusts struct {
			Nodes    []Issue
			PageInfo struct {
				EndCursor   githubv4.String
				HasNextPage bool
			}
		} `graphql:"pullRequests(first: 100, after: $cursor, orderBy: {field: CREATED_AT, direction: ASC})"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}
