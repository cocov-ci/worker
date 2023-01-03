package redis

type GitStorage struct {
	Mode string `json:"mode,omitempty"`
	Path string `json:"path,omitempty"`
}

type Job struct {
	JobID      string     `json:"job_id,omitempty"`
	Org        string     `json:"org,omitempty"`
	Repo       string     `json:"repo,omitempty"`
	Commitish  string     `json:"sha,omitempty"`
	Checks     []string   `json:"checks,omitempty"`
	GitStorage GitStorage `json:"git_storage"`
}
