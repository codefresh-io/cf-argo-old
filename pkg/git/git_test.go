package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewProvider(t *testing.T) {
	tests := map[string]struct {
		opts             *Options
		expectedProvider Provider
		expectedError    string
	}{
		"Github": {
			&Options{
				Type: "github",
			},
			&github{},
			"",
		},
		"No Type": {
			&Options{},
			nil,
			ErrProviderNotSupported.Error(),
		},
		"Bad Type": {
			&Options{Type: "foo"},
			nil,
			ErrProviderNotSupported.Error(),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p, err := NewProvider(test.opts)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}
			assert.IsType(t, test.expectedProvider, p)
		})
	}
}

func Test_SplitCloneURL(t *testing.T) {
	tests := map[string]struct {
		given         string
		expectedOwner string
		expectedRepo  string
		err           string
	}{
		"Simple https": {
			given:         "https://github.com/owner/repo",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			err:           "",
		},
		"Simple http": {
			given:         "http://github.com/owner/repo",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			err:           "",
		},
		"Simple ssh": {
			given:         "ssh://git@github.com:owner/repo.git",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			err:           "",
		},
		"No protocol ssh": {
			given:         "git@github.com:owner/repo.git",
			expectedOwner: "owner",
			expectedRepo:  "repo",
			err:           "",
		},
		"No protocol error": {
			given:         "github.com/owner/repo.git",
			expectedOwner: "",
			expectedRepo:  "",
			err:           "malformed repository url",
		},
		"Unsupported protocol error": {
			given:         "ftp://github.com/owner/repo.git",
			expectedOwner: "",
			expectedRepo:  "",
			err:           "unsupported scheme in clone url \"ftp\"",
		},
		"Not a URL": {
			given:         "this is not a url",
			expectedOwner: "",
			expectedRepo:  "",
			err:           "malformed repository url",
		},
		"Not enough url parts": {
			given:         "https://github.com/owner",
			expectedOwner: "",
			expectedRepo:  "",
			err:           "malformed repository url",
		},
	}

	for tname, test := range tests {
		t.Run(tname, func(t *testing.T) {
			o, r, e := SplitCloneURL(test.given)
			if test.err != "" {
				assert.EqualError(t, e, test.err)
			} else {
				assert.NoError(t, e)
				assert.Equal(t, test.expectedRepo, r)
				assert.Equal(t, test.expectedOwner, o)
			}
		})
	}
}
