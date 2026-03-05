# Change Intent: fail-fast-critical-readiness

- **Status**: implemented
- **Created**: 2026-03-05
- **Owner**: Codex

## Request Context

Upstream multi-container pipeline waits on downstream operator integration tests. App-framework failures in `ClusterManagerReady` / `ClusterMasterReady` can run until the CI job timeout (6h), making root-cause analysis slow.

## Intent

Make critical readiness checks fail fast on terminal pod states and use a bounded readiness timeout so infrastructure/product failures surface in minutes, not hours.

## Scope

- In scope:
  - Harden `test/testenv/verificationutils.go` readiness checks for:
    - `ClusterManagerReady`
    - `ClusterMasterReady`
  - Add terminal pod-state detection (`CrashLoopBackOff`, `ErrImagePull`, etc.) and stop waiting immediately when detected.
  - Add bounded timeout for these readiness checks with optional env override.
  - Add unit tests covering timeout capping/override behavior and terminal pod-state detection.
- Out of scope:
  - Changing CR API/spec semantics.
  - Rewriting app-framework test scenarios.

## Test Plan

- `go test ./test/testenv -run TestDoesNotExist`
- `go test ./test/testenv`
- Re-run downstream `int-test-eks` / `int-test-aks` in pipeline and verify failures terminate with fail-fast signatures instead of hitting 6h timeout.

## Implementation Log

1. 2026-03-05T14:16:40Z: created plan doc before code changes.
2. 2026-03-05T14:19:00Z: added bounded timeout helper for critical readiness checks with optional `SPLUNK_OPERATOR_READY_CHECK_TIMEOUT_SECONDS` override.
3. 2026-03-05T14:20:00Z: added terminal pod-state fail-fast detection for `CrashLoopBackOff`/image pull/config/runtime terminal reasons.
4. 2026-03-05T14:21:00Z: refactored `ClusterManagerReady` and `ClusterMasterReady` to use `Eventually(func() error)` + `StopTrying` on terminal conditions.
5. 2026-03-05T14:22:00Z: added `test/testenv/verificationutils_test.go` to cover timeout cap/env override and fail-fast pod-state detection.
6. 2026-03-05T14:23:00Z: validated compile and package build with `go test ./test/testenv`, `go test ./test/testenv -run TestDoesNotExist`, and `go test ./... -run TestDoesNotExist`.
