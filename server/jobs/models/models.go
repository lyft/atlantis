package models

type OutputBuffer struct {
	OperationComplete bool
	Buffer            []string
}

type PullContext struct {
	PullNum     int
	Repo        string
	ProjectName string
	Workspace   string
}

type JobContext struct {
	PullContext
	HeadCommit string
}

type ProjectCmdOutputLine struct {
	JobID             string
	JobContext        JobContext
	Line              string
	OperationComplete bool
}
