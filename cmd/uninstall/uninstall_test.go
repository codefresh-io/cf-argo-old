package uninstall

import (
	"context"
	"testing"

	"github.com/codefresh-io/cf-argo/pkg/git"
	mockGit "github.com/codefresh-io/cf-argo/pkg/git/mocks"
	"github.com/codefresh-io/cf-argo/pkg/log"
	"github.com/stretchr/testify/mock"
)

func generateMockRepo() *mockGit.Repository {
	mockRepo := new(mockGit.Repository)
	mockRepo.On("Add", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("string")).Return(nil)
	mockRepo.On("Commit", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("string")).Return("hash", nil)
	mockRepo.On("Push", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("*git.PushOptions")).Return(nil)
	return mockRepo
}

func generateContext() context.Context {
	return log.WithLogger(context.Background(), log.NopLogger{})
}

func Test_persistGitopsRepo(t *testing.T) {
	mockRepo := generateMockRepo()
	values.GitopsRepo = mockRepo

	ctx := generateContext()

	msg, gitToken := "some message", "some token"
	persistGitopsRepo(ctx, &options{
		gitToken: gitToken,
	}, msg)
	mockRepo.AssertCalled(t, "Add", ctx, ".")
	mockRepo.AssertCalled(t, "Commit", ctx, msg)
	mockRepo.AssertCalled(t, "Push", ctx, &git.PushOptions{
		Auth: &git.Auth{
			Password: gitToken,
		},
	})
}

func Test_persistGitopsRepo_dryRun(t *testing.T) {
	mockRepo := generateMockRepo()
	values.GitopsRepo = mockRepo

	ctx := generateContext()

	msg := "some message"
	persistGitopsRepo(ctx, &options{
		dryRun: true,
	}, msg)
	mockRepo.AssertCalled(t, "Add", ctx, ".")
	mockRepo.AssertCalled(t, "Commit", ctx, msg)
	mockRepo.AssertNotCalled(t, "Push")
}

