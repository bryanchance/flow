package client

type ClientOpt func(c *ClientConfig)

// WithToken is an opt that sets the client token
func WithToken(token string) ClientOpt {
	return func(c *ClientConfig) {
		c.Token = token
	}
}

// WithUsername is an opt that sets the client username
func WithUsername(username string) ClientOpt {
	return func(c *ClientConfig) {
		c.Username = username
	}
}
