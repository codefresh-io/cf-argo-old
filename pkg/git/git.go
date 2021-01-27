package git

import (
	"context"
	"errors"
	"os"

	"github.com/codefresh-io/cf-argo/pkg/log"
	"github.com/go-git/go-git/plumbing/transport"
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
		// CreateRepository creates the repository in the remote provider and returns a
		// clone url
		CreateRepository(ctx context.Context, owner, name string) (string, error)
		Clone(ctx context.Context, opts *CloneOptions) (Repository, error)
	}

	// Options for a new git provider
	Options struct {
		Type string
		Auth *Auth
		Host string
	}

	// Auth for git provider
	Auth struct {
		Username string
		Password string
	}

	CloneOptions struct {
		Url  string
		Path string
		Auth *Auth
	}

	repo struct {
		r *gg.Repository
	}
)

// Errors
var (
	ErrProviderNotSupported = errors.New("git provider not supported")
	ErrNilOpts              = errors.New("options cannot be nil")
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

func Clone(ctx context.Context, opts *CloneOptions) (Repository, error) {
	var auth transport.AuthMethod

	// use authentication
	if opts.Auth != nil {
		username := opts.Auth.Username
		if username == "" {
			username = "codefresh"
		}
		auth = &http.BasicAuth{
			Username: username,
			Password: opts.Auth.Password,
		}
	}

	log.G(ctx).WithFields(log.Fields{
		"url":  opts.Url,
		"path": opts.Path,
	}).Debug("cloning repo")
	r, err := gg.PlainCloneContext(ctx, opts.Path, false, &gg.CloneOptions{
		Depth:    1,
		URL:      opts.Url,
		Auth:     auth,
		Progress: os.Stdout,
	})
	if err != nil {
		return nil, err
	}

	return &repo{r}, nil
}

func Init(ctx context.Context, path string) (Repository, error) {
	l := log.G(ctx).WithFields(log.Fields{
		"path": path,
	})

	l.Debug("initiallizing local repository")
	r, err := gg.PlainInit(path, false)
	if err != nil {
		return nil, err
	}
	l.Debug("local repository initiallized")

	return &repo{r}, err
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
