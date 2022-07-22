package client

type ClientOpt func(c *ClientConfig)

// WithToken is an opt that sets the client token
func WithToken(token string) ClientOpt {
	return func(c *ClientConfig) {
		c.Token = token
	}
}

// WithServiceToken is an opt that sets the client service token
func WithServiceToken(token string) ClientOpt {
	return func(c *ClientConfig) {
		c.ServiceToken = token
	}
}

// WithAPIToken is an opt that sets the client user api token
func WithAPIToken(token string) ClientOpt {
	return func(c *ClientConfig) {
		c.APIToken = token
	}
}

// WithUsername is an opt that sets the client username
func WithUsername(username string) ClientOpt {
	return func(c *ClientConfig) {
		c.Username = username
	}
}

// WithNamespace is an opt that sets the client namespace
func WithNamespace(ns string) ClientOpt {
	return func(c *ClientConfig) {
		c.Namespace = ns
	}
}

// WithInsecure is an opt to set insecure for GRPC
func WithInsecure() ClientOpt {
	return func(c *ClientConfig) {
		c.Insecure = true
	}
}
