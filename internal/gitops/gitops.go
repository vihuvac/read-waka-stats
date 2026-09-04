// Package gitops clones a profile repository and publishes README/chart updates.
package gitops

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/vihuvac/read-waka-stats/internal/githubx"
	"github.com/vihuvac/read-waka-stats/internal/logging"
)

// CommitPublisher creates a verified commit on a branch.
type CommitPublisher interface {
	CreateCommitOnBranch(ctx context.Context, in githubx.CreateCommitInput) (string, error)
}

// Options configure clone and publish behavior.
type Options struct {
	Owner         string
	Repo          string
	Token         string
	WorkDir       string
	Branch        string
	DefaultBranch string
	CommitMessage string
	Log           *logging.Logger
	// URL overrides the default https://github.com/{owner}/{repo}.git clone URL (tests).
	URL string
}

// Repository is a cloned git worktree.
type Repository struct {
	opts Options
	repo *git.Repository
	wt   *git.Worktree
	auth *githttp.BasicAuth
}

// Clone clones owner/repo into WorkDir at the target branch.
func Clone(opts Options) (*Repository, error) {
	if opts.WorkDir == "" {
		opts.WorkDir = "repo"
	}
	_ = os.RemoveAll(opts.WorkDir)
	auth := &githttp.BasicAuth{Username: "x-access-token", Password: opts.Token}
	cloneURL := opts.URL
	if cloneURL == "" {
		cloneURL = fmt.Sprintf("https://github.com/%s/%s.git", opts.Owner, opts.Repo)
	}

	branch := opts.Branch
	if branch == "" {
		branch = opts.DefaultBranch
	}

	cloneOpts := &git.CloneOptions{
		URL:  cloneURL,
		Auth: auth,
	}
	if branch != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(branch)
		cloneOpts.SingleBranch = true
	}

	r, err := git.PlainClone(opts.WorkDir, false, cloneOpts)
	if err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}
	wt, err := r.Worktree()
	if err != nil {
		return nil, err
	}
	if branch == "" {
		head, err := r.Head()
		if err != nil {
			return nil, err
		}
		if head.Name().IsBranch() {
			branch = head.Name().Short()
		}
	}
	opts.Branch = branch
	return &Repository{opts: opts, repo: r, wt: wt, auth: auth}, nil
}

// Path returns a path inside the worktree.
func (r *Repository) Path(rel string) string {
	return filepath.Join(r.opts.WorkDir, rel)
}

// Root is the clone directory.
func (r *Repository) Root() string {
	return r.opts.WorkDir
}

// Branch returns the cloned branch name.
func (r *Repository) Branch() string {
	return r.opts.Branch
}

// Owner returns the repository owner.
func (r *Repository) Owner() string {
	return r.opts.Owner
}

// RepoName returns the repository name.
func (r *Repository) RepoName() string {
	return r.opts.Repo
}

// HeadOID returns the current HEAD commit OID.
func (r *Repository) HeadOID() (string, error) {
	head, err := r.repo.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

// Publish creates a verified commit for paths that differ from HEAD.
// Unchanged paths are skipped. If every path is unchanged, Publish is a no-op.
func (r *Repository) Publish(ctx context.Context, pub CommitPublisher, paths []string) error {
	if pub == nil {
		return fmt.Errorf("commit publisher is required")
	}
	additions, err := r.changedAdditions(paths)
	if err != nil {
		return err
	}
	if len(additions) == 0 {
		if r.opts.Log != nil {
			r.opts.Log.Success("No changes to commit")
		}
		return nil
	}

	oid, err := r.HeadOID()
	if err != nil {
		return fmt.Errorf("head oid: %w", err)
	}
	branch := r.opts.Branch
	if branch == "" {
		return fmt.Errorf("branch name is unresolved")
	}

	commitOID, err := pub.CreateCommitOnBranch(ctx, githubx.CreateCommitInput{
		Owner:           r.opts.Owner,
		Repo:            r.opts.Repo,
		BranchName:      branch,
		Message:         r.opts.CommitMessage,
		ExpectedHeadOid: oid,
		FileAdditions:   additions,
	})
	if err != nil {
		return err
	}
	if r.opts.Log != nil {
		r.opts.Log.Success("Published verified commit %s", commitOID)
	}
	return nil
}

// changedAdditions returns file additions for paths whose contents differ from HEAD.
func (r *Repository) changedAdditions(paths []string) ([]githubx.FileAddition, error) {
	seen := make(map[string]struct{}, len(paths))
	var out []githubx.FileAddition
	for _, rel := range paths {
		rel = filepath.ToSlash(rel)
		if rel == "" {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}

		data, err := os.ReadFile(r.Path(rel))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		same, err := r.matchesHead(rel, data)
		if err != nil {
			return nil, err
		}
		if same {
			continue
		}
		out = append(out, githubx.FileAddition{Path: rel, Contents: data})
	}
	return out, nil
}

// matchesHead reports whether data equals the file contents at HEAD for rel.
func (r *Repository) matchesHead(rel string, data []byte) (bool, error) {
	head, err := r.repo.Head()
	if err != nil {
		return false, err
	}
	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return false, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return false, err
	}
	file, err := tree.File(rel)
	if err != nil {
		if err == object.ErrFileNotFound {
			return false, nil
		}
		return false, err
	}
	blob, err := r.repo.BlobObject(file.Hash)
	if err != nil {
		return false, err
	}
	reader, err := blob.Reader()
	if err != nil {
		return false, err
	}
	defer reader.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return false, err
	}
	return bytes.Equal(buf.Bytes(), data), nil
}
