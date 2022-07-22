# Flow Distributed Workflow System

[![Docs](https://img.shields.io/badge/Docs-Documentation-blue)](https://flow.ehazlett.dev)

# Flow Server
The Flow server manages job submissions. It provides a GRPC API that is used by clients
to submit and manage workflows.

# Flow CLI
The Flow CLI manages workflows from the command line. You can queue, view, and delete workflows.

```
NAME:
   fctl workflows queue - queue workflow

USAGE:
   fctl workflows queue [command options] [arguments...]

OPTIONS:
   --name value, -n value               workflow instance name
   --type value, -t value               workflow type
   --input-file value, -f value         file path to use as workflow input
   --input-workflow-id value, -w value  output from another workflow to use as input (can be multiple)
   --priority value                     workflow priority (low, normal, urgent) (default: "normal")
   --parameter value, -p value          specify workflow parameters (KEY=VAL)
   --help, -h                           show help (default: false)

```

# PostgreSQL
[PostgreSQL](https://www.postgresql.org) is used for database storage for workflow metadata, user accounts, and queueing.

# MinIO
[MinIO](https://min.io/) is used for workflow input and output storage.  Minio is lightweight
and works very well with Flow but any compatible S3 system should work.

# Flow Processors
Flow processors provide the processing logic for queued workflows. Processors use GRPC to access the
Flow server which streams workflow events to the corresponding processor type.
