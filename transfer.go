package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

const (
	defaultEndpoint = "https://api.github.com"
)

type SRC struct {
	Owner    string
	Name     string
	Endpoint string
	Client   *githubv4.Client
}

type DST struct {
	Owner    string
	Name     string
	Endpoint string
	Client   *github.Client
}

type Transfer struct {
	*SRC
	*DST
	Labels          []Label
	Milestones      []Milestone
	Issues          []Issue
	Pulls           []Issue
	ImportRequested []int
	Replace         *Map
}

func New(ctx context.Context) (*Transfer, error) {
	var (
		srcRepo     = flag.String("src", "", "source repository: foo/bar")
		dstRepo     = flag.String("dst", "", "destination repository: foo/bar")
		srcEndpoint = flag.String("src-endpoint", defaultEndpoint, "source api endpoint")
		dstEndpoint = flag.String("dst-endpoint", defaultEndpoint, "destination api endpoint")
	)
	flag.Parse()

	srcToken := os.Getenv("SRC_TOKEN")
	dstToken := os.Getenv("DST_TOKEN")

	srcTs := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: srcToken},
	)
	srcTc := oauth2.NewClient(ctx, srcTs)
	srcClient := githubv4.NewClient(srcTc)
	if defaultEndpoint != *srcEndpoint {
		srcClient = githubv4.NewEnterpriseClient(*srcEndpoint, srcTc)
	}

	dstTs := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: dstToken},
	)
	dstTc := oauth2.NewClient(ctx, dstTs)
	dstClient := github.NewClient(dstTc)
	if defaultEndpoint != *dstEndpoint {
		var e error
		dstClient, e = github.NewEnterpriseClient(*dstEndpoint, *dstEndpoint, dstTc)
		if e != nil {
			return nil, e
		}
	}

	s := strings.Split(*srcRepo, "/")
	d := strings.Split(*dstRepo, "/")

	replace, err := LoadReplacementMap()
	if err != nil {
		return nil, err
	}

	return &Transfer{
		SRC: &SRC{
			Owner:    s[0],
			Name:     s[1],
			Endpoint: *srcEndpoint,
			Client:   srcClient,
		},
		DST: &DST{
			Owner:    d[0],
			Name:     d[1],
			Endpoint: *dstEndpoint,
			Client:   dstClient,
		},
		Labels:          nil,
		Milestones:      nil,
		Issues:          nil,
		Pulls:           nil,
		ImportRequested: nil,
		Replace:         replace,
	}, nil
}

func (t *Transfer) Exec(ctx context.Context) error {
	if err := t.Fetch(ctx); err != nil {
		return err
	}
	if err := t.Do(ctx); err != nil {
		return err
	}
	//t.ImportIssueStatus(ctx)

	return nil
}

func (t *Transfer) Fetch(ctx context.Context) error {
	if err := t.FetchLabels(ctx); err != nil {
		return err
	}

	if err := t.FetchMilestones(ctx); err != nil {
		return err
	}

	if err := t.FetchIssues(ctx); err != nil {
		return err
	}

	if err := t.FetchPulls(ctx); err != nil {
		return err
	}

	return nil
}

func (t *Transfer) FetchLabels(ctx context.Context) error {
	var labels []Label
	var lq LabelsQuery
	lv := map[string]interface{}{
		"owner":  githubv4.String(t.SRC.Owner),
		"repo":   githubv4.String(t.SRC.Name),
		"cursor": (*githubv4.String)(nil),
	}
	for {
		err := t.SRC.Client.Query(ctx, &lq, lv)
		if err != nil {
			return err
		}
		labels = append(labels, lq.Repository.Labels.Nodes...)
		if !lq.Repository.Labels.PageInfo.HasNextPage {
			break
		}
		lv["cursor"] = githubv4.NewString(lq.Repository.Labels.PageInfo.EndCursor)
	}

	t.Labels = labels

	return nil
}

func (t *Transfer) FetchMilestones(ctx context.Context) error {
	var milestones []Milestone
	var mq MilestonesQuery
	mv := map[string]interface{}{
		"owner":  githubv4.String(t.SRC.Owner),
		"repo":   githubv4.String(t.SRC.Name),
		"cursor": (*githubv4.String)(nil),
	}
	for {
		err := t.SRC.Client.Query(ctx, &mq, mv)
		if err != nil {
			return err
		}
		milestones = append(milestones, mq.Repository.Milestones.Nodes...)
		if !mq.Repository.Milestones.PageInfo.HasNextPage {
			break
		}
		mv["cursor"] = githubv4.NewString(mq.Repository.Milestones.PageInfo.EndCursor)
	}

	t.Milestones = milestones

	return nil
}

func (t *Transfer) FetchIssues(ctx context.Context) error {
	var issues []Issue
	var iq IssuesQuery
	iv := map[string]interface{}{
		"owner":  githubv4.String(t.SRC.Owner),
		"repo":   githubv4.String(t.SRC.Name),
		"cursor": (*githubv4.String)(nil),
	}
	for {
		err := t.SRC.Client.Query(ctx, &iq, iv)
		if err != nil {
			return err
		}
		issues = append(issues, iq.Repository.Issues.Nodes...)
		if !iq.Repository.Issues.PageInfo.HasNextPage {
			break
		}
		iv["cursor"] = githubv4.NewString(iq.Repository.Issues.PageInfo.EndCursor)
	}

	t.Issues = issues

	return nil
}

func (t *Transfer) FetchPulls(ctx context.Context) error {
	var pulls []Issue
	var pq PullReqeustsQuery
	pv := map[string]interface{}{
		"owner":  githubv4.String(t.SRC.Owner),
		"repo":   githubv4.String(t.SRC.Name),
		"cursor": (*githubv4.String)(nil),
	}
	for {
		err := t.SRC.Client.Query(ctx, &pq, pv)
		if err != nil {
			return err
		}
		pulls = append(pulls, pq.Repository.PullReqeusts.Nodes...)
		if !pq.Repository.PullReqeusts.PageInfo.HasNextPage {
			break
		}
		pv["cursor"] = githubv4.NewString(pq.Repository.PullReqeusts.PageInfo.EndCursor)
	}

	t.Pulls = pulls

	return nil
}

func (t *Transfer) Do(ctx context.Context) error {
	if err := t.DoLabels(ctx); err != nil {
		fmt.Printf("label create error (%s): %#v\n",
			err.(*github.ErrorResponse).Response.Status,
			err.(*github.ErrorResponse).Message)
		return err
	}

	if err := t.DoMilestones(ctx); err != nil {
		fmt.Printf("milestone create error (%s): %#v\n",
			err.(*github.ErrorResponse).Response.Status,
			err.(*github.ErrorResponse).Message)
		return err
	}

	if err := t.DoIssues(ctx); err != nil {
		fmt.Printf("issue import request error (%s): %#v\n",
			err.(*github.ErrorResponse).Response.Status,
			err.(*github.ErrorResponse).Message)
		return err
	}

	return nil
}

func (t *Transfer) DoLabels(ctx context.Context) error {
	for _, v := range t.Labels {
		input := &github.Label{
			Name:        &v.Name,
			Color:       &v.Color,
			Description: &v.Description,
		}
		_, _, err := t.DST.Client.Issues.CreateLabel(ctx, t.DST.Owner, t.DST.Name, input)
		if err != nil {
			return err
		}
		fmt.Printf("created label: %s\n", v.Name)
	}
	return nil
}

func (t *Transfer) DoMilestones(ctx context.Context) error {
	for _, v := range t.Milestones {
		state := strings.ToLower(v.State)
		input := &github.Milestone{
			Title:       &v.Title,
			State:       &state,
			DueOn:       &v.DueOn,
			Description: &v.Description,
		}
		_, _, err := t.DST.Client.Issues.CreateMilestone(ctx, t.DST.Owner, t.DST.Name, input)
		if err != nil {
			return err
		}
		fmt.Printf("created milestone: %s\n", v.Title)
	}

	return nil
}

func (t *Transfer) DoIssues(ctx context.Context) error {
	lastNumber := t.Issues[len(t.Issues)-1].Number
	counter := 0

	for i := 1; i < lastNumber; i++ {
		v := t.Issues[counter]
		if i < v.Number {
			err := t.importIssue(ctx, t.buildImportIssueRequest(ctx, t.findPullRequest(i)))
			if err != nil {
				return err
			}
			continue
		}
		counter++

		err := t.importIssue(ctx, t.buildImportIssueRequest(ctx, &v))
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *Transfer) findPullRequest(n int) *Issue {
	counter := 0
	found := false
	for i, v := range t.Pulls {
		if v.Number == n {
			counter = i
			found = true
			break
		}
	}
	if found == false {
		return nil
	}

	return &t.Pulls[counter]
}

func (t *Transfer) buildImportDummyIssueRequest(tt *time.Time) *IssueImportRequest {
	closed := true
	return &IssueImportRequest{
		IssueImport: IssueImport{
			Title:     "Dummy",
			Body:      "This is a dummy to align the issue numbers for move.",
			CreatedAt: tt,
			ClosedAt:  tt,
			UpdatedAt: tt,
			Closed:    &closed,
			Labels:    nil,
		},
		Comments: nil,
	}
}

func (t *Transfer) buildImportIssueRequest(ctx context.Context, v *Issue) *IssueImportRequest {
	if v == nil {
		now := time.Now()
		return t.buildImportDummyIssueRequest(&now)
	}

	var labels []string
	for _, vv := range v.Labels.Nodes {
		labels = append(labels, vv.Name)
	}
	body := bodyPrefix(v.Author.AvatarURL, v.Author.Login) + t.replaceBody(v.Body)
	var comments []*IssueImportComment
	for _, vv := range v.Comments.Nodes {
		comments = append(comments, &IssueImportComment{
			CreatedAt: &vv.CreatedAt,
			Body:      bodyPrefix(vv.Author.AvatarURL, vv.Author.Login) + t.replaceBody(vv.Body),
		})
	}

	input := &IssueImportRequest{
		IssueImport: IssueImport{
			Title:     v.Title,
			Body:      body,
			CreatedAt: &v.CreatedAt,
			ClosedAt:  &v.ClosedAt,
			UpdatedAt: &v.UpdatedAt,
			Closed:    &v.Closed,
			Labels:    labels,
		},
		Comments: comments,
	}

	var assigneeName string
	if v.Assignees.TotalCount > 0 {
		assigneeName = t.replaceUser(v.Assignees.Nodes[0].Login)
		if t.existUser(ctx, assigneeName) {
			input.IssueImport.Assignee = &assigneeName
		}
	}
	if v.Milestone.Number > 0 {
		input.IssueImport.Milestone = &v.Milestone.Number
	}

	return input
}

func (t *Transfer) importIssue(ctx context.Context, input *IssueImportRequest) error {
	got, _, err := ImportIssue(t.DST.Client, ctx, t.DST.Owner, t.DST.Name, input)
	if err != nil {
		return err
	}

	number, _ := strconv.Atoi(path.Base(*got.URL))
	fmt.Printf("requested issue import: importID %d - %s\n", number, input.IssueImport.Title)
	//t.ImportRequested = append(t.ImportRequested, number)

	return nil
}

func (t *Transfer) existUser(ctx context.Context, name string) bool {
	_, _, err := t.DST.Client.Users.Get(ctx, name)
	if err != nil {
		return false
	}
	return true
}

func (t *Transfer) replaceUser(n string) string {
	if t.Replace != nil && len(t.Replace.User) > 0 {
		for _, v := range t.Replace.User {
			if v.Wrong == n {
				return v.Right
			}
		}
	}
	return n
}

func (t *Transfer) replaceBody(b string) string {
	if t.Replace != nil && len(t.Replace.Body) > 0 {
		for _, v := range t.Replace.Body {
			b = strings.ReplaceAll(b, v.Wrong, v.Right)
		}
	}
	return b
}

func (t *Transfer) ImportIssueStatus(ctx context.Context) {
	for _, v := range t.ImportRequested {
		got, _, err := CheckImportIssueStatus(t.DST.Client, ctx, t.DST.Owner, t.DST.Name, int64(v))
		if err != nil {
			fmt.Printf("issue import status error (%s): %#v\n",
				err.(*github.ErrorResponse).Response.Status,
				err.(*github.ErrorResponse).Message)
			continue
		}
		if *got.Status == "imported" {
			continue
		}
		fmt.Printf("%s: %s\n", *got.Status, *got.URL)
		for _, v := range got.Errors {
			fmt.Printf("%s [%s]: %s\n", *v.Field, *v.Code, *v.Value)
		}
	}
}

func bodyPrefix(avatarURL string, login string) string {
	return "<img src=\"" + avatarURL + "\" width=\"25\"> <b>" + login + "</b> commented:\n\n"
}
