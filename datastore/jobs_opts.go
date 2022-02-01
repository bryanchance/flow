package datastore

type JobOpt func(c *JobOptConfig)

// WithExcludeJobFrames
func WithExcludeJobFrames(c *JobOptConfig) {
	c.excludeFrames = true
}

// WithExcludeJobSlices
func WithExcludeJobSlices(c *JobOptConfig) {
	c.excludeSlices = true
}
