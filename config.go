package finca

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
	// NomadAddress is the endpoint for the Nomad cluster
	NomadAddress string
	// NomadRegion is the Nomad cluster region
	NomadRegion string
	// NomadNamespace is the Nomad cluster namespace
	NomadNamespace string
	// NomadDatacenters are the datacenters to use in jobs
	NomadDatacenters []string
	// JobPrefix is the prefix for all queued jobs
	JobPrefix string
	// JobPriority is the priority for queued jobs
	JobPriority int
	// JobImage is the container image that will be used to process the job
	JobImage string
	// JobMaxAttempts is the maximum number of attempts the job will be tried
	JobMaxAttempts int
	// JobCPU is the amount of CPU (in Mhz) that will be configured for each job
	JobCPU int
	// JobMemory is the amount of memory (in MB) that will be configured for each job
	JobMemory int
}
