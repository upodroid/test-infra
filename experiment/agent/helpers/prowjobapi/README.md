# ProwJob API

The server loads `experiment/agent/helpers/prowjobs.js` once during startup and
keeps that immutable snapshot in memory for the lifetime of the process. Startup
fails if the snapshot cannot be read or decoded.

Run it from the repository root:

```sh
go run ./experiment/agent/helpers/prowjobapi
```

The default listen address is `:8080`. The available endpoints are:

- `GET /healthz` returns the health status and number of loaded ProwJobs.
- `GET /prowjobs` returns the in-memory ProwJob list.
- `GET /prowjobs/{id}` returns one ProwJob by its Kubernetes metadata name.

`GET /prowjobs` accepts exact-match `name`, `job`, `org`, `repo`, `type`, and
`state` query parameters. Filters can be combined:

```sh
curl 'http://localhost:8080/prowjobs?org=kubernetes&repo=test-infra&state=failure'
```

The snapshot path and listen address can be changed:

```sh
go run ./experiment/agent/helpers/prowjobapi \
  --prowjobs-file=/path/to/prowjobs.js \
  --listen=:8081
```
