package api

import (
	"encoding/json"
	"github.com/cocov-ci/worker/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)
import "github.com/heyvito/httpie"

func httpStatusCode(code int, body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}
}

func makeServer(t *testing.T, responses ...*httpie.Response) *httpie.Server {
	srv := httpie.New(responses...)
	t.Cleanup(srv.Stop)
	return srv
}

func TestNew(t *testing.T) {
	t.Run("Invalid URL", func(t *testing.T) {
		client, err := New(string([]byte{0x7f}), "foobar")
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "invalid control character")
	})

	t.Run("When ping fails", func(t *testing.T) {
		server := makeServer(t,
			httpie.WithCustom("/v1/ping", httpStatusCode(404, "Not found!")),
		)

		client, err := New(server.URL, "foobar")
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "HTTP 404: Not found!")
	})

	t.Run("When ping succeeds", func(t *testing.T) {
		server := makeServer(t,
			httpie.WithCustom("/v1/ping", httpStatusCode(200, "Yay")),
		)

		client, err := New(server.URL, "foobar")
		assert.NoError(t, err)
		assert.NotNil(t, client)
	})
}

func TestParseManagedService(t *testing.T) {
	data, err := json.Marshal(map[string]string{
		"code":    "foo.bar",
		"message": "This is a test",
	})
	require.NoError(t, err)

	server := makeServer(t,
		httpie.WithCustom("/v1/ping", httpStatusCode(500, string(data))),
	)

	client, err := New(server.URL, "foobar")
	assert.Nil(t, client)
	assert.ErrorAs(t, err, &Error{})
	assert.Equal(t, "foo.bar", err.(Error).Code)
	assert.Equal(t, "This is a test", err.(Error).Message)
	assert.Equal(t, 500, err.(Error).HTTPStatus)
	assert.Equal(t, "foo.bar: This is a test (HTTP 500)", err.Error())
}

func makeTestServer(t *testing.T, endpoint string, handler http.HandlerFunc) Client {
	srv := makeServer(t,
		httpie.WithCustom("/v1/ping", httpStatusCode(200, "")),
		httpie.WithCustom(endpoint, handler),
	)

	cli, err := New(srv.URL, "token")
	assert.NoError(t, err)
	return cli
}

func makeJob() *redis.Job {
	return &redis.Job{
		JobID:      "jid",
		RepoID:     10,
		Org:        "org",
		Repo:       "repo",
		Commitish:  "sha",
		Checks:     nil,
		GitStorage: redis.GitStorage{},
	}
}

func getJSON[T any](t *testing.T, from *http.Request) T {
	var v T
	err := json.NewDecoder(from.Body).Decode(&v)
	require.NoError(t, err)
	return v
}

func TestSetCheckError(t *testing.T) {
	ok := make(chan bool)
	cli := makeTestServer(t, "/v1/repositories/10/commits/sha/checks", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "PATCH")
		data := getJSON[map[string]string](t, r)
		assert.Equal(t, "plugin", data["plugin_name"])
		assert.Equal(t, "errored", data["status"])
		assert.Equal(t, "reason", data["error_output"])
		assert.Equal(t, "bearer token", r.Header.Get("Authorization"))
		close(ok)
	})

	err := cli.SetCheckError(makeJob(), "plugin", "reason")
	<-ok
	require.NoError(t, err)
}

func TestSetCheckRunning(t *testing.T) {
	ok := make(chan bool)
	cli := makeTestServer(t, "/v1/repositories/10/commits/sha/checks", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "PATCH")
		data := getJSON[map[string]string](t, r)
		assert.Equal(t, "plugin", data["plugin_name"])
		assert.Equal(t, "running", data["status"])
		assert.Equal(t, "bearer token", r.Header.Get("Authorization"))
		close(ok)
	})

	err := cli.SetCheckRunning(makeJob(), "plugin")
	<-ok
	require.NoError(t, err)
}

func TestSetCheckSucceeded(t *testing.T) {
	ok := make(chan bool)
	cli := makeTestServer(t, "/v1/repositories/10/commits/sha/checks", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "PATCH")
		data := getJSON[map[string]string](t, r)
		assert.Equal(t, "plugin", data["plugin_name"])
		assert.Equal(t, "succeeded", data["status"])
		assert.Equal(t, "bearer token", r.Header.Get("Authorization"))
		close(ok)
	})

	err := cli.SetCheckSucceeded(makeJob(), "plugin")
	<-ok
	require.NoError(t, err)
}

func TestPushIssues(t *testing.T) {
	ok := make(chan bool)
	cli := makeTestServer(t, "/v1/repositories/10/issues", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "PUT")
		data := getJSON[map[string]any](t, r)
		assert.Equal(t, "status", data["status"].(string))
		assert.Equal(t, "sha", data["sha"].(string))
		assert.Equal(t, "true", data["issues"].(map[string]any)["test"].(string))
		assert.Equal(t, "bearer token", r.Header.Get("Authorization"))
		close(ok)
	})

	err := cli.PushIssues(makeJob(), map[string]any{"test": "true"}, "status")
	<-ok
	require.NoError(t, err)
}
