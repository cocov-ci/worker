package redis

type ControlRequest struct {
	Operation string `json:"operation"`
}

type CancellationRequest struct {
	CheckSetId int    `json:"check_set_id,omitempty"`
	JobId      string `json:"job_id,omitempty"`
}
