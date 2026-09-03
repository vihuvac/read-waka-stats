package githubx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrStaleHead indicates createCommitOnBranch rejected the expected tip OID.
var ErrStaleHead = errors.New("stale branch head oid")

// FileAddition is a path and raw file contents to add or update on a branch.
type FileAddition struct {
	Path     string
	Contents []byte
}

// CreateCommitInput configures a verified createCommitOnBranch mutation.
type CreateCommitInput struct {
	Owner            string
	Repo             string
	BranchName       string
	Message          string
	ExpectedHeadOid  string
	FileAdditions    []FileAddition
}

// CreateCommitOnBranch creates a GitHub-signed commit via GraphQL.
// The commit author is the identity of the authenticated token.
func (c *Client) CreateCommitOnBranch(ctx context.Context, in CreateCommitInput) (string, error) {
	if in.Owner == "" || in.Repo == "" {
		return "", fmt.Errorf("owner and repo are required")
	}
	if in.BranchName == "" {
		return "", fmt.Errorf("branch name is required")
	}
	if in.ExpectedHeadOid == "" {
		return "", fmt.Errorf("expectedHeadOid is required")
	}
	if len(in.FileAdditions) == 0 {
		return "", fmt.Errorf("at least one file addition is required")
	}

	additions := make([]map[string]string, 0, len(in.FileAdditions))
	for _, f := range in.FileAdditions {
		if f.Path == "" {
			return "", fmt.Errorf("file path is required")
		}
		additions = append(additions, map[string]string{
			"path":     f.Path,
			"contents": base64.StdEncoding.EncodeToString(f.Contents),
		})
	}

	headline := strings.TrimSpace(in.Message)
	if headline == "" {
		headline = "Updated with Dev Metrics"
	}

	const mutation = `
mutation CreateCommitOnBranch($input: CreateCommitOnBranchInput!) {
  createCommitOnBranch(input: $input) {
    commit {
      oid
    }
  }
}`

	vars := map[string]any{
		"input": map[string]any{
			"branch": map[string]any{
				"repositoryNameWithOwner": in.Owner + "/" + in.Repo,
				"branchName":              in.BranchName,
			},
			"message": map[string]any{
				"headline": headline,
			},
			"fileChanges": map[string]any{
				"additions": additions,
			},
			"expectedHeadOid": in.ExpectedHeadOid,
		},
	}

	data, err := c.graphql(ctx, mutation, vars)
	if err != nil {
		if isStaleHeadMessage(err.Error()) {
			return "", fmt.Errorf("%w: %v", ErrStaleHead, err)
		}
		return "", err
	}

	var parsed struct {
		CreateCommitOnBranch struct {
			Commit struct {
				OID string `json:"oid"`
			} `json:"commit"`
		} `json:"createCommitOnBranch"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}
	oid := parsed.CreateCommitOnBranch.Commit.OID
	if oid == "" {
		return "", fmt.Errorf("createCommitOnBranch returned empty commit oid")
	}
	return oid, nil
}

func isStaleHeadMessage(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "expected head oid") ||
		strings.Contains(lower, "expectedheadoid") ||
		strings.Contains(lower, "expected the head") ||
		strings.Contains(lower, "expected oid") ||
		(strings.Contains(lower, "oid") && strings.Contains(lower, "does not match"))
}
