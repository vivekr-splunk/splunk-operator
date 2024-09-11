#!/usr/bin/env bash

NAMESPACE="splunk-operator"
DOMAIN="enterprise.splunk.com"

removeCRDs() {
	#DOMAIN=$1
	echo "Removing CRs with domain $DOMAIN"
	## get list of CRDS
	echo '$(kubectl api-resources --verbs=list --namespaced -o name | grep -iF "$DOMAIN")'
	CRDS=$(kubectl api-resources --verbs=list --namespaced -o name | grep -iF "$DOMAIN")
	for CRD in $CRDS
	do	
		echo "In CRD $CRD"
		## get list of namespaces these crs live
		if [[ $# -eq 0 ]]
  		then
			NAMESPACES=$(kubectl get $CRD -A -o jsonpath="{.items[*].metadata.namespace}")
		else
			NAMESPACES=$1
		fi	
		
		for NAMESPACE in $NAMESPACES
		do
			echo "try to create namespace $NAMESPACE incase no longer exist"
			kubectl create namespace $NAMESPACE 
			## get list of CRs in each namespace
			CRS=$(kubectl get -n $NAMESPACE $CRD | tail -n +2 | awk '{ print $1 }')
			for CR in $CRS
			do
				echo "Patch and Remove CR: $CRD $CR in namespace $NAMESPACE"
				kubectl patch $CRD $CR  -n $NAMESPACE --type="merge" -p '{"metadata": {"finalizers": null}}' -o yaml > /dev/null 2>&1
				kubectl delete $CRD $CR  -n $NAMESPACE --timeout 20s || true
				kubectl delete pvc -n $NAMESPACE --all --timeout 20s || true
			done
			# delete namespace
			kubectl delete namespace $NAMESPACE --timeout 5s || true
		done
	done
}

removeCRDs $@