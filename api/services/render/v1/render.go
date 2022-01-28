package render

type RenderJob interface {
	GetID() string
	GetJobSource() string
	GetRequest() *JobRequest
	GetRenderFrame() int64
}
