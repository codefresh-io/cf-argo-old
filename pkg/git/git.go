package git

import (
	"context"
	"errors"
	"os"

	gg "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type (
	// Repository represents a git repository
	Repository interface {
		Add(pattern string) error
		Commit(msg string) error
		Push(ctx context.Context) error
	}

	// Provider represents a git provider
	Provider interface {
		CloneRepository(ctx context.Context, url, path string) (Repository, error)
		CreateRepository(ctx context.Context, owner, name, path string) (Repository, error)
	}

	// Options for a new git provider
	Options struct {
		Type string
		Auth Auth
		Host string
	}

	// Auth for git provider
	Auth struct {
		Username string
		Password string
	}

	repo struct {
		r *gg.Repository
	}
)

// Errors
var (
	ErrProviderNotSupported = errors.New("git provider not supported")
)

// New creates a new git provider
func New(opts *Options) (Provider, error) {
	switch opts.Type {
	case "github":
		return newGithub(opts)
	default:
		return nil, ErrProviderNotSupported
	}
}

func cloneRepository(ctx context.Context, opts *Options, url, path string) (Repository, error) {
	username := opts.Auth.Username
	if username == "" {
		username = "codefresh"
	}

	r, err := gg.PlainCloneContext(ctx, path, false, &gg.CloneOptions{
		Depth: 1,
		URL:   url,
		Auth: &http.BasicAuth{
			Username: username,
			Password: opts.Auth.Password,
		},
		Progress: os.Stdout,
	})
	if err != nil {
		return nil, err
	}

	return &repo{r}, nil
}

func (r *repo) Add(pattern string) error {
	w, err := r.r.Worktree()
	if err != nil {
		return err
	}

	return w.AddGlob(pattern)
}

func (r *repo) Commit(msg string) error {
	return nil
}

func (r *repo) Push(ctx context.Context) error {
	return nil
}
