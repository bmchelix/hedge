package job

type HedgeTrainJob struct {
	JobName   string `json:"job_name"`
	ImagePath string `json:"image_path"`
	DataFile  string `json:"data_file"`
}

type HedgeJobStatus struct {
	JobName string `json:"job_name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
