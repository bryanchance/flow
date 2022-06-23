// Copyright 2022 Evan Hazlett
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package flow

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	// QueueSubjectJobPriorityNormal is normal priority for workflows
	QueueSubjectJobPriorityNormal = "normal"
	// QueueSubjectJobPriorityUrgent is urgent priority for workflows (processed first)
	QueueSubjectJobPriorityUrgent = "urgent"
	// QueueSubjectJobPriorityLow is low priority for workflows (processed last)
	QueueSubjectJobPriorityLow = "low"

	// S3WorkflowPath is the path in the s3 bucket for workflow content
	S3WorkflowPath = "workflows"

	// CtxTokenKey is the key stored in the context for the token
	CtxTokenKey = "token"
	// CtxServiceTokenKey is the service key stored in the context
	CtxServiceTokenKey = "service-token"
	// CtxTokenKey is the key stored in the context for the username
	CtxUsernameKey = "username"
	// CtxTokenKey is the key stored in the context if the user is an admin
	CtxAdminKey = "isAdmin"
	// CtxNamespaceKey is the key stored in the context for the namespace
	CtxNamespaceKey = "namespace"
	// CtxDefaultNamespace is the default key used when unauthenticated and no auth
	CtxDefaultNamespace = "default"
)

type duration struct {
	time.Duration
}

func (d duration) MarshalText() (text []byte, err error) {
	ds := fmt.Sprintf("%v", d.Duration)
	text = []byte(ds)
	return
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type AuthenticatorConfig struct {
	// Name is the name of the authenticator
	Name string
	// Options are passed to the authenticator
	Options map[string]string
}

// Config is the configuration used for the server
type Config struct {
	// GRPCAddress is the address for the grpc server
	GRPCAddress string
	// TLSCertificate is the certificate used for grpc communication
	TLSServerCertificate string
	// TLSKey is the key used for grpc communication
	TLSServerKey string
	// TLSClientCertificate is the client certificate used for communication
	TLSClientCertificate string
	// TLSClientKey is the client key used for communication
	TLSClientKey string
	// TLSInsecureSkipVerify disables certificate verification
	TLSInsecureSkipVerify bool
	// S3Endpoint is the endpoint for the S3 compatible service
	S3Endpoint string
	// S3AccessID is the S3 access id
	S3AccessID string
	// S3AccessKey is the S3 key
	S3AccessKey string
	// S3Bucket is the S3 bucket
	S3Bucket string
	// S3UseSSL enables SSL for the S3 service
	S3UseSSL bool
	// NATSUrl is the URL for the NATS server
	NATSURL string
	// NATSJobStreamName is the queue subject for the workers
	NATSJobStreamName string
	// NATSJobStatusStreamName is the queue subject for the servers
	NATSJobStatusStreamName string
	// NATSKVBucketWorkerControl is the name of the kv store in the for worker control
	NATSKVBucketWorkerControl string
	// DatabaseAddress is the address of the database
	DatabaseAddress string
	// QueueAddress is the address of the queue
	QueueAddress string
	// WorkflowTimeout is the max timeout for workers to process workflows
	WorkflowTimeout duration
	// Workers are worker specific configuration
	Workers []*Worker
	// ProfilerAddress enables the performance profiler on the specified address
	ProfilerAddress string
	// TraceEndpoint is the endpoint of the telemetry tracer
	TraceEndpoint string
	// Environment is the environment the app is running in
	Environment string
	// InitialAdminPassword is the password used when creating the initial admin account. If empty, a random one is generated.
	InitialAdminPassword string
	// Authenticator is the auth configuration
	Authenticator *AuthenticatorConfig

	// TODO: cleanup
	// JobTimeout is the maximum amount of time a job can take
	JobTimeout duration
	// JobPrefix is the prefix for all queued jobs
	JobPrefix string
	// JobPriority is the priority for queued jobs
	JobPriority int
	// JobCPU is the amount of CPU (in Mhz) that will be configured for each job
	JobCPU int
	// JobMemory is the amount of memory (in MB) that will be configured for each job
	JobMemory int
}

type Worker struct {
	// Name is the name of the worker to apply the configuration
	// leave blank for all
	Name string
	// MaxJobs is the maximum number of jobs this worker will perform and then exit
	MaxJobs int
	// RenderEngines is a list of enabled render engines for the worker
	RenderEngines []*RenderEngine
}

type RenderEngine struct {
	// Name is the name of the render engine
	Name string
	// Path is the path to the render engine executable
	Path string
}

func (c *Config) GetJobTimeout() time.Duration {
	return c.JobTimeout.Duration
}

func (c *Config) GetWorkflowTimeout() time.Duration {
	return c.JobTimeout.Duration
}

func DefaultConfig() *Config {
	return &Config{
		GRPCAddress:     "127.0.0.1:8080",
		DatabaseAddress: "redis://127.0.0.1:6379/0",
		ProfilerAddress: "",
	}
}

// GetWorkerConfig returns the node specific configuration or default if not found
func (c *Config) GetWorkerConfig(name string) *Worker {
	worker := &Worker{
		MaxJobs: 0,
	}
	for _, w := range c.Workers {
		if w.Name == name {
			return w
		}
		if w.Name == "" {
			worker = w
		}
	}
	// if no specific configuration found return default
	return worker
}

// LoadConfig returns a Flow config from the specified file path
func LoadConfig(configPath string) (*Config, error) {
	var cfg *Config
	if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("A config file must be specified.  Generate a new one with the \"flow config\" command.")
		}
		return nil, err
	}

	return cfg, nil
}
