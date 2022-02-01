package client

type ClientOpt func(c *ClientConfig)

// WithToken is an opt that sets the client token
func WithToken(token string) ClientOpt {
	return func(c *ClientConfig) {
		c.Token = token
	}
}
