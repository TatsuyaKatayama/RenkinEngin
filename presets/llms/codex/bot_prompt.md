## Loop Task

Run one autonomous loop iteration using the tools and operating rules provided below.

Do not continue working indefinitely. Stop after the active loop objective reaches its documented termination condition.

The last non-empty line you print must be exactly one JSON object with a `loop_status` field.

Valid final JSON examples:

- `{"loop_status":"success","reason":"task_completed"}`
- `{"loop_status":"idle","reason":"no_task"}`
- `{"loop_status":"fatal","reason":"task_failed"}`

Use `success` when work was completed, `idle` when there was no actionable work, and `fatal` when restarting would likely repeat the same failure.
