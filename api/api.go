package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/levigross/grequests"
	"go.uber.org/zap"

	"github.com/cocov-ci/worker/redis"
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
	SetCheckCanceled(job *redis.Job, plugin string) error
	SetSetRunning(job *redis.Job) error
	PushIssues(job *redis.Job, plugin string, issues any) error
	WrapUp(job *redis.Job) error
	GetSecret(secret *redis.Mount) ([]byte, error)
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
	_, err := a.do("GET", "/system/probes/health", nil)
	return err
}

func (a api) SetCheckError(job *redis.Job, plugin, reason string) error {
	_, err := a.do("PATCH", fmt.Sprintf("/v1/repositories/%d/commits/%s/checks", job.RepoID, job.Commitish), &grequests.RequestOptions{
		JSON: map[string]any{
			"plugin_name":  strings.SplitN(plugin, ":", 2)[0],
			"status":       "errored",
			"error_output": reason,
		},
	})

	return err
}

func (a api) SetCheckRunning(job *redis.Job, plugin string) error {
	_, err := a.do("PATCH", fmt.Sprintf("/v1/repositories/%d/commits/%s/checks", job.RepoID, job.Commitish), &grequests.RequestOptions{
		JSON: map[string]any{
			"plugin_name": strings.SplitN(plugin, ":", 2)[0],
			"status":      "in_progress",
		},
	})

	return err
}

func (a api) SetCheckSucceeded(job *redis.Job, plugin string) error {
	_, err := a.do("PATCH", fmt.Sprintf("/v1/repositories/%d/commits/%s/checks", job.RepoID, job.Commitish), &grequests.RequestOptions{
		JSON: map[string]any{
			"plugin_name": strings.SplitN(plugin, ":", 2)[0],
			"status":      "completed",
		},
	})

	return err
}

func (a api) SetCheckCanceled(job *redis.Job, plugin string) error {
	_, err := a.do("PATCH", fmt.Sprintf("/v1/repositories/%d/commits/%s/checks", job.RepoID, job.Commitish), &grequests.RequestOptions{
		JSON: map[string]any{
			"plugin_name": strings.SplitN(plugin, ":", 2)[0],
			"status":      "canceled",
		},
	})

	return err
}

func (a api) PushIssues(job *redis.Job, plugin string, issues any) error {
	_, err := a.do("PUT", fmt.Sprintf("/v1/repositories/%d/issues", job.RepoID), &grequests.RequestOptions{
		JSON: map[string]any{
			"source": plugin,
			"sha":    job.Commitish,
			"issues": issues,
		},
	})

	return err
}

func (a api) WrapUp(job *redis.Job) error {
	_, err := a.do("POST", fmt.Sprintf("/v1/repositories/%d/commits/%s/checks/wrap_up", job.RepoID, job.Commitish), &grequests.RequestOptions{})

	return err
}

func (a api) SetSetRunning(job *redis.Job) error {
	_, err := a.do("POST", fmt.Sprintf("/v1/repositories/%d/commits/%s/check_set/notify_in_progress", job.RepoID, job.Commitish), &grequests.RequestOptions{})

	return err
}

func (a api) GetSecret(secret *redis.Mount) ([]byte, error) {
	if secret.Kind != "secret" {
		return nil, fmt.Errorf("cannot acquire non-secret mount %s", secret.Kind)
	}

	return a.do("GET", "/v1/secrets/data", &grequests.RequestOptions{
		Params: map[string]string{"authorization": secret.Authorization},
	})
}
