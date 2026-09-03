// Package gitops clones a profile repository and commits README/chart updates.
package gitops

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/vihuvac/read-waka-stats/internal/logging"
)

const singleCommitBranch = "latest_branch"

// Options configure clone and commit behavior.
type Options struct {
	Owner         string
	Repo          string
	Token         string
	WorkDir       string
	PullBranch    string
	PushBranch    string
	DefaultBranch string
	CommitSingle  bool
	CommitMessage string
	AuthorName    string
	AuthorEmail   string
	Log           *logging.Logger
}

// Repository is a cloned git worktree.
type Repository struct {
	opts Options
	repo *git.Repository
	wt   *git.Worktree
	auth *githttp.BasicAuth
}

// Clone clones owner/repo into WorkDir.
func Clone(opts Options) (*Repository, error) {
	if opts.WorkDir == "" {
		opts.WorkDir = "repo"
	}
	_ = os.RemoveAll(opts.WorkDir)
	auth := &githttp.BasicAuth{Username: "x-access-token", Password: opts.Token}
	url := fmt.Sprintf("https://github.com/%s/%s.git", opts.Owner, opts.Repo)

	cloneOpts := &git.CloneOptions{
		URL:  url,
		Auth: auth,
	}
	branch := opts.PushBranch
	if opts.CommitSingle {
		branch = opts.PullBranch
	}
	if branch == "" {
		branch = opts.DefaultBranch
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
	if opts.CommitSingle {
		head, err := r.Head()
		if err != nil {
			return nil, err
		}
		orphan := plumbing.NewHashReference(plumbing.NewBranchReferenceName(singleCommitBranch), head.Hash())
		if err := r.Storer.SetReference(orphan); err != nil {
			return nil, err
		}
		if err := wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(singleCommitBranch),
			Force:  true,
		}); err != nil {
			return nil, err
		}
	}
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

// Add stages a file relative to the worktree.
func (r *Repository) Add(rel string) error {
	_, err := r.wt.Add(rel)
	return err
}

// CommitAndPush creates a commit when the tree is dirty and pushes it.
func (r *Repository) CommitAndPush() error {
	status, err := r.wt.Status()
	if err != nil {
		return err
	}
	if status.IsClean() {
		if r.opts.Log != nil {
			r.opts.Log.Success("No changes to commit")
		}
		return nil
	}
	sig := &object.Signature{
		Name:  r.opts.AuthorName,
		Email: r.opts.AuthorEmail,
		When:  time.Now(),
	}
	_, err = r.wt.Commit(r.opts.CommitMessage, &git.CommitOptions{
		Author:    sig,
		Committer: sig,
	})
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	pushOpts := &git.PushOptions{Auth: r.auth}
	if r.opts.CommitSingle {
		target := r.opts.PushBranch
		if target == "" {
			target = r.opts.DefaultBranch
		}
		refSpec := fmt.Sprintf("+refs/heads/%s:refs/heads/%s", singleCommitBranch, target)
		pushOpts.RefSpecs = []gitconfig.RefSpec{gitconfig.RefSpec(refSpec)}
		pushOpts.Force = true
	}
	if err := r.repo.Push(pushOpts); err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("push: %w", err)
	}
	if r.opts.Log != nil {
		r.opts.Log.Success("Repository synchronized")
	}
	return nil
}
