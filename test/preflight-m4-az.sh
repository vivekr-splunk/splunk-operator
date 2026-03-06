#!/bin/bash

set -euo pipefail

required_azs="${M4_REQUIRED_AZS:-3}"
preflight_mode="${M4_AZ_PREFLIGHT:-auto}" # auto|true|false
focus="${TEST_TO_RUN:-${TEST_FOCUS:-${TEST_REGEX:-}}}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "ERROR: kubectl is required for preflight checks."
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required for preflight checks."
  exit 1
fi

if [[ "${preflight_mode}" == "auto" ]]; then
  if echo "${focus}" | grep -Eiq '(^|[^a-z0-9])m4([^a-z0-9]|$)'; then
    preflight_mode="true"
  else
    preflight_mode="false"
  fi
fi

if [[ "${preflight_mode}" != "true" ]]; then
  echo "Skipping M4 AZ preflight (mode=${preflight_mode}, focus='${focus}')."
  exit 0
fi

if ! [[ "${required_azs}" =~ ^[0-9]+$ ]] || [[ "${required_azs}" -lt 1 ]]; then
  echo "ERROR: M4_REQUIRED_AZS must be a positive integer (got '${required_azs}')."
  exit 1
fi

zones="$(kubectl get nodes -o json | jq -r '
  .items[]
  | select(any(.status.conditions[]?; .type == "Ready" and .status == "True"))
  | (.metadata.labels["topology.kubernetes.io/zone"] // .metadata.labels["failure-domain.beta.kubernetes.io/zone"] // empty)
' | sed '/^$/d' | sort -u)"

zone_count="$(printf '%s\n' "${zones}" | sed '/^$/d' | wc -l | tr -d '[:space:]')"

if [[ "${zone_count}" -lt "${required_azs}" ]]; then
  echo "ERROR: M4 preflight failed: requires at least ${required_azs} unique Ready-node AZs, found ${zone_count}."
  if [[ -n "${zones}" ]]; then
    echo "Detected AZs: ${zones//$'\n'/, }"
  else
    echo "Detected AZs: <none>"
  fi
  echo "Ready-node zone labels:"
  kubectl get nodes \
    -o custom-columns=NAME:.metadata.name,READY:.status.conditions[-1].status,ZONE:.metadata.labels.topology\\.kubernetes\\.io/zone,ZONE_BETA:.metadata.labels.failure-domain\\.beta\\.kubernetes\\.io/zone
  exit 1
fi

echo "M4 AZ preflight passed: found ${zone_count} unique Ready-node AZ(s): ${zones//$'\n'/, }"
