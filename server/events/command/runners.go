package command

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_runner.go Runner

// Runner runs individual command workflows.
type Runner interface {
	Run(ctx *Context, cmd *Comment)
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_project_runner.go ProjectRunner

// Runner runs individual command workflows.
type ProjectRunner interface {
	Run(ctx ProjectContext) ProjectResult
}
