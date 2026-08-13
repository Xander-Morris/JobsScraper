package jobs

type JobSource interface {
	FetchJobs() ([]Job, error)
}