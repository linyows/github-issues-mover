package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/repr"
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
	IsImport        bool
	SkipLabels      bool
	SkipMilestones  bool
	SkipAvatars     bool
	Sync            bool
	Debug           bool
	DryRun          bool
}

type IssueAndCommentsRequest struct {
	Issue    *github.IssueRequest
	Comments []*github.IssueComment
}

func New(ctx context.Context) (*Transfer, error) {
	var (
		srcRepo        = flag.String("src", "", "source repository: foo/bar")
		dstRepo        = flag.String("dst", "", "destination repository: foo/bar")
		srcEndpoint    = flag.String("src-endpoint", defaultEndpoint, "source api endpoint")
		dstEndpoint    = flag.String("dst-endpoint", defaultEndpoint, "destination api endpoint")
		isImport       = flag.Bool("import", true, "use issue import api")
		skipLabels     = flag.Bool("skip-labels", false, "skip create labels")
		skipMilestones = flag.Bool("skip-milestones", false, "skip create milestones")
		skipAvatars    = flag.Bool("skip-avatars", false, "skip linking avatars")
		sync           = flag.Bool("sync", false, "create issues synchronously (recommended)")
		debug          = flag.Bool("debug", false, "debugging output")
		dryrun         = flag.Bool("dry-run", false, "make no changes")
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
		srcClient = githubv4.NewEnterpriseClient(*srcEndpoint+"/api/graphql", srcTc)
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
		IsImport:        *isImport,
		SkipLabels:      *skipLabels,
		SkipMilestones:  *skipMilestones,
		SkipAvatars:     *skipAvatars,
		Sync:            *sync,
		Debug:           *debug,
		DryRun:          *dryrun,
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
	if !t.SkipLabels {
		if err := t.FetchLabels(ctx); err != nil {
			return err
		}
	}

	if !t.SkipMilestones {
		if err := t.FetchMilestones(ctx); err != nil {
			return err
		}
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

	for i := range pulls {
		pulls[i].Title = "[PR] " + pulls[i].Title
	}
	t.Pulls = pulls

	return nil
}

func (t *Transfer) Do(ctx context.Context) error {
	if !t.SkipLabels {
		if err := t.DoLabels(ctx); err != nil {
			return fmt.Errorf("label create error: %w", err)
		}
	}

	if !t.SkipMilestones {
		if err := t.DoMilestones(ctx); err != nil {
			return fmt.Errorf("milestone create error: %w", err)
		}
	}

	if err := t.DoIssues(ctx); err != nil {
		return fmt.Errorf("issue create error: %w", err)
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
	issuesAndPulls := make([]Issue, 0, len(t.Issues)+len(t.Pulls))
	issuesAndPulls = append(issuesAndPulls, t.Issues...)
	issuesAndPulls = append(issuesAndPulls, t.Pulls...)

	sort.Slice(issuesAndPulls, func(i, j int) bool {
		return issuesAndPulls[i].Number < issuesAndPulls[j].Number
	})

	if t.Debug {
		fmt.Printf("Issues and pulls:\n%s\n\n", repr.String(issuesAndPulls, repr.Indent("  ")))
	}

	issueNumber := 1

	for _, v := range issuesAndPulls {
		var err error
		// Create dummy issues between issue numbers to maintain alignment.
		// Note: in asynchronous mode, it may not be necessary to do this, since
		// issues could be created out of order.
		for ; issueNumber < v.Number; issueNumber++ {
			fmt.Printf("Creating issue #%d - Dummy for alignment\n", issueNumber)
			if t.IsImport {
				err = t.importIssue(ctx, t.buildImportIssueRequest(ctx, nil))
			} else {
				err = t.createIssueWithComments(ctx, t.buildCreateIssueRequest(ctx, nil))
			}
			if err != nil {
				return err
			}
		}
		// Create an issue from a real issue or a pull request
		fmt.Printf("Creating issue #%d - %s\n", v.Number, v.Title)
		if t.IsImport {
			err = t.importIssue(ctx, t.buildImportIssueRequest(ctx, &v))
		} else {
			err = t.createIssueWithComments(ctx, t.buildCreateIssueRequest(ctx, &v))
		}
		if err != nil {
			return err
		}
		issueNumber++
	}
	return nil
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
	body := t.bodyPrefix(v.Author.AvatarURL, v.Author.Login, nil) + t.replaceBody(v.Body)
	var comments []*IssueImportComment
	for _, vv := range v.Comments.Nodes {
		comments = append(comments, &IssueImportComment{
			CreatedAt: &vv.CreatedAt,
			Body:      t.bodyPrefix(vv.Author.AvatarURL, vv.Author.Login, nil) + t.replaceBody(vv.Body),
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
	if t.DryRun {
		return nil
	}
	got, _, err := ImportIssue(t.DST.Client, ctx, t.DST.Owner, t.DST.Name, input)
	if err != nil {
		return err
	}

	if !t.Sync {
		// Note: if following up on asynchronous issues is reimplemented, this should create a
		// slice of URLs or IssueImportRequests, instead of import IDs.
		// number, _ := strconv.Atoi(path.Base(*got.URL))
		// fmt.Printf("requested issue import: importID %d - %s\n", number, input.IssueImport.Title)
		// t.ImportRequested = append(t.ImportRequested, number)
		return nil
	}

	// To ensure issues are created in order, create them synchronously as recommended
	// by the Import API docs. This is slow, but safe. We poll the endpoint with a
	// multiplicative delay, but we reset this delay after every issue.

	const maxRetries = 10
	var delayMs = 1000
	var delayScale = 1.6

	for i := 0; ; i++ {
		fmt.Printf("| Import status: %s\n", *got.Status) // would be nicer as a horizontal "..."
		switch *got.Status {
		case "pending":
		case "imported":
			return nil
		default:
			return fmt.Errorf("issue import failed: %s", repr.String(got))
		}
		if i >= maxRetries {
			break
		}
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		delayMs = int(float64(delayMs) * delayScale)
		got, _, err = GetImportIssueResponse(t.DST.Client, ctx, *got.URL)
		if err != nil {
			return err
		}
	}
	return fmt.Errorf("max retries exceeded: %s", repr.String(got))
}

func (t *Transfer) buildCreateDummyIssueRequest(tt *time.Time) *IssueAndCommentsRequest {
	st := "closed"
	ti := "Dummy"
	bo := "This is a dummy to align the issue numbers for move."
	return &IssueAndCommentsRequest{
		Issue: &github.IssueRequest{
			Title: &ti,
			Body:  &bo,
			State: &st,
		},
		Comments: nil,
	}
}

func (t *Transfer) buildCreateIssueRequest(ctx context.Context, v *Issue) *IssueAndCommentsRequest {
	if v == nil {
		now := time.Now()
		return t.buildCreateDummyIssueRequest(&now)
	}

	state := strings.ToLower(v.State)
	labels := []string{}
	for _, vv := range v.Labels.Nodes {
		labels = append(labels, vv.Name)
	}
	body := t.bodyPrefix(v.Author.AvatarURL, v.Author.Login, &v.CreatedAt) + t.replaceBody(v.Body)
	var comments []*github.IssueComment
	for _, vv := range v.Comments.Nodes {
		cBody := t.bodyPrefix(vv.Author.AvatarURL, vv.Author.Login, &vv.CreatedAt) + t.replaceBody(vv.Body)
		comments = append(comments, &github.IssueComment{
			CreatedAt: &vv.CreatedAt,
			Body:      &cBody,
		})
	}

	input := &IssueAndCommentsRequest{
		Issue: &github.IssueRequest{
			Title:  &v.Title,
			Body:   &body,
			State:  &state,
			Labels: &labels,
		},
		Comments: comments,
	}

	var assigneeName string
	if v.Assignees.TotalCount > 0 {
		assigneeName = t.replaceUser(v.Assignees.Nodes[0].Login)
		if t.existUser(ctx, assigneeName) {
			input.Issue.Assignee = &assigneeName
		}
	}
	if v.Milestone.Number > 0 {
		input.Issue.Milestone = &v.Milestone.Number
	}

	return input
}

func (t *Transfer) createIssueWithComments(ctx context.Context, input *IssueAndCommentsRequest) error {
	issue, _, err := t.DST.Client.Issues.Create(ctx, t.DST.Owner, t.DST.Name, input.Issue)
	if err != nil {
		fmt.Printf("%#v\n", input.Issue)
		return err
	}
	fmt.Printf("created issue: #%d - %s\n", *issue.Number, *issue.Title)

	for _, v := range input.Comments {
		_, _, err := t.DST.Client.Issues.CreateComment(ctx, t.DST.Owner, t.DST.Name, *issue.Number, v)
		if err != nil {
			switch err := err.(type) {
			case *github.ErrorResponse:
				return err
			default:
				fmt.Printf("comment error: %s\n", err.Error())
				_, _, err2 := t.DST.Client.Issues.CreateComment(ctx, t.DST.Owner, t.DST.Name, *issue.Number, v)
				if err2 != nil {
					fmt.Printf("comment retry error: %s\n", err2.Error())
				}
			}
		}
	}

	if *input.Issue.State == "closed" {
		_, _, err = t.DST.Client.Issues.Edit(ctx, t.DST.Owner, t.DST.Name, *issue.Number, &github.IssueRequest{State: input.Issue.State})
		if err != nil {
			fmt.Printf("%#v\n", input.Issue)
			return err
		}
		fmt.Printf("closed issue: #%d\n", *issue.Number)
	}

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

func (t *Transfer) ShowAssignees(ctx context.Context) error {
	if err := t.Fetch(ctx); err != nil {
		return err
	}
	for _, v := range t.Issues {
		var assigneeName string
		if v.Assignees.TotalCount > 0 {
			assigneeName = t.replaceUser(v.Assignees.Nodes[0].Login)
			if t.existUser(ctx, assigneeName) {
				fmt.Printf("%d %s\n", v.Number, assigneeName)
			}
		}
	}
	for _, v := range t.Pulls {
		var assigneeName string
		if v.Assignees.TotalCount > 0 {
			assigneeName = t.replaceUser(v.Assignees.Nodes[0].Login)
			if t.existUser(ctx, assigneeName) {
				fmt.Printf("%d %s\n", v.Number, assigneeName)
			}
		}
	}
	return nil
}

func (t *Transfer) bodyPrefix(avatarURL string, login string, tm *time.Time) string {
	var avatarStr string
	if !t.SkipAvatars {
		avatarStr = fmt.Sprintf("<img src=\"%s\" width=\"25\"> ", avatarURL)
	}

	if tm == nil {
		return fmt.Sprintf("%s<b>%s</b> commented:\n\n", avatarStr, login)
	}
	return fmt.Sprintf("%s<b>%s</b> commented (%s):\n\n",
		avatarStr, login, tm.Format(time.RFC822))
}
