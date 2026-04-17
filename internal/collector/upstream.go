package collector

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"
)

type UpstreamChecker struct {
	url        string
	interval   time.Duration
	httpClient *http.Client
	mu         sync.RWMutex
	latest     string
	releaseURL string
	lastFetch  time.Time
	userAgent  string
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// NewUpstreamChecker returns a checker that polls every `interval` (min 1h enforced).
// `userAgent` is passed as the User-Agent header.
func NewUpstreamChecker(interval time.Duration, userAgent string) *UpstreamChecker {
	if interval < 1*time.Hour {
		log.Printf("warn: upstream check interval %v too short, enforcing minimum 1h", interval)
		interval = 1 * time.Hour
	}
	return &UpstreamChecker{
		url:       "https://api.github.com/repos/IceWhaleTech/ZimaOS/releases?per_page=30",
		interval:  interval,
		userAgent: userAgent,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start launches the polling goroutine. Performs one fetch immediately,
// then ticks every `interval`. Exits when ctx is cancelled.
func (u *UpstreamChecker) Start(ctx context.Context) {
	go func() {
		u.fetch()

		ticker := time.NewTicker(u.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				u.fetch()
			}
		}
	}()
}

// Latest returns the cached (version, release_url). Returns ("", "") if no
// successful fetch yet. Safe to call from multiple goroutines.
func (u *UpstreamChecker) Latest() (string, string) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.latest, u.releaseURL
}

func (u *UpstreamChecker) fetch() {
	req, err := http.NewRequest("GET", u.url, nil)
	if err != nil {
		log.Printf("warn: upstream check: %v", err)
		return
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", u.userAgent)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		log.Printf("warn: upstream check: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("warn: upstream check: http %d", resp.StatusCode)
		return
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		log.Printf("warn: upstream check: %v", err)
		return
	}

	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+$`)

	for _, r := range releases {
		if versionPattern.MatchString(r.TagName) {
			u.mu.Lock()
			u.latest = r.TagName
			u.releaseURL = r.HTMLURL
			u.lastFetch = time.Now()
			u.mu.Unlock()
			return
		}
	}

	log.Printf("warn: upstream check: no stable release found")
}
