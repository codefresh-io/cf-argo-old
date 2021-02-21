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
		"No type": {
			&Options{},
			nil,
			ErrProviderNotSupported.Error(),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p, err := New(test.opts)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}
			assert.IsType(t, test.expectedProvider, p)
		})
	}
}
