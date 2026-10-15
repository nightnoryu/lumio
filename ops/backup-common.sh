#!/usr/bin/env bash

namespace=${KUBE_NAMESPACE:-lumio}
s3_port=${LUMIO_BACKUP_S3_PORT:-13900}
remote_s3_port=${LUMIO_BACKUP_REMOTE_S3_PORT:-23900}
ssh_host=${KUBE_SSH_HOST:-}
forward_pid=
forward_log=

kube() {
	if [[ -z $ssh_host ]]; then
		kubectl "$@"
	else
		local remote_command
		printf -v remote_command '%q ' kubectl "$@"
		ssh -o BatchMode=yes "$ssh_host" "$remote_command"
	fi
}

check_tools() {
	local tool
	for tool in aws base64 sha256sum; do
		command -v "$tool" >/dev/null || { printf '%s is required\n' "$tool" >&2; exit 1; }
	done
	if [[ -n $ssh_host ]]; then
		command -v ssh >/dev/null || { echo 'ssh is required' >&2; exit 1; }
	else
		command -v kubectl >/dev/null || { echo 'kubectl is required' >&2; exit 1; }
	fi
	for port in "$s3_port" "$remote_s3_port"; do
		if [[ ! $port =~ ^[0-9]+$ || $port -lt 1 || $port -gt 65535 ]]; then
			echo 'Garage port settings must be TCP ports' >&2
			exit 2
		fi
	done
}

check_deployments() {
	local deployment replicas
	for deployment in lumio-web lumio-worker; do
		replicas=$(kube -n "$namespace" get "deployment/$deployment" -o jsonpath='{.spec.replicas}')
		if [[ $replicas != 1 ]]; then
			printf 'Expected deployment/%s to have one replica\n' "$deployment" >&2
			exit 1
		fi
	done
	kube -n "$namespace" rollout status deployment/postgres --timeout=120s
	kube -n "$namespace" rollout status deployment/garage --timeout=120s
}

stop_app() {
	kube -n "$namespace" scale deployment/lumio-web deployment/lumio-worker --replicas=0
	local attempt pods
	for ((attempt = 0; attempt < 60; attempt++)); do
		pods=$(kube -n "$namespace" get pods -l 'app in (lumio-web,lumio-worker)' -o name)
		[[ -z $pods ]] && return 0
		sleep 2
	done
	echo 'Timed out waiting for web and worker pods to stop' >&2
	return 1
}

start_app() {
	kube -n "$namespace" scale deployment/lumio-web deployment/lumio-worker --replicas=1
	kube -n "$namespace" rollout status deployment/lumio-web --timeout=300s
	kube -n "$namespace" rollout status deployment/lumio-worker --timeout=300s
}

postgres_pod() {
	local pods
	pods=$(kube -n "$namespace" get pods -l app=postgres --field-selector=status.phase=Running -o name)
	if [[ $(printf '%s\n' "$pods" | sed '/^$/d' | wc -l) -ne 1 ]]; then
		echo 'Expected exactly one running PostgreSQL pod' >&2
		return 1
	fi
	printf '%s\n' "$pods"
}

start_storage_forward() {
	forward_log=$(mktemp)
	if [[ -z $ssh_host ]]; then
		kubectl -n "$namespace" port-forward --address 127.0.0.1 service/garage "$s3_port:3900" >"$forward_log" 2>&1 &
	else
		local remote_command
		printf -v remote_command '%q ' kubectl -n "$namespace" port-forward --address 127.0.0.1 service/garage "$remote_s3_port:3900"
		ssh -o BatchMode=yes -o ExitOnForwardFailure=yes \
			-L "127.0.0.1:$s3_port:127.0.0.1:$remote_s3_port" \
			"$ssh_host" "$remote_command" >"$forward_log" 2>&1 &
	fi
	forward_pid=$!
	local attempt
	for ((attempt = 0; attempt < 30; attempt++)); do
		if grep -q 'Forwarding from' "$forward_log"; then
			return 0
		fi
		if ! kill -0 "$forward_pid" 2>/dev/null; then
			break
		fi
		sleep 1
	done
	cat "$forward_log" >&2
	echo 'Could not start Garage port forward' >&2
	return 1
}

storage_credentials() {
	AWS_ACCESS_KEY_ID=$(kube -n "$namespace" get secret lumio-secrets -o jsonpath='{.data.LUMIO_S3_ACCESS_KEY}' | base64 -d)
	AWS_SECRET_ACCESS_KEY=$(kube -n "$namespace" get secret lumio-secrets -o jsonpath='{.data.LUMIO_S3_SECRET_KEY}' | base64 -d)
	[[ -n $AWS_ACCESS_KEY_ID && -n $AWS_SECRET_ACCESS_KEY ]] || { echo 'Garage credentials are missing' >&2; return 1; }
	export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_DEFAULT_REGION=garage AWS_EC2_METADATA_DISABLED=true
}

stop_storage_forward() {
	if [[ -n $forward_pid ]]; then
		kill "$forward_pid" 2>/dev/null || true
		wait "$forward_pid" 2>/dev/null || true
	fi
	if [[ -n $forward_log ]]; then
		rm -f "$forward_log"
	fi
}
