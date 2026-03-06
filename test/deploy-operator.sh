#!/bin/bash

scriptdir=$(dirname "$0")
topdir=${scriptdir}/..

source ${scriptdir}/env.sh

# Allow callers (CI/dev) to select which kustomize overlay to use for operator deployment.
# Default keeps legacy behavior.
: "${OPERATOR_ENVIRONMENT:=debug}"
: "${OPERATOR_NAMESPACE:=splunk-operator}"

wait_for_enterprise_crd_deletion() {
  local timeout_seconds="${CRD_DELETE_TIMEOUT_SECONDS:-900}"
  local deadline=$((SECONDS + timeout_seconds))
  local deleting_crds=()

  while true; do
    mapfile -t deleting_crds < <(kubectl get crd -o custom-columns=NAME:.metadata.name,DELETION:.metadata.deletionTimestamp --no-headers 2>/dev/null | awk '$1 ~ /\.enterprise\.splunk\.com$/ && $2 != "<none>" {print $1}')
    if [ "${#deleting_crds[@]}" -eq 0 ]; then
      return 0
    fi
    if [ "${SECONDS}" -ge "${deadline}" ]; then
      echo "Timed out waiting for enterprise CRD deletion to finish."
      printf '  %s\n' "${deleting_crds[@]}"
      return 1
    fi
    echo "Waiting for enterprise CRDs to finish deleting: ${deleting_crds[*]}"
    sleep 5
  done
}

cleanup_enterprise_custom_resources() {
  local resources=(
    standalones.enterprise.splunk.com
    clustermanagers.enterprise.splunk.com
    clustermasters.enterprise.splunk.com
    indexerclusters.enterprise.splunk.com
    ingestorclusters.enterprise.splunk.com
    licensemanagers.enterprise.splunk.com
    licensemasters.enterprise.splunk.com
    monitoringconsoles.enterprise.splunk.com
    objectstorages.enterprise.splunk.com
    queues.enterprise.splunk.com
    searchheadclusters.enterprise.splunk.com
  )
  local resource
  local objects
  local ns
  local name

  echo "Cleaning up existing enterprise custom resources..."
  for resource in "${resources[@]}"; do
    if ! objects="$(kubectl get "${resource}" -A -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{"\n"}{end}' 2>/dev/null)"; then
      continue
    fi
    if [ -z "${objects}" ]; then
      continue
    fi

    while IFS=' ' read -r ns name; do
      [ -n "${ns}" ] || continue
      [ -n "${name}" ] || continue
      if ! kubectl get namespace "${ns}" >/dev/null 2>&1; then
        echo "Re-creating missing namespace ${ns} to clear orphan ${resource}/${name}"
        kubectl create namespace "${ns}" >/dev/null 2>&1 || true
      fi
      kubectl patch "${resource}" "${name}" -n "${ns}" --type=merge -p '{"metadata":{"finalizers":[]}}' >/dev/null 2>&1 || true
      kubectl delete "${resource}" "${name}" -n "${ns}" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
    done <<< "${objects}"
  done
}

remove_cluster_scoped_operator_if_present() {
  echo "Ensuring no cluster-scoped operator is running in ${OPERATOR_NAMESPACE}..."
  kubectl -n "${OPERATOR_NAMESPACE}" delete deployment splunk-operator-controller-manager --ignore-not-found=true --wait=true >/dev/null 2>&1 || true
  kubectl -n "${OPERATOR_NAMESPACE}" delete pod -l control-plane=controller-manager --ignore-not-found=true --wait=true >/dev/null 2>&1 || true
}

# Check if exactly 2 arguments are supplied
if [ "$#" -ne 2 ]; then
  echo "Error: Exactly 2 arguments are required."
  echo "Usage: $0 <PRIVATE_SPLUNK_OPERATOR_IMAGE> <PRIVATE_SPLUNK_ENTERPRISE_IMAGE>"
  exit 1
fi

# Assign arguments to variables
PRIVATE_SPLUNK_OPERATOR_IMAGE="$1"
PRIVATE_SPLUNK_ENTERPRISE_IMAGE="$2"

if [  "${DEPLOYMENT_TYPE}" == "helm" ]; then
  echo "Installing Splunk Operator using Helm charts"
  helm uninstall splunk-operator -n splunk-operator
  # Install the CRDs
  echo "Installing enterprise CRDs..."
  make kustomize
  cleanup_enterprise_custom_resources
  make uninstall
  wait_for_enterprise_crd_deletion || exit 1
  make install
  if [ "${CLUSTER_WIDE}" != "true" ]; then
    helm install splunk-operator --create-namespace --namespace splunk-operator --set splunkOperator.clusterWideAccess=false --set splunkOperator.image.repository=${PRIVATE_SPLUNK_OPERATOR_IMAGE} --set image.repository=${PRIVATE_SPLUNK_ENTERPRISE_IMAGE} --set splunkOperator.splunkGeneralTerms="--accept-sgt-current-at-splunk-com" helm-chart/splunk-operator
  else
    helm install splunk-operator --create-namespace --namespace splunk-operator --set splunkOperator.image.repository=${PRIVATE_SPLUNK_OPERATOR_IMAGE} --set image.repository=${PRIVATE_SPLUNK_ENTERPRISE_IMAGE} --set splunkOperator.splunkGeneralTerms="--accept-sgt-current-at-splunk-com" helm-chart/splunk-operator
  fi
elif [  "${CLUSTER_WIDE}" != "true" ]; then
  # Install the CRDs
  echo "Installing enterprise CRDs..."
  make kustomize
  cleanup_enterprise_custom_resources
  make uninstall
  wait_for_enterprise_crd_deletion || exit 1
  bin/kustomize build config/crd | kubectl apply --server-side --force-conflicts -f -
  remove_cluster_scoped_operator_if_present
else
  echo "Installing enterprise operator from ${PRIVATE_SPLUNK_OPERATOR_IMAGE} using enterprise image from ${PRIVATE_SPLUNK_ENTERPRISE_IMAGE}..."
  # Re-running cluster-wide tests can leave a bound app-download PVC whose storageClass
  # is immutable. Remove old operator resources first so deploy does not fail on PVC patch.
  kubectl -n "${OPERATOR_NAMESPACE}" delete deployment splunk-operator-controller-manager --ignore-not-found=true --wait=true || true
  kubectl -n "${OPERATOR_NAMESPACE}" delete pvc splunk-operator-app-download --ignore-not-found=true --wait=true || true
  make deploy IMG=${PRIVATE_SPLUNK_OPERATOR_IMAGE} NAMESPACE=${OPERATOR_NAMESPACE} SPLUNK_ENTERPRISE_IMAGE=${PRIVATE_SPLUNK_ENTERPRISE_IMAGE} SPLUNK_GENERAL_TERMS="--accept-sgt-current-at-splunk-com" WATCH_NAMESPACE="" ENVIRONMENT=${OPERATOR_ENVIRONMENT} SPLUNK_POD_ARCH=${SPLUNK_POD_ARCH} RELATED_IMAGE_SPLUNK_INIT=${RELATED_IMAGE_SPLUNK_INIT} RELATED_IMAGE_SPLUNK_SIDECAR=${RELATED_IMAGE_SPLUNK_SIDECAR}
fi

if [ $? -ne 0 ]; then
  echo "Unable to install the operator. Exiting..."
  kubectl describe pod -n "${OPERATOR_NAMESPACE}"
  exit 1
fi

echo "Dumping operator config here..."
kubectl describe deployment splunk-operator-controller-manager -n "${OPERATOR_NAMESPACE}"


if [  "${CLUSTER_WIDE}" == "true" ]; then
  echo "wait for operator pod to be ready..."
  # sleep before checking for deployment, in slow clusters deployment call may not even started
  # in those cases, kubectl will fail with error:  no matching resources found
  sleep 2
  kubectl wait --for=condition=ready pod -l control-plane=controller-manager --timeout=600s -n "${OPERATOR_NAMESPACE}"
  if [ $? -ne 0 ]; then
    echo "kubectl get pods -n kube-system ---"
    kubectl get pods -n kube-system
    echo "kubectl get deployement ebs-csi-controller -n kube-system ---"
    kubectl get deployement ebs-csi-controller -n kube-system
    echo "kubectl describe pvc -n splunk-operator ---"
    kubectl describe pvc -n "${OPERATOR_NAMESPACE}"
    echo "kubectl describe pv ---"
    kubectl describe pv
    echo "kubectl describe pod -n splunk-operator ---"
    kubectl describe pod -n "${OPERATOR_NAMESPACE}"
    echo "Operator installation not ready..."
    exit 1
  fi
fi
