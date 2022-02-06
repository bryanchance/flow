# Fynca Render System
Fynca is a lightweight distributed render system for Blender.

```
                   ┌─────────────────┐
                   │  Fynca CLI / UI │
                   └─────────────────┘
                          │
                          │
                          ▼
           ┌────────────────────────────┐
           │                            │
           │        Fynca Server        │─ ─ ─ ┐
           │                            │      ▼
           └────────────────────────────┘  ┌───────┐
                          │                │       │
                          │                │ Minio │◀ ┐
                          │                │       │
                          ▼                └───────┘  │
┌──────────────────────────────────────────────────┐
│                                                  │  │
│                 Fynca Workers                    │
│                                                  │  │
│                                                  │
│ ┌────────┐  ┌────────┐   ┌────────┐  ┌────────┐  │  │
│ │j1      │  │        │   │        │  │ j3     │  │
│ │        │  │        │   │        │  │        │  │  │
│ │        │  │j2s0    │   │j2s1    │  │        │  │
│ └────────┘  └────────┘   └────────┘  └────────┘  │─ ┘
│                                                  │
│ ┌────────┐  ┌────────┐   ┌────────┐  ┌────────┐  │
│ │        │  │        │   │        │  │        │  │
│ │        │  │        │   │        │  │        │  │
│ │ j2s2   │  │ j2s3   │   │ j4     │  │ j5     │  │
│ └────────┘  └────────┘   └────────┘  └────────┘  │
│                                                  │
└──────────────────────────────────────────────────┘
```

# Fynca Server
The Fynca server manages job submissions.  It provides a GRPC API that is used by clients
to submit and manage render jobs.

# Fynca CLI
The Fynca CLI manages render jobs from the command line.  It is intended as an administrative
interface to the Fynca server as it provides all configuration options.

```
NAME:
   fctl queue - queue render job

USAGE:
   fctl queue [command options] [arguments...]

OPTIONS:
   --name value                          job name
   --project-file value, -f value        path to project file
   --resolution-x value, -x value        render resolution (x) (default: 1920)
   --resolution-y value, -y value        render resolution (y) (default: 1080)
   --resolution-scale value              render resolution scale (default: 100)
   --render-start-frame value, -s value  render start frame (default: 1)
   --render-end-frame value, -e value    render end frame (default: 0)
   --render-samples value                render samples (default: 100)
   --render-use-gpu, -g                  use gpu for rendering (default: false)
   --render-priority value, -p value     set render priority (0-100) (default: 50)
   --cpu value                           specify cpu (in Mhz) for each task in the render job (default: 0)
   --memory value                        specify memory (in MB) for each task in the render job (default: 0)
   --render-slices value                 number of slices to subdivide a frame for rendering (default: 0)
   --help, -h                            show help (default: false)
```

# NATS
Fynca uses the [NATS](https://www.nats.io/) queue for processing render jobs.

# Redis
[Redis](https://redis.io) is used for database storage for job metadata and user accounts.

# Minio
[Minio](https://min.io/) is recommended for both project storage and render task results.  Minio is lightweight
and works very well with Fynca but any compatible S3 system should work.

# Fynca Worker
The Fynca worker is a small application that manages Blender to process jobs.  It accesses the NATS queue for
jobs and stores the results in MinIO.

# Render Slices
For optimized rendering with very large resolutions or high cycle counts, render slicing can be enabled that will
split the frame into regions to be rendered across the cluster.  When render slicing is enabled, an extra
compositing task is used to composite each slice into a final render.  This is transparent to the user and the result
is the same in Minio.  Job render times are improved dramatically depending on the number of nodes and their
corresponding performance.
