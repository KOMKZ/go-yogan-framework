# Queue Component

`queue` is the framework abstraction for long-running background tasks. The
first adapter is Asynq. Application and domain code should depend on
`queue.TaskDefinition`, `queue.Registry`, `queue.Client`, and `queue.Handler`,
not on Asynq types.

The framework owns governance and dispatch:

- task metadata and runtime config merge;
- sync in-process dispatch through the same registry used by workers;
- queue enqueue options, trace headers, timeout, retention, and queue
  selection;
- worker server dispatch through `Registry.Dispatch`.

Application code should not hand-roll a second sync path or duplicate handler
lifecycle code.

## Config

```yaml
queue:
  enabled: true
  driver: asynq
  brokers:
    jobs:
      redis:
        mode: dedicated
        addr: 127.0.0.1:6379
        db: 3
    media:
      redis:
        mode: dedicated
        addr: 127.0.0.1:6379
        db: 4
  queues:
    default:
      broker: jobs
      name: default
      max_pending: 10000
    media:
      broker: media
      name: default
      max_pending: 1000
  default_task:
    queue: default
    mode: queue
    timeout: 5m
  profiles:
    default:
      workers:
        jobs:
          concurrency: 5
          queues:
            default: 1
    media:
      workers:
        media:
          concurrency: 5
          queues:
            media: 1
  tasks:
    export.demo:
      queue: default
      mode: queue # queue | sync
      timeout: 10m
```

Asynq's direct Redis option does not support a key prefix. Use a dedicated Redis
DB or instance per broker for queue isolation.

## Brokers And Queues

`broker` is the Redis/Asynq instance boundary. `queue` is the logical routing
name used by tasks and business code. A logical queue resolves to one broker and
one physical Asynq queue name inside that broker.

Worker profiles describe which logical queues this worker process consumes. A
single profile can start one Asynq server per broker:

```yaml
queue:
  profiles:
    mixed:
      workers:
        jobs:
          concurrency: 10
          queues:
            default: 1
        media:
          concurrency: 4
          queues:
            media: 1
```

Application/domain code should only use logical queue names. Redis instance
selection remains framework routing policy.

## Queue Capacity

`queues.<name>.max_pending` is a producer-side capacity guard. When greater than
zero, the Asynq client checks the target queue's pending count before enqueueing. If
`pending >= max_pending`, enqueue is rejected with `ErrQueueBacklogExceeded`.
The returned `BacklogExceededError` includes the queue name, current pending
count, and configured limit.

`max_pending <= 0` disables the limit:

```yaml
queue:
  queues:
    media:
      broker: media
      name: default
      max_pending: 1000
```

Producer runtimes that create tracker records should call `CapacityChecker`
before writing their tracker state. A capacity rejection is an admission-control
failure, so it should return to the caller without creating a job record.

The guard is intentionally best-effort because the queue length can change
between inspection and enqueue. It is meant to reject overload early and return
a typed error to the application/runtime, not to replace Redis memory limits or
infrastructure monitoring.

## Task Definition

Business domains should declare task metadata once:

```go
var DemoTask = queue.TaskDefinition[DemoCommand]{
    Type:        "export.demo",
    Description: "Demo export job",
    Queue:       "default",
    Mode:        queue.ModeQueue,
    Timeout:     10 * time.Minute,
    IdempotencyKey: func(cmd DemoCommand) string {
        return cmd.Name
    },
}
```

Runtime config is the final source of truth. `TaskDefinition.ApplyTaskConfig`
merges configured queue, mode, timeout, retention, and unique TTL over
the code defaults.

## Registry Dispatch

`Registry.Dispatch(ctx, task)` executes the registered handler in-process. The
Asynq server also dispatches through this method, so sync and async paths use
the same handler registration.

```go
err := registry.Dispatch(ctx, queue.Task{
    ID:      jobID,
    Type:    "export.demo",
    Queue:   "default",
    Payload: payload,
    Headers: map[string]string{"job_id": jobID},
})
```

## Worker App

Use `application.NewWorker(configPath, prefix, profile, flags)` for a
long-running worker process. The app should register domain providers in
`initDI()` and register handlers during `OnSetup`.

```go
func (a *HriseWorker) registerHandlers() error {
    return exportDo.RegisterJobs(a.Core.GetInjector())
}
```

Handler logic belongs in the owning domain package. The worker app only wires
DI and invokes domain registrars.

## Producer

Producer runtime should read the configured mode, run queue capacity preflight
for queue-mode tasks, create the tracker record only after admission succeeds,
and then either call `Registry.Dispatch` or `queue.Client.Enqueue`.
Task-specific publisher code should only pass the task definition and command to
that runtime.

```go
record, err := jobruntime.Publish(ctx, publisher, exportjob.DemoTask, exportjob.DemoCommand{
    Name: name,
})
```

Trace IDs are propagated through task headers using `x-trace-id`, then restored
to `context.Context` as `trace_id` before calling the handler.
