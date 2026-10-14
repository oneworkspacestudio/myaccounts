package portable

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

type ReleaseInfo struct {
	Tag        string `json:"tag_name"`
	URL        string `json:"html_url"`
	Prerelease bool   `json:"prerelease"`
}

func CheckUpdate(ctx context.Context, token string) (ReleaseInfo, error) {
	var info ReleaseInfo
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/fahimc/myaccounts/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, e := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if e != nil {
		return info, e
	}
	defer response.Body.Close()
	if response.StatusCode == 404 {
		return info, errors.New("no published release is available, or the private repository requires an authorized GitHub token")
	}
	if response.StatusCode != 200 {
		return info, errors.New("GitHub release check was rejected")
	}
	e = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&info)
	return info, e
}
