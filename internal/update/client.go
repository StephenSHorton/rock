package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/StephenSHorton/rock/internal/version"
)

const (
	defaultAPI  = "https://api.github.com"
	defaultRepo = "StephenSHorton/rock"
)

// Asset is one GitHub release file.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Release is the latest GitHub release we care about.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

func (r Release) Version() string { return Normalize(r.Tag) }

// Client talks to the GitHub releases API.
type Client struct {
	HTTP   *http.Client
	API    string
	Repo   string
	GOOS   string
	GOARCH string
}

func (c *Client) api() string {
	if c != nil && c.API != "" {
		return strings.TrimRight(c.API, "/")
	}
	if v := strings.TrimSpace(os.Getenv("ROCK_UPDATE_API")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultAPI
}

func (c *Client) repo() string {
	if c != nil && c.Repo != "" {
		return c.Repo
	}
	return defaultRepo
}

func (c *Client) goos() string {
	if c != nil && c.GOOS != "" {
		return c.GOOS
	}
	return runtime.GOOS
}

func (c *Client) goarch() string {
	if c != nil && c.GOARCH != "" {
		return c.GOARCH
	}
	return runtime.GOARCH
}

func (c *Client) http() *http.Client {
	if c != nil && c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// Latest fetches the newest GitHub release.
func (c *Client) Latest(ctx context.Context) (Release, error) {
	url := c.api() + "/repos/" + c.repo() + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "rock/"+version.Version)
	res, err := c.http().Do(req)
	if err != nil {
		return Release{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Release{}, err
	}
	if res.StatusCode >= 300 {
		return Release{}, fmt.Errorf("github %d: %s", res.StatusCode, truncate(string(body), 200))
	}
	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return Release{}, err
	}
	if rel.Tag == "" {
		return Release{}, fmt.Errorf("github release has no tag")
	}
	return rel, nil
}

// Fetch writes url to path.
func (c *Client) Fetch(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "rock/"+version.Version)
	res, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("download %s: http %d", url, res.StatusCode)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, io.LimitReader(res.Body, 80<<20))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
