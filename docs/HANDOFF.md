# Handoff (Remote Workspace Context)

This repo is the splunk-operator branch used to validate multi-container pods on EKS.

## What To Open In VSCode (Remote-SSH)
Open the workspace file:
- `/home/vivekr/multi-container-splunk/multi-container-splunk.code-workspace`

## Branch
- `vivek/e2e-multicontainer-harness`

## What Changed In This Branch
EKS-focused e2e improvements for multi-container pods:
- IRSA serviceAccount annotation wiring for EKS testenv
- `kubectl exec` default container fixes for multi-container
- safer S3 creds handling (avoid empty secrets; allow IAM auth)
- metrics bind collision avoidance
- related-image env plumbing for init/sidecar images

## Draft PR
- https://github.com/splunk/splunk-operator/pull/1739
