package git

import "context"

type github struct {
	opts *Options
}

func newGithub(opts *Options) (Provider, error) {
	client := &github{opts}
	return client, nil
}

func (client *github) CloneRepository(ctx context.Context, url, path string) (Repository, error) {
	return cloneRepository(ctx, client.opts, url, path)
}

func (client *github) CreateRepository(ctx context.Context, owner, name, path string) (Repository, error) {
	return nil, nil
}
