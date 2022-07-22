package workflows

import (
	"time"
)

type Config struct {
	ID                    string
	Type                  string
	Address               string
	TLSCertificate        string
	TLSKey                string
	TLSInsecureSkipVerify bool
	MaxWorkflows          uint64
	ServiceToken          string
	WorkflowTimeout       time.Duration
	Namespace             string
}
