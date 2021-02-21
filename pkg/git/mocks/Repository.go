// Code generated by mockery v1.1.1. DO NOT EDIT.

package mocks

import (
	context "context"

	git "github.com/codefresh-io/cf-argo/pkg/git"
	mock "github.com/stretchr/testify/mock"
)

// Repository is an autogenerated mock type for the Repository type
type Repository struct {
	mock.Mock
}

// Add provides a mock function with given fields: ctx, pattern
func (_m *Repository) Add(ctx context.Context, pattern string) error {
	ret := _m.Called(ctx, pattern)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, pattern)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddRemote provides a mock function with given fields: ctx, name, url
func (_m *Repository) AddRemote(ctx context.Context, name string, url string) error {
	ret := _m.Called(ctx, name, url)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, name, url)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Commit provides a mock function with given fields: ctx, msg
func (_m *Repository) Commit(ctx context.Context, msg string) (string, error) {
	ret := _m.Called(ctx, msg)

	var r0 string
	if rf, ok := ret.Get(0).(func(context.Context, string) string); ok {
		r0 = rf(ctx, msg)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, msg)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// IsNewRepo provides a mock function with given fields:
func (_m *Repository) IsNewRepo() (bool, error) {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Push provides a mock function with given fields: _a0, _a1
func (_m *Repository) Push(_a0 context.Context, _a1 *git.PushOptions) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *git.PushOptions) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Root provides a mock function with given fields:
func (_m *Repository) Root() (string, error) {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
