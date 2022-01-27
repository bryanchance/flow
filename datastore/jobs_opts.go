package datastore

// WithExcludeJobFrames
func WithExcludeJobFrames(c *JobOptConfig) {
	c.excludeFrames = true
}

// WithExcludeJobSlices
func WithExcludeJobSlices(c *JobOptConfig) {
	c.excludeSlices = true
}
