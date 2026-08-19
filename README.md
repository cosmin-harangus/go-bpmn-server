# go-bpmn-server

HTTP server / reference implementation for [go-bpmn-engine](https://github.com/cosmin-harangus/go-bpmn-engine).

Exposes a REST API to deploy BPMN processes, create and run instances, handle jobs and user tasks, and publish messages. Backed by PostgreSQL.

## Requirements

- Go 1.26+
- PostgreSQL 14+

## Configuration

| Variable       | Required | Default | Description                          |
|---------------|----------|---------|--------------------------------------|
| `DATABASE_URL` | Yes      | —       | PostgreSQL connection string         |
| `PORT`         | No       | `8080`  | Port the HTTP server listens on      |

Copy `.env.example` to `.env` and fill in your values.

## Running

```bash
# From source
DATABASE_URL=postgres://user:password@localhost:5432/bpmn?sslmode=disable go run ./cmd/server

# From a release binary
DATABASE_URL=postgres://... ./go-bpmn-server
```

Migrations are applied automatically on startup.

## API

All requests require an `X-Tenant-ID` header.

### Processes

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/processes` | Deploy a BPMN 2.0 XML process definition |

```
POST /processes
Content-Type: application/xml
X-Tenant-ID: my-tenant

<body: BPMN 2.0 XML>
```

### Instances

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/instances` | List instances (`state`, `process_key`, `limit`, `offset`) |
| `POST` | `/instances` | Create and run an instance |
| `GET`  | `/instances/{id}` | Get instance detail with current tokens |
| `POST` | `/instances/{id}/run` | Resume execution of an existing instance |
| `POST` | `/instances/{id}/cancel` | Cancel a running instance |
| `POST` | `/instances/{id}/suspend` | Suspend a running instance |
| `POST` | `/instances/{id}/resume` | Resume a suspended instance |

```
POST /instances
X-Tenant-ID: my-tenant

{ "process_key": "my-process", "variables": { "orderId": "123" } }
```

### Jobs (service tasks)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/jobs/{id}/complete` | Complete a job with output variables |
| `POST` | `/jobs/{id}/fail` | Fail a job, decrement retries |

```
POST /jobs/{id}/complete
X-Tenant-ID: my-tenant

{ "variables": { "result": "ok" } }
```

```
POST /jobs/{id}/fail
X-Tenant-ID: my-tenant

{ "retries": 2, "message": "upstream timeout" }
```

### User tasks

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/user-tasks` | List user tasks (`state`, `assignee`, `candidate_group`, `process_key`, `instance_id`, `limit`, `offset`) |
| `POST` | `/user-tasks/{id}/complete` | Complete a user task with output variables |
| `POST` | `/user-tasks/{id}/claim` | Claim a task for a user |
| `POST` | `/user-tasks/{id}/unclaim` | Remove the task assignee |

```
POST /user-tasks/{id}/claim
X-Tenant-ID: my-tenant

{ "user_id": "alice" }
```

### Incidents

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/incidents` | List incidents (`state`, `instance_id`, `limit`, `offset`) |
| `POST` | `/incidents/{id}/resolve` | Resolve an incident, retry job with new retries/variables |

```
POST /incidents/{id}/resolve
X-Tenant-ID: my-tenant

{ "retries": 3, "variables": { "fixedInput": true } }
```

### Messages

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/messages` | Publish a message to correlate with waiting instances |

```
POST /messages
X-Tenant-ID: my-tenant

{ "message_name": "OrderReceived", "correlation_key": "order-123", "variables": { "amount": 99.99 } }
```

## Building

```bash
make build   # build
make fmt     # format
make vet     # go vet
```

## License

MIT
