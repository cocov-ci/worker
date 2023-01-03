package api

import (
	"encoding/json"
	"fmt"
	"github.com/cocov-ci/worker/redis"
	"github.com/levigross/grequests"
	"go.uber.org/zap"
	"net/url"
	"strings"
)

type ServiceError struct {
	HTTPStatus int
	Body       []byte
}

func (e ServiceError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.HTTPStatus, e.Body)
}

type Error struct {
	Code       string `json:"code"`
	HTTPStatus int
	Message    string `json:"message"`
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s (HTTP %d)", e.Code, e.Message, e.HTTPStatus)
}

func New(baseHost, serviceToken string) (Client, error) {
	client := &api{
		baseHost: strings.TrimSuffix(baseHost, "/"),
		log:      zap.L().With(zap.String("facility", "api-client")),
		token:    serviceToken,
	}

	host, err := url.Parse(baseHost)
	if err != nil {
		return nil, err
	}

	if err := client.Ping(); err != nil {
		return nil, err
	}

	client.log.Info("Connected to Cocov API Server", zap.String("host", host.Host))

	return client, nil
}

type Client interface {
	Ping() error
	SetCheckError(job *redis.Job, plugin, reason string) error
	SetCheckRunning(job *redis.Job, plugin string) error
	SetCheckSucceeded(job *redis.Job, plugin string) error
	PushIssues(job *redis.Job, issues map[string]any, status string) error
}

type api struct {
	baseHost string
	log      *zap.Logger
	token    string
}

func (a api) do(method, endpoint string, opts *grequests.RequestOptions) ([]byte, error) {
	if opts == nil {
		opts = &grequests.RequestOptions{}
	}
	if opts.Headers == nil {
		opts.Headers = map[string]string{}
	}

	opts.Headers["Authorization"] = "bearer " + a.token
	opts.Headers["Accept"] = "application/json"

	resp, err := grequests.DoRegularRequest(method, a.baseHost+endpoint, opts)
	if err != nil {
		return nil, err
	}

	if !resp.Ok {
		responseData := resp.Bytes()
		if len(responseData) > 0 && responseData[0] == '{' {
			var managedError Error
			err = json.Unmarshal(responseData, &managedError)
			if err == nil {
				managedError.HTTPStatus = resp.StatusCode
				return nil, managedError
			}
		}
		return nil, ServiceError{Body: responseData, HTTPStatus: resp.StatusCode}
	}

	return resp.Bytes(), nil
}

func (a api) Ping() error {
	_, err := a.do("GET", "/v1/ping", nil)
	return err
}

func (a api) SetCheckError(job *redis.Job, plugin, reason string) error {
	_, err := a.do("PATCH", fmt.Sprintf("/v1/repositories/%s/commits/%s/checks", job.Repo, job.Commitish), &grequests.RequestOptions{
		JSON: map[string]any{
			"plugin_name":  strings.SplitN(plugin, ":", 2)[0],
			"status":       "errored",
			"error_output": reason,
		},
	})

	return err
}

func (a api) SetCheckRunning(job *redis.Job, plugin string) error {
	_, err := a.do("PATCH", fmt.Sprintf("/v1/repositories/%s/commits/%s/checks", job.Repo, job.Commitish), &grequests.RequestOptions{
		JSON: map[string]any{
			"plugin_name": strings.SplitN(plugin, ":", 2)[0],
			"status":      "running",
		},
	})

	return err
}

func (a api) SetCheckSucceeded(job *redis.Job, plugin string) error {
	_, err := a.do("PATCH", fmt.Sprintf("/v1/repositories/%s/commits/%s/checks", job.Repo, job.Commitish), &grequests.RequestOptions{
		JSON: map[string]any{
			"plugin_name": strings.SplitN(plugin, ":", 2)[0],
			"status":      "succeeded",
		},
	})

	return err
}

func (a api) PushIssues(job *redis.Job, issues map[string]any, status string) error {
	_, err := a.do("PUT", fmt.Sprintf("/v1/repositories/%s/issues", job.Repo), &grequests.RequestOptions{
		JSON: map[string]any{
			"status": status,
			"sha":    job.Commitish,
			"issues": issues,
		},
	})

	return err
}
