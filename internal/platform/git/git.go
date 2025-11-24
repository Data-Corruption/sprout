package git

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// LatestGitHubReleaseTag returns the tag from /releases/latest redirect.
func LatestGitHubReleaseTag(ctx context.Context, repoURL string) (string, error) {
	latest := strings.TrimSuffix(repoURL, ".git") + "/releases/latest"

	client := &http.Client{
		Timeout: 10 * time.Second,
		// capture the first redirect but don’t follow it.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, latest, nil)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	loc := resp.Header.Get("Location") // e.g. ".../releases/tag/v2.1.0"
	if loc == "" {
		return "", fmt.Errorf("no redirect; status %s", resp.Status)
	}
	if i := strings.LastIndex(loc, "/tag/"); i >= 0 {
		return loc[i+5:], nil // drop ".../tag/"
	}
	return "", fmt.Errorf("unexpected Location %q", loc)
}

// GitHubReleaseSource implements the ReleaseSource interface for GitHub.
type GitHubReleaseSource struct{}

func (g *GitHubReleaseSource) GetLatest(ctx context.Context, repoURL string) (string, error) {
	return LatestGitHubReleaseTag(ctx, repoURL)
}
