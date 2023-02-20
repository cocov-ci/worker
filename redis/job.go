package redis

type GitStorage struct {
	Mode string `json:"mode,omitempty"`
	Path string `json:"path,omitempty"`
}

type Check struct {
	Plugin string            `json:"plugin,omitempty"`
	Envs   map[string]string `json:"envs,omitempty"`
	Mounts []Mount           `json:"mounts,omitempty"`
}

type Mount struct {
	Kind          string `json:"kind,omitempty"`
	Authorization string `json:"authorization,omitempty"`
	Target        string `json:"target,omitempty"`
}

type Job struct {
	JobID      string     `json:"job_id,omitempty"`
	CheckSetId int        `json:"check_set_id,omitempty"`
	Org        string     `json:"org,omitempty"`
	Repo       string     `json:"repo,omitempty"`
	RepoID     uint64     `json:"repo_id,omitempty"`
	Commitish  string     `json:"sha,omitempty"`
	Checks     []Check    `json:"checks,omitempty"`
	GitStorage GitStorage `json:"git_storage"`
}
