# Change Note: App Framework Manual-Update ConfigMap Conflict Retry

Date: `2026-03-06`  
Area: `pkg/splunk/enterprise`  
Scope: App Framework manual poll flow (no CR spec change)

## Problem

During GCP app-framework manual-poll runs, `Standalone` and `MonitoringConsole` reconciles can update the shared namespace ConfigMap `splunk-<namespace>-manual-app-update` at nearly the same time.

Observed failure signature:

- `Operation cannot be fulfilled on configmaps "<...>-manual-app-update": the object has been modified`
- error bubbled from `updateManualAppUpdateConfigMapLocked` through `initAndCheckAppInfoStatus`
- `Standalone` phase became `Error`, causing test assertion failure (`expected Ready`)

## Root Cause

`updateManualAppUpdateConfigMapLocked` performed a single update attempt and returned conflict errors directly. A transient resourceVersion conflict (`409`) was treated as terminal for that reconcile path.

## Change

Updated `updateManualAppUpdateConfigMapLocked` to:

- retry ConfigMap update on `IsConflict(err)` with fresh reads
- keep non-conflict errors as terminal
- fail only after bounded retries

Also added unit coverage that injects one update conflict and verifies the method succeeds after retry.

## Validation

- Unit:
  - `go test ./pkg/splunk/enterprise -run TestUpdateManualAppUpdateConfigMapLocked -count=1`
  - `go test ./pkg/splunk/enterprise -count=1`
- Integration (focused failing spec rerun on GCP):
  - `test/appframework_gcp/s1` focused spec:
    - `s1gcp, smokegcp, appframeworkgcp: can deploy a standalone instance with App Framework enabled for manual poll`
  - Result: `PASS` (`1 Passed, 0 Failed`)
  - JUnit artifact: `rerun-manual-poll-junit-fix.xml`

## Notes

- No API/CRD/OpenAPI change.
- No Splunk CR spec change.
- This is a reconcile robustness fix for transient Kubernetes update conflicts.
