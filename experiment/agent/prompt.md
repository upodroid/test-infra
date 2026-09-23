# Agent Prompt

You are starting a Datadog investigation for Kubernetes Prow jobs selected by user-provided Prow filters.

## Goal

Build a CPU/memory/runtime table for Kubernetes Prow job pods matching the requested filters with these columns:

- `pod`
- `job`
- `runtime`
- `requested CPU`
- `avg CPU used`
- `max 30s avg CPU used`
- `max 1m avg CPU used`
- `max 5m avg CPU used`
- `requested memory`
- `avg memory used`
- `max 30s avg memory used`
- `max 1m avg memory used`
- `max 5m avg memory used`

## Datadog Context

- Datadog site: `us5`.
- Use the Datadog MCP server for Datadog access.
- Use read-only Datadog MCP queries only; do not create, update, mute, delete, or acknowledge Datadog resources.
- Do not ask the user to manually execute Datadog queries if the Datadog MCP server is available.
- Don't search for jobs defined in this repo, instead get the jobs based on data you see in Datadog.
- If the Datadog MCP server is unavailable or lacks required permissions, stop and clearly report the missing capability or permission.
- Correct Prow job tag: `prow.k8s.io/job`.
- Correct Prow repository tag: `prow.k8s.io/repo`.
- Incorrect tag that returned no data: `kube_label_prow_k8s_io_job`.
- CPU request metric: `kubernetes.cpu.requests`, already in cores.
- CPU usage metric: `kubernetes.cpu.usage.total`, in nanocores; divide by `1,000,000,000` to get cores.
- Memory request metric: `kubernetes.memory.requests`, in bytes; divide by `1,073,741,824` to get GiB.
- Memory usage metric: `kubernetes.memory.usage`, in bytes; divide by `1,073,741,824` to get GiB.
- Avoid raw instantaneous CPU peaks because they produced unrealistic values above the 8-core node capacity.
- Prefer rolled-up averages:
  - `avg:kubernetes.cpu.usage.total{...}.rollup(avg,30)` for max 30s average CPU.
  - `avg:kubernetes.cpu.usage.total{...}.rollup(avg,60)` for max 1m average CPU.
  - `avg:kubernetes.cpu.usage.total{...}.rollup(avg,300)` for max 5m average CPU.
  - `avg:kubernetes.memory.usage{...}.rollup(avg,30)` for max 30s average memory.
  - `avg:kubernetes.memory.usage{...}.rollup(avg,60)` for max 1m average memory.
  - `avg:kubernetes.memory.usage{...}.rollup(avg,300)` for max 5m average memory.
- Runtime should come from Datadog Kubernetes/containerd events where possible, from pod/container start to terminal task/container destroy.
- If lifecycle events are incomplete, fall back to first/last observed logs and clearly label the source.

## Required Filter Prompt

Before running metric or event queries, ask the user for the Prow job filter input and, optionally, the Prow repo filter input.

- Use `JOB_FILTER` as the value for `prow.k8s.io/job`.
- Use `REPO_FILTER` as the value for `prow.k8s.io/repo`.
- If the user specifies a repo filter, set `JOB_FILTER` to `*`.
- If the user does not specify a repo filter, set `REPO_FILTER` to `*`.
- Do not hardcode any previous job prefix; only use the user-provided `JOB_FILTER` or `*` according to the rules above.

## Useful Queries

Run these through the Datadog MCP server. For metrics, use the MCP server's metric query/timeseries capability with the requested time window. For lifecycle evidence, use the MCP server's event/log search capability and sort results ascending by timestamp.

Requested CPU:

```text
max:kubernetes.cpu.requests{prow.k8s.io/job:JOB_FILTER,prow.k8s.io/repo:REPO_FILTER} by {prow.k8s.io/job,prow.k8s.io/repo,pod_name}
```

Average CPU used:

```text
avg:kubernetes.cpu.usage.total{prow.k8s.io/job:JOB_FILTER,prow.k8s.io/repo:REPO_FILTER} by {prow.k8s.io/job,prow.k8s.io/repo,pod_name}.rollup(avg,300)
```

Max 1m average CPU used:

```text
avg:kubernetes.cpu.usage.total{prow.k8s.io/job:JOB_FILTER,prow.k8s.io/repo:REPO_FILTER} by {prow.k8s.io/job,prow.k8s.io/repo,pod_name}.rollup(avg,60)
```

Max 30s average CPU used:

```text
avg:kubernetes.cpu.usage.total{prow.k8s.io/job:JOB_FILTER,prow.k8s.io/repo:REPO_FILTER} by {prow.k8s.io/job,prow.k8s.io/repo,pod_name}.rollup(avg,30)
```

Max 5m average CPU used:

```text
avg:kubernetes.cpu.usage.total{prow.k8s.io/job:JOB_FILTER,prow.k8s.io/repo:REPO_FILTER} by {prow.k8s.io/job,prow.k8s.io/repo,pod_name}.rollup(avg,300)
```

Requested memory:

```text
max:kubernetes.memory.requests{prow.k8s.io/job:JOB_FILTER,prow.k8s.io/repo:REPO_FILTER} by {prow.k8s.io/job,prow.k8s.io/repo,pod_name}
```

Average memory used:

```text
avg:kubernetes.memory.usage{prow.k8s.io/job:JOB_FILTER,prow.k8s.io/repo:REPO_FILTER} by {prow.k8s.io/job,prow.k8s.io/repo,pod_name}.rollup(avg,300)
```

Max 30s average memory used:

```text
avg:kubernetes.memory.usage{prow.k8s.io/job:JOB_FILTER,prow.k8s.io/repo:REPO_FILTER} by {prow.k8s.io/job,prow.k8s.io/repo,pod_name}.rollup(avg,30)
```

Max 1m average memory used:

```text
avg:kubernetes.memory.usage{prow.k8s.io/job:JOB_FILTER,prow.k8s.io/repo:REPO_FILTER} by {prow.k8s.io/job,prow.k8s.io/repo,pod_name}.rollup(avg,60)
```

Max 5m average memory used:

```text
avg:kubernetes.memory.usage{prow.k8s.io/job:JOB_FILTER,prow.k8s.io/repo:REPO_FILTER} by {prow.k8s.io/job,prow.k8s.io/repo,pod_name}.rollup(avg,300)
```

Lifecycle events for selected pods:

```text
pod_name:(POD1 OR POD2 OR POD3) (source:kubernetes OR source:containerd)
```

Sort lifecycle events ascending by timestamp.

## Runtime Evidence Rules

- Start timestamp: earliest meaningful `Scheduled`, `Started`, or containerd create/start event after failed scheduling noise.
- End timestamp: latest terminal containerd task delete/destroy event for test or sidecar, or an equivalent pod terminal event.
- Do not use `FailedScheduling` as runtime start if a later schedule/start exists.
- Clearly state when runtime is event-derived versus log-observed.

## Caveats

- Datadog metric responses were truncated for the all-job query: approximately 3,730 pod/job groups existed, but only 400 rows were emitted in one saved output.
- Some live Kubernetes inventory/manifest lookups failed because completed Prow pods had aged out of inventory.
- `metadata.creationTimestamp` was not available through the attempted manifest JSONPath for visible pods.
- A combined scalar query returned identical values for max 1m and max 5m average CPU for the selected pods; do not assume this always holds for future data.
