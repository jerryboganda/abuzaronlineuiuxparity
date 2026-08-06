// Package syncer provides the branch-edge to central synchronization loop.
// It deliberately uses a configured server-side session cookie; no central
// database credentials are ever sent to a counter browser or stored in SQLite.
package syncer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/abuzar/abuzar-next/services/edge/internal/store"
)

type Client struct {
	store        *store.Store
	centralURL   string
	sessionToken string
	httpClient   *http.Client
	batchSize    int
}

type Result struct {
	Pushed        int   `json:"pushed"`
	Duplicates    int   `json:"duplicates"`
	Conflicts     int   `json:"conflicts"`
	Pulled        int   `json:"pulled"`
	CentralCursor int64 `json:"centralCursor"`
}

type syncPushResult struct {
	Accepted   int `json:"accepted"`
	Duplicates int `json:"duplicates"`
	Conflicts  int `json:"conflicts"`
}

type syncPullResult struct {
	Events     []store.Event `json:"events"`
	NextCursor int64         `json:"nextCursor"`
}

func New(localStore *store.Store, centralURL, sessionToken string) (*Client, error) {
	if localStore == nil {
		return nil, errors.New("local edge store is required")
	}
	centralURL = strings.TrimRight(strings.TrimSpace(centralURL), "/")
	if centralURL == "" {
		return nil, errors.New("central URL is required")
	}
	if _, err := url.ParseRequestURI(centralURL); err != nil {
		return nil, fmt.Errorf("invalid central URL: %w", err)
	}
	if strings.TrimSpace(sessionToken) == "" {
		return nil, errors.New("central session token is required")
	}
	return &Client{
		store:        localStore,
		centralURL:   centralURL,
		sessionToken: sessionToken,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		batchSize:    500,
	}, nil
}

func (c *Client) Run(ctx context.Context, interval time.Duration, report func(Result, error)) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if report == nil {
		report = func(Result, error) {}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		report(c.SyncOnce(ctx))
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *Client) SyncOnce(ctx context.Context) (Result, error) {
	result, err := c.push(ctx)
	if err != nil {
		return result, err
	}
	pulled, err := c.pull(ctx)
	result.Pulled = pulled
	if err != nil {
		return result, err
	}
	return result, nil
}

func (c *Client) push(ctx context.Context) (Result, error) {
	var result Result
	cursorValue, err := c.store.Cursor(ctx, "central_pushed_sequence")
	if err != nil {
		return result, err
	}
	cursor, _ := strconv.ParseInt(cursorValue, 10, 64)
	events, nextCursor, err := c.store.OutgoingAfter(ctx, cursor, c.batchSize)
	if err != nil {
		return result, err
	}
	if len(events) == 0 {
		return result, nil
	}
	response, err := c.request(ctx, http.MethodPost, "/v1/sync/push", map[string]any{"events": events})
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, responseError(response)
	}
	var body syncPushResult
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return result, err
	}
	if err := c.store.SetCursor(ctx, "central_pushed_sequence", strconv.FormatInt(nextCursor, 10)); err != nil {
		return result, err
	}
	result.Pushed = body.Accepted
	result.Duplicates = body.Duplicates
	result.Conflicts = body.Conflicts
	return result, nil
}

func (c *Client) pull(ctx context.Context) (int, error) {
	cursorValue, err := c.store.Cursor(ctx, "central_pull")
	if err != nil {
		return 0, err
	}
	cursor, _ := strconv.ParseInt(cursorValue, 10, 64)
	path := "/v1/sync/pull?cursor=" + strconv.FormatInt(cursor, 10) + "&limit=" + strconv.Itoa(c.batchSize)
	response, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return 0, responseError(response)
	}
	var body syncPullResult
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return 0, err
	}
	for _, event := range body.Events {
		if _, err := c.store.InsertPulledEvent(ctx, event); err != nil {
			return 0, err
		}
	}
	if err := c.store.SetCursor(ctx, "central_pull", strconv.FormatInt(body.NextCursor, 10)); err != nil {
		return 0, err
	}
	return len(body.Events), nil
}

func (c *Client) request(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader *strings.Reader
	if body == nil {
		reader = strings.NewReader("")
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = strings.NewReader(string(encoded))
	}
	request, err := http.NewRequestWithContext(ctx, method, c.centralURL+path, reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.AddCookie(&http.Cookie{Name: "abuzar_session", Value: c.sessionToken, Path: "/"})
	return c.httpClient.Do(request)
}

func responseError(response *http.Response) error {
	var problem struct {
		Detail string `json:"detail"`
		Code   string `json:"code"`
	}
	_ = json.NewDecoder(response.Body).Decode(&problem)
	if problem.Detail != "" {
		return fmt.Errorf("central sync %s: %s", problem.Code, problem.Detail)
	}
	return fmt.Errorf("central sync returned HTTP %d", response.StatusCode)
}
