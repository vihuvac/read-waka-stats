// Package githubx talks to GitHub REST, GraphQL, and the contributions API.
package githubx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vihuvac/read-waka-stats/internal/httpx"
	"github.com/vihuvac/read-waka-stats/internal/logging"
)

// User is the authenticated (or named) GitHub user.
type User struct {
	Login             string
	NodeID            string
	Email             string
	DiskUsage         int64
	Hireable          bool
	PublicRepos       int
	OwnedPrivateRepos int
}

// Repository is a GitHub repository used for language and commit stats.
type Repository struct {
	Name            string
	Owner           string
	IsPrivate       bool
	IsFork          bool
	PrimaryLanguage string
}

// YearContrib is yearly contribution totals from github-contributions.
type YearContrib struct {
	Year  int `json:"year"`
	Total int `json:"total"`
}

// UnmarshalJSON accepts year as either a JSON number or a string (API drift).
func (y *YearContrib) UnmarshalJSON(data []byte) error {
	var raw struct {
		Year  json.RawMessage `json:"year"`
		Total int             `json:"total"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	y.Total = raw.Total
	if len(raw.Year) == 0 || string(raw.Year) == "null" {
		return nil
	}
	var asInt int
	if err := json.Unmarshal(raw.Year, &asInt); err == nil {
		y.Year = asInt
		return nil
	}
	var asStr string
	if err := json.Unmarshal(raw.Year, &asStr); err != nil {
		return fmt.Errorf("year: %w", err)
	}
	n, err := strconv.Atoi(asStr)
	if err != nil {
		return fmt.Errorf("year: %w", err)
	}
	y.Year = n
	return nil
}

// Client accesses GitHub APIs.
type Client struct {
	HTTP    *httpx.Client
	Token   string
	Log     *logging.Logger
	APIBase string // defaults to https://api.github.com
}

func (c *Client) apiBase() string {
	if c.APIBase != "" {
		return strings.TrimRight(c.APIBase, "/")
	}
	return "https://api.github.com"
}

func (c *Client) authHeaders() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + c.Token,
		"Accept":        "application/vnd.github+json",
		"User-Agent":    "read-waka-stats",
	}
}

// FetchUser loads the authenticated user, or a named user when login is set.
func (c *Client) FetchUser(ctx context.Context, login string) (User, error) {
	path := c.apiBase() + "/user"
	if login != "" {
		path = c.apiBase() + "/users/" + login
	}
	body, status, err := c.HTTP.GetJSON(ctx, path, c.authHeaders())
	if err != nil {
		return User{}, err
	}
	if status != 200 {
		return User{}, fmt.Errorf("GitHub user API HTTP %d: %s", status, string(body))
	}
	var raw struct {
		Login             string `json:"login"`
		NodeID            string `json:"node_id"`
		Email             string `json:"email"`
		DiskUsage         *int64 `json:"disk_usage"`
		Hireable          *bool  `json:"hireable"`
		PublicRepos       int    `json:"public_repos"`
		OwnedPrivateRepos *int   `json:"owned_private_repos"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return User{}, err
	}
	u := User{
		Login:       raw.Login,
		NodeID:      raw.NodeID,
		Email:       raw.Email,
		DiskUsage:   -1,
		PublicRepos: raw.PublicRepos,
	}
	if raw.DiskUsage != nil {
		u.DiskUsage = *raw.DiskUsage * 1024 // GitHub reports kilobytes
	}
	if raw.Hireable != nil {
		u.Hireable = *raw.Hireable
	}
	if raw.OwnedPrivateRepos != nil {
		u.OwnedPrivateRepos = *raw.OwnedPrivateRepos
	}
	return u, nil
}

// FetchProfileViews returns weekly view count for owner/repo.
func (c *Client) FetchProfileViews(ctx context.Context, ownerRepo string) (int, error) {
	url := fmt.Sprintf("%s/repos/%s/traffic/views?per=week", c.apiBase(), ownerRepo)
	body, status, err := c.HTTP.GetJSON(ctx, url, c.authHeaders())
	if err != nil {
		return 0, err
	}
	if status != 200 {
		return 0, fmt.Errorf("traffic views HTTP %d", status)
	}
	var raw struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, err
	}
	return raw.Count, nil
}

// FetchContributions loads yearly contribution totals.
func (c *Client) FetchContributions(ctx context.Context, login string) ([]YearContrib, error) {
	url := "https://github-contributions.vercel.app/api/v1/" + login
	body, status, err := c.HTTP.GetJSON(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("contributions HTTP %d", status)
	}
	var raw struct {
		Years []YearContrib `json:"years"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw.Years, nil
}

const reposOwnedQuery = `
query($login: String!, $after: String) {
  user(login: $login) {
    repositories(first: 100, after: $after, orderBy: {field: CREATED_AT, direction: DESC}, affiliations: [OWNER, COLLABORATOR], isFork: false) {
      nodes {
        name
        isPrivate
        owner { login }
        primaryLanguage { name }
      }
      pageInfo { endCursor hasNextPage }
    }
  }
}`

const reposContributedQuery = `
query($login: String!, $after: String) {
  user(login: $login) {
    repositoriesContributedTo(first: 100, after: $after, orderBy: {field: CREATED_AT, direction: DESC}, includeUserRepositories: true) {
      nodes {
        name
        isPrivate
        isFork
        owner { login }
        primaryLanguage { name }
      }
      pageInfo { endCursor hasNextPage }
    }
  }
}`

const branchesQuery = `
query($owner: String!, $name: String!, $after: String) {
  repository(owner: $owner, name: $name) {
    refs(refPrefix: "refs/heads/", first: 100, after: $after, orderBy: {direction: DESC, field: TAG_COMMIT_DATE}) {
      nodes { name }
      pageInfo { endCursor hasNextPage }
    }
  }
}`

const commitsQuery = `
query($owner: String!, $name: String!, $branch: String!, $id: ID!, $after: String) {
  repository(owner: $owner, name: $name) {
    ref(qualifiedName: $branch) {
      target {
        ... on Commit {
          history(author: {id: $id}, first: 100, after: $after) {
            nodes {
              additions
              deletions
              committedDate
              oid
            }
            pageInfo { endCursor hasNextPage }
          }
        }
      }
    }
  }
}`

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Client) graphql(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase()+"/graphql", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "read-waka-stats")
		resp, err := c.HTTP.Do(ctx, req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		body, err := readClose(resp)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 || resp.StatusCode == 429 {
			lastErr = fmt.Errorf("graphql HTTP %d", resp.StatusCode)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("graphql HTTP %d: %s", resp.StatusCode, truncate(body, 400))
		}
		var parsed gqlResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}
		if len(parsed.Errors) > 0 {
			return nil, fmt.Errorf("graphql: %s", parsed.Errors[0].Message)
		}
		return parsed.Data, nil
	}
	return nil, lastErr
}

func readClose(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(resp.Body)
	return buf.Bytes(), err
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) <= n {
		return s
	}
	return s[:n]
}

type pageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

type langNode struct {
	Name string `json:"name"`
}

type ownerNode struct {
	Login string `json:"login"`
}

type repoNode struct {
	Name            string    `json:"name"`
	IsPrivate       bool      `json:"isPrivate"`
	IsFork          bool      `json:"isFork"`
	Owner           ownerNode `json:"owner"`
	PrimaryLanguage *langNode `json:"primaryLanguage"`
}

func toRepo(n repoNode) Repository {
	r := Repository{
		Name:      n.Name,
		Owner:     n.Owner.Login,
		IsPrivate: n.IsPrivate,
		IsFork:    n.IsFork,
	}
	if n.PrimaryLanguage != nil {
		r.PrimaryLanguage = n.PrimaryLanguage.Name
	}
	return r
}

// FetchRepositories loads owned/collaborated repos plus contributed-to repos.
func (c *Client) FetchRepositories(ctx context.Context, login string, maxRepos int) ([]Repository, error) {
	owned, err := c.paginateRepos(ctx, login, true, maxRepos)
	if err != nil {
		return nil, err
	}
	if maxRepos > 0 && len(owned) >= maxRepos {
		return owned[:maxRepos], nil
	}
	remaining := 0
	if maxRepos > 0 {
		remaining = maxRepos - len(owned)
	}
	contrib, err := c.paginateRepos(ctx, login, false, remaining)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, r := range owned {
		seen[r.Name] = struct{}{}
	}
	out := owned
	for _, r := range contrib {
		if r.IsFork {
			continue
		}
		if _, ok := seen[r.Name]; ok {
			continue
		}
		out = append(out, r)
		if maxRepos > 0 && len(out) >= maxRepos {
			break
		}
	}
	if maxRepos > 0 && len(out) > maxRepos {
		out = out[:maxRepos]
	}
	return out, nil
}

func (c *Client) paginateRepos(ctx context.Context, login string, owned bool, maxNodes int) ([]Repository, error) {
	query := reposOwnedQuery
	if !owned {
		query = reposContributedQuery
	}
	var out []Repository
	var after *string
	for {
		vars := map[string]any{"login": login, "after": after}
		data, err := c.graphql(ctx, query, vars)
		if err != nil {
			return nil, err
		}
		var parsed struct {
			User struct {
				Repositories struct {
					Nodes    []repoNode `json:"nodes"`
					PageInfo pageInfo   `json:"pageInfo"`
				} `json:"repositories"`
				RepositoriesContributedTo struct {
					Nodes    []repoNode `json:"nodes"`
					PageInfo pageInfo   `json:"pageInfo"`
				} `json:"repositoriesContributedTo"`
			} `json:"user"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			return nil, err
		}
		conn := parsed.User.Repositories
		if !owned {
			conn = parsed.User.RepositoriesContributedTo
		}
		for _, n := range conn.Nodes {
			out = append(out, toRepo(n))
			if maxNodes > 0 && len(out) >= maxNodes {
				return out, nil
			}
		}
		if !conn.PageInfo.HasNextPage {
			return out, nil
		}
		cur := conn.PageInfo.EndCursor
		after = &cur
	}
}

// Branch is a repository branch name.
type Branch struct {
	Name string `json:"name"`
}

// FetchBranches lists branch names for a repository.
func (c *Client) FetchBranches(ctx context.Context, owner, name string) ([]Branch, error) {
	var out []Branch
	var after *string
	for {
		data, err := c.graphql(ctx, branchesQuery, map[string]any{"owner": owner, "name": name, "after": after})
		if err != nil {
			return nil, err
		}
		var parsed struct {
			Repository *struct {
				Refs struct {
					Nodes    []Branch `json:"nodes"`
					PageInfo pageInfo `json:"pageInfo"`
				} `json:"refs"`
			} `json:"repository"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			return nil, err
		}
		if parsed.Repository == nil {
			return out, nil
		}
		out = append(out, parsed.Repository.Refs.Nodes...)
		if !parsed.Repository.Refs.PageInfo.HasNextPage {
			return out, nil
		}
		cur := parsed.Repository.Refs.PageInfo.EndCursor
		after = &cur
	}
}

// Commit is a single commit in history.
type Commit struct {
	Additions     int    `json:"additions"`
	Deletions     int    `json:"deletions"`
	CommittedDate string `json:"committedDate"`
	OID           string `json:"oid"`
}

// FetchCommits lists commits by the given author on a branch.
func (c *Client) FetchCommits(ctx context.Context, owner, name, branch, authorID string) ([]Commit, error) {
	ref := branch
	if !strings.HasPrefix(ref, "refs/heads/") {
		ref = "refs/heads/" + branch
	}
	var out []Commit
	var after *string
	for {
		data, err := c.graphql(ctx, commitsQuery, map[string]any{
			"owner": owner, "name": name, "branch": ref, "id": authorID, "after": after,
		})
		if err != nil {
			return nil, err
		}
		var parsed struct {
			Repository *struct {
				Ref *struct {
					Target struct {
						History struct {
							Nodes    []Commit `json:"nodes"`
							PageInfo pageInfo `json:"pageInfo"`
						} `json:"history"`
					} `json:"target"`
				} `json:"ref"`
			} `json:"repository"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			return nil, err
		}
		if parsed.Repository == nil || parsed.Repository.Ref == nil {
			return out, nil
		}
		hist := parsed.Repository.Ref.Target.History
		out = append(out, hist.Nodes...)
		if !hist.PageInfo.HasNextPage {
			return out, nil
		}
		cur := hist.PageInfo.EndCursor
		after = &cur
	}
}

// LinguistColors maps language name to hex color.
type LinguistColors map[string]string

// FetchLinguistColors downloads GitHub linguist language colors.
func (c *Client) FetchLinguistColors(ctx context.Context) (LinguistColors, error) {
	url := "https://cdn.jsdelivr.net/gh/github/linguist@master/lib/linguist/languages.yml"
	body, status, err := c.HTTP.GetJSON(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("linguist HTTP %d", status)
	}
	return parseLinguistColors(string(body)), nil
}

func parseLinguistColors(yml string) LinguistColors {
	out := LinguistColors{}
	lang := ""
	for _, line := range strings.Split(yml, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.HasSuffix(line, ":") {
			lang = strings.TrimSuffix(line, ":")
			continue
		}
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "color:") && lang != "" {
			color := strings.TrimSpace(strings.TrimPrefix(trim, "color:"))
			color = strings.Trim(color, `"'`)
			out[lang] = color
		}
	}
	return out
}

// ParseLinguistColors is exported for tests.
func ParseLinguistColors(yml string) LinguistColors {
	return parseLinguistColors(yml)
}
