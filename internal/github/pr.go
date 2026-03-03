package github

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/drpaneas/prview/internal/model"
	gh "github.com/google/go-github/v60/github"
)

func (c *Client) FetchPR(ctx context.Context, input model.PRInput) (*model.PRData, error) {
	pr, _, err := c.gh.PullRequests.Get(ctx, input.Owner, input.Repo, input.Number)
	if err != nil {
		return nil, fmt.Errorf("fetching PR: %w", err)
	}

	meta := model.PRMetadata{
		Title:      pr.GetTitle(),
		Author:     pr.GetUser().GetLogin(),
		State:      pr.GetState(),
		CreatedAt:  pr.GetCreatedAt().Time,
		HeadBranch: pr.GetHead().GetRef(),
		BaseBranch: pr.GetBase().GetRef(),
		IsDraft:    pr.GetDraft(),
		BaseSHA:    pr.GetBase().GetSHA(),
		HeadSHA:    pr.GetHead().GetSHA(),
	}

	reviews, _, err := c.gh.PullRequests.ListReviews(ctx, input.Owner, input.Repo, input.Number, nil)
	if err == nil {
		meta.ReviewCount = len(reviews)
	}

	meta.CIStatus = c.fetchCIStatus(ctx, input, pr.GetHead().GetSHA())

	diffs, err := c.fetchDiffs(ctx, input)
	if err != nil {
		return nil, err
	}

	baseFiles, headFiles, err := c.fetchFileContents(ctx, input, meta.BaseSHA, meta.HeadSHA, diffs)
	if err != nil {
		return nil, err
	}

	blames, err := c.fetchBlames(ctx, input, meta.BaseSHA, diffs)
	if err != nil {
		blames = nil // non-fatal
	}

	authorProfile := c.fetchAuthorProfile(ctx, input, meta.Author, pr.GetAuthorAssociation())

	return &model.PRData{
		Meta:      meta,
		Diffs:     diffs,
		BaseFiles: baseFiles,
		HeadFiles: headFiles,
		Blames:    blames,
		Author:    authorProfile,
	}, nil
}

func (c *Client) fetchCIStatus(ctx context.Context, input model.PRInput, sha string) string {
	status, _, err := c.gh.Repositories.GetCombinedStatus(ctx, input.Owner, input.Repo, sha, nil)
	if err != nil {
		return "unknown"
	}
	return status.GetState()
}

func (c *Client) fetchDiffs(ctx context.Context, input model.PRInput) ([]model.FileDiff, error) {
	var allDiffs []model.FileDiff
	opts := &gh.ListOptions{PerPage: 100, Page: 1}

	for {
		files, resp, err := c.gh.PullRequests.ListFiles(ctx, input.Owner, input.Repo, input.Number, opts)
		if err != nil {
			return nil, fmt.Errorf("fetching PR files: %w", err)
		}

		for _, f := range files {
			allDiffs = append(allDiffs, model.FileDiff{
				Path:      f.GetFilename(),
				OldPath:   f.GetPreviousFilename(),
				Status:    f.GetStatus(),
				Additions: f.GetAdditions(),
				Deletions: f.GetDeletions(),
				Patch:     f.GetPatch(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allDiffs, nil
}

const maxFileFetches = 50

func (c *Client) fetchFileContents(ctx context.Context, input model.PRInput, baseSHA, headSHA string, diffs []model.FileDiff) ([]model.FileContent, []model.FileContent, error) {
	var baseFiles, headFiles []model.FileContent
	fetched := 0

	for _, d := range diffs {
		if d.IsBinary || fetched >= maxFileFetches {
			continue
		}
		if !strings.HasSuffix(d.Path, ".go") && d.Path != "go.mod" {
			continue
		}

		if d.Status != "added" {
			content, err := c.getFileAtRef(ctx, input, basePathForDiff(d), baseSHA)
			if err == nil {
				baseFiles = append(baseFiles, content)
				fetched++
			}
		}

		if d.Status != "removed" {
			content, err := c.getFileAtRef(ctx, input, d.Path, headSHA)
			if err == nil {
				headFiles = append(headFiles, content)
				fetched++
			}
		}
	}

	return baseFiles, headFiles, nil
}

func basePathForDiff(d model.FileDiff) string {
	if d.Status == "renamed" && d.OldPath != "" {
		return d.OldPath
	}
	return d.Path
}

func (c *Client) getFileAtRef(ctx context.Context, input model.PRInput, path, ref string) (model.FileContent, error) {
	opts := &gh.RepositoryContentGetOptions{Ref: ref}
	fc, _, _, err := c.gh.Repositories.GetContents(ctx, input.Owner, input.Repo, path, opts)
	if err != nil {
		return model.FileContent{}, err
	}
	if fc == nil {
		return model.FileContent{}, fmt.Errorf("no content for %s at %s", path, ref)
	}
	content, err := fc.GetContent()
	if err != nil {
		return model.FileContent{}, err
	}
	return model.FileContent{Path: path, Content: content, SHA: ref}, nil
}

func (c *Client) fetchAuthorProfile(ctx context.Context, input model.PRInput, author, association string) model.AuthorProfile {
	profile := model.AuthorProfile{Login: author}

	query := fmt.Sprintf("is:pr author:%s repo:%s/%s is:merged", author, input.Owner, input.Repo)
	result, _, err := c.gh.Search.Issues(ctx, query, &gh.SearchOptions{
		Sort:        "created",
		Order:       "asc",
		ListOptions: gh.ListOptions{PerPage: 1},
	})
	if err == nil && result.GetTotal() > 0 {
		profile.MergedPRs = result.GetTotal()
		if len(result.Issues) > 0 {
			profile.FirstContribDate = result.Issues[0].GetCreatedAt().Time
		}
	}

	if profile.MergedPRs > 0 {
		resultDesc, _, err := c.gh.Search.Issues(ctx, query, &gh.SearchOptions{
			Sort:        "created",
			Order:       "desc",
			ListOptions: gh.ListOptions{PerPage: 1},
		})
		if err == nil && len(resultDesc.Issues) > 0 {
			profile.LastContribDate = resultDesc.Issues[0].GetCreatedAt().Time
		}
	}

	switch association {
	case "OWNER", "MEMBER", "COLLABORATOR":
		profile.IsFirstTime = false
	default:
		profile.IsFirstTime = profile.MergedPRs == 0
	}

	profile.TopAreas = c.fetchAuthorTopAreas(ctx, input, author)

	return profile
}

func (c *Client) fetchAuthorTopAreas(ctx context.Context, input model.PRInput, author string) []string {
	query := fmt.Sprintf("is:pr author:%s repo:%s/%s is:merged", author, input.Owner, input.Repo)
	result, _, err := c.gh.Search.Issues(ctx, query, &gh.SearchOptions{
		Sort:        "created",
		Order:       "desc",
		ListOptions: gh.ListOptions{PerPage: 5},
	})
	if err != nil || len(result.Issues) == 0 {
		return nil
	}

	dirCounts := map[string]int{}
	for _, issue := range result.Issues {
		prNum := issue.GetNumber()
		files, _, err := c.gh.PullRequests.ListFiles(ctx, input.Owner, input.Repo, prNum, &gh.ListOptions{PerPage: 30})
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, f := range files {
			dir := filepath.Dir(f.GetFilename())
			if !seen[dir] {
				dirCounts[dir]++
				seen[dir] = true
			}
		}
	}

	type dirScore struct {
		dir   string
		count int
	}
	var sorted []dirScore
	for dir, count := range dirCounts {
		sorted = append(sorted, dirScore{dir, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	var top []string
	for i, ds := range sorted {
		if i >= 3 {
			break
		}
		top = append(top, ds.dir)
	}
	return top
}
