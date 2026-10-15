#!/usr/bin/env bash

set -euo pipefail

if [[ $# -gt 1 || ( $# -eq 1 && $1 != --check ) ]]; then
	printf 'Usage: %s [--check]\n' "$0" >&2
	exit 2
fi

source "$(dirname "${BASH_SOURCE[0]}")/backup-common.sh"
check_tools
check_deployments
if [[ ${1-} == --check ]]; then
	pod=$(postgres_pod)
	kube -n "$namespace" exec "$pod" -c postgres -- pg_dump --version
	kube -n "$namespace" exec "$pod" -c postgres -- pg_restore --version
	kube -n "$namespace" get service/garage secret/lumio-secrets -o name
	echo 'Backup prerequisites are ready'
	exit 0
fi

output="lumio-$(date -u +%Y%m%dT%H%M%SZ)"
if [[ -e $output || -L $output ]]; then
	printf 'Backup already exists: %s\n' "$output" >&2
	exit 1
fi

umask 077
tmp=$(mktemp -d "${output}.tmp.XXXXXXXX")
app_stopping=false
cleanup() {
	local status=$?
	trap - EXIT
	stop_storage_forward
	if [[ $app_stopping == true ]]; then
		start_app || status=1
	fi
	if [[ -d $tmp ]]; then
		rm -rf -- "$tmp"
	fi
	exit "$status"
}
trap cleanup EXIT

app_stopping=true
stop_app
pod=$(postgres_pod)
kube -n "$namespace" exec "$pod" -c postgres -- pg_dump -U lumio -d lumio -Fc > "$tmp/lumio.dump"
[[ -s $tmp/lumio.dump ]] || { echo 'Database dump is empty' >&2; exit 1; }
kube -n "$namespace" exec -i "$pod" -c postgres -- pg_restore -l < "$tmp/lumio.dump" > /dev/null

storage_credentials
start_storage_forward
aws --endpoint-url "http://127.0.0.1:$s3_port" s3api head-bucket --bucket lumio
mkdir "$tmp/objects"
aws --endpoint-url "http://127.0.0.1:$s3_port" s3 sync s3://lumio "$tmp/objects" --no-progress
stop_storage_forward
forward_pid=
forward_log=

(cd "$tmp" && {
	sha256sum lumio.dump
	find objects -type f -print0 | sort -z | xargs -0 -r sha256sum
} > SHA256SUMS)
printf 'Completed at %s UTC\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$tmp/COMPLETE"
mv -- "$tmp" "$output"
printf 'Saved backup to %s\n' "$output"
