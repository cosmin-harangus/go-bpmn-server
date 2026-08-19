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

### Deploy a process

```
POST /processes
Content-Type: application/xml
X-Tenant-ID: my-tenant

<body: BPMN 2.0 XML>
```

### Create and run an instance

```
POST /instances
X-Tenant-ID: my-tenant

{
  "process_key": "my-process",
  "variables": { "orderId": "123" }
}
```

### Run an existing instance

```
POST /instances/{id}/run
X-Tenant-ID: my-tenant
```

### Complete a job (service task)

```
POST /jobs/{id}/complete
X-Tenant-ID: my-tenant

{
  "variables": { "result": "ok" }
}
```

### Fail a job

```
POST /jobs/{id}/fail
X-Tenant-ID: my-tenant

{
  "retries": 2,
  "message": "upstream timeout"
}
```

### Complete a user task

```
POST /user-tasks/{id}/complete
X-Tenant-ID: my-tenant

{
  "variables": { "approved": true }
}
```

### Publish a message

```
POST /messages
X-Tenant-ID: my-tenant

{
  "message_name": "OrderReceived",
  "correlation_key": "order-123",
  "variables": { "amount": 99.99 }
}
```

## Building

```bash
make build   # build
make fmt     # format
make vet     # go vet
```

## License

MIT
