package finca

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nats-io/nats.go"
)

const (
	// ServerQueueGroupName is the name for the server queue group
	ServerQueueGroupName = "finca-servers"
	// WorkerQueueGroupName is the name for the worker queue group
	WorkerQueueGroupName = "finca-workers"

	// S3ProjectPath is the project for project files
	S3ProjectPath = "projects"
	// S3RenderPath is the s3 bucket for final renders
	S3RenderPath = "render"
	// S3JobPath is the path to the render job config
	S3JobPath = "job.json"
	// S3JobStatusContentType is the content type for job status objects
	S3JobContentType = "application/json"

	// KVBucketTTLWorkers is the TTL for the worker bucket
	KVBucketTTLWorkers = time.Second * 10

	// queueJobSubject is the subject for queued messages
	queueJobSubject = "JOBS"
	// queueJobStatusSubject is the subject for job status updates
	queueJobStatusSubject = "STATUS"
	// kvBucketNameWorkers is the name of the kv store in the queue for the worker info
	kvBucketNameWorkers = "finca-workers"
	// kvBucketWorkerControl is the name of the kv store in the queue for issuing worker control messages
	kvBucketWorkerControl = "finca-worker-control"
	// objectStoreName is the name of the object store for the system
	objectStoreName = "finca"
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
	// NATSWorkerSubject is the queue subject for the workers
	NATSJobSubject string
	// NATSServerSubject is the queue subject for the servers
	NATSJobStatusSubject string
	// NATSKVBucketNameWorkers is the name of the kv store in the queue
	NATSKVBucketNameWorkers string
	// NATSKVBucketWorkerControl is the name of the kv store in the for worker control
	NATSKVBucketWorkerControl string
	// DatabaseAddress is the address of the database
	DatabaseAddress string
	// JobTimeout is the maximum amount of time a job can take
	JobTimeout duration
	// Workers are worker specific configuration
	Workers []*Worker
	// ProfilerAddress enables the performance profiler on the specified address
	ProfilerAddress string

	// TODO: cleanup
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

func DefaultConfig() *Config {
	return &Config{
		GRPCAddress:               "127.0.0.1:8080",
		NATSURL:                   nats.DefaultURL,
		NATSJobSubject:            queueJobSubject,
		NATSJobStatusSubject:      queueJobStatusSubject,
		NATSKVBucketNameWorkers:   kvBucketNameWorkers,
		NATSKVBucketWorkerControl: kvBucketWorkerControl,
		DatabaseAddress:           "redis://127.0.0.1:6379/0",
		JobTimeout:                duration{time.Second * 28800},
		JobPriority:               50,
		JobCPU:                    1000,
		JobMemory:                 1024,
		ProfilerAddress:           "",
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

// LoadConfig returns a Finca config from the specified file path
func LoadConfig(configPath string) (*Config, error) {
	var cfg *Config
	if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("A config file must be specified.  Generate a new one with the \"finca config\" command.")
		}
		return nil, err
	}

	return cfg, nil
}
