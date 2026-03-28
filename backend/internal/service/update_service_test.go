package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubUpdateGitHubReleaseClient struct {
	repo string
}

func (s *stubUpdateGitHubReleaseClient) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	s.repo = repo
	return &GitHubRelease{TagName: "v1.0.1"}, nil
}

func (s *stubUpdateGitHubReleaseClient) DownloadFile(context.Context, string, string, int64) error {
	return nil
}

func (s *stubUpdateGitHubReleaseClient) FetchChecksumFile(_ context.Context, url string) ([]byte, error) {
	return nil, nil
}

func TestNewUpdateService_UsesBuildConfiguredRepo(t *testing.T) {
	svc := NewUpdateService(nil, nil, "1.0.0", "release", "CoderDKai/sub2api")
	require.Equal(t, "CoderDKai/sub2api", svc.updateRepo)
}

func TestNewUpdateService_FallsBackToDefaultRepoWhenBlank(t *testing.T) {
	t.Setenv("SUB2API_UPDATE_REPO", "")
	svc := NewUpdateService(nil, nil, "1.0.0", "release", "")
	require.Equal(t, defaultUpdateRepo, svc.updateRepo)
}

func TestCompareVersions_UsesNumericSemverOnly(t *testing.T) {
	require.Equal(t, -1, compareVersions("1.0.0", "1.0.1"))
	require.Equal(t, 0, compareVersions("1.0.0", "1.0.0"))
	require.Equal(t, 1, compareVersions("1.1.0", "1.0.9"))
}

func TestCheckUpdate_UsesInjectedRepo(t *testing.T) {
	client := &stubUpdateGitHubReleaseClient{}
	svc := NewUpdateService(nil, client, "1.0.0", "release", "CoderDKai/sub2api")
	info, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, "CoderDKai/sub2api", client.repo)
	require.Equal(t, "1.0.1", info.LatestVersion)
	require.True(t, info.HasUpdate)
}
