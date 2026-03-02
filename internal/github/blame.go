package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/drpaneas/prview/internal/model"
)

// GraphQL blame query - the REST API doesn't support blame directly.
const blameQuery = `query($owner: String!, $repo: String!, $ref: String!, $path: String!) {
  repository(owner: $owner, name: $repo) {
    object(expression: $ref) {
      ... on Commit {
        blame(path: $path) {
          ranges {
            startingLine
            endingLine
            commit {
              author {
                name
                email
                date
                user {
                  login
                }
              }
            }
          }
        }
      }
    }
  }
}`

type graphQLRequest struct {
	Query     string            `json:"query"`
	Variables map[string]string `json:"variables"`
}

type blameRange struct {
	StartingLine int `json:"startingLine"`
	EndingLine   int `json:"endingLine"`
	Commit       struct {
		Author struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
			User  *struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"author"`
	} `json:"commit"`
}

type graphQLResponse struct {
	Data struct {
		Repository struct {
			Object struct {
				Blame struct {
					Ranges []blameRange `json:"ranges"`
				} `json:"blame"`
			} `json:"object"`
		} `json:"repository"`
	} `json:"data"`
}

func (c *Client) fetchBlames(ctx context.Context, input model.PRInput, baseSHA string, diffs []model.FileDiff) ([]model.BlameResult, error) {
	var results []model.BlameResult

	for _, d := range diffs {
		if d.Status == "added" || d.IsBinary {
			continue
		}

		blame, err := c.fetchBlame(ctx, input, baseSHA, basePathForDiff(d))
		if err != nil {
			continue
		}
		results = append(results, blame)
	}

	return results, nil
}

func (c *Client) fetchBlame(ctx context.Context, input model.PRInput, ref, path string) (model.BlameResult, error) {
	reqBody := graphQLRequest{
		Query: blameQuery,
		Variables: map[string]string{
			"owner": input.Owner,
			"repo":  input.Repo,
			"ref":   ref,
			"path":  path,
		},
	}

	httpClient := c.gh.Client()
	if httpClient == nil {
		return model.BlameResult{}, fmt.Errorf("no HTTP client available")
	}

	body, err := jsonReader(reqBody)
	if err != nil {
		return model.BlameResult{}, fmt.Errorf("marshaling blame request for %s: %w", path, err)
	}

	resp, err := httpClient.Post("https://api.github.com/graphql", "application/json", body)
	if err != nil {
		return model.BlameResult{}, fmt.Errorf("blame request for %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return model.BlameResult{}, fmt.Errorf("blame request for %s: status %d", path, resp.StatusCode)
	}

	var gqlResp graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return model.BlameResult{}, fmt.Errorf("decoding blame for %s: %w", path, err)
	}

	var lines []model.BlameLine
	for _, r := range gqlResp.Data.Repository.Object.Blame.Ranges {
		login := ""
		if r.Commit.Author.User != nil {
			login = r.Commit.Author.User.Login
		}
		for line := r.StartingLine; line <= r.EndingLine; line++ {
			lines = append(lines, model.BlameLine{
				Author: r.Commit.Author.Name,
				Email:  r.Commit.Author.Email,
				Login:  login,
				Line:   line,
				Date:   r.Commit.Author.Date,
			})
		}
	}

	return model.BlameResult{Path: path, Lines: lines}, nil
}

func jsonReader(v any) (io.Reader, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshaling JSON: %w", err)
	}
	return bytes.NewReader(data), nil
}
