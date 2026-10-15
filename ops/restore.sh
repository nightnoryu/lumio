#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 2 || $1 != --yes ]]; then
	printf 'Usage: %s --yes BACKUP_DIRECTORY\n' "$0" >&2
	exit 2
fi

source "$(dirname "${BASH_SOURCE[0]}")/backup-common.sh"
check_tools
backup=$2
if [[ ! -f $backup/COMPLETE || ! -s $backup/lumio.dump || ! -d $backup/objects || ! -f $backup/SHA256SUMS ]]; then
	printf 'Incomplete backup: %s\n' "$backup" >&2
	exit 1
fi
(cd "$backup" && sha256sum -c SHA256SUMS)
check_deployments
pod=$(postgres_pod)
kube -n "$namespace" exec -i "$pod" -c postgres -- pg_restore -l < "$backup/lumio.dump" > /dev/null

umask 077
app_stopping=false
cleanup() {
	local status=$?
	trap - EXIT
	stop_storage_forward
	if [[ $app_stopping == true ]]; then
		kube -n "$namespace" scale deployment/lumio-web deployment/lumio-worker --replicas=0 >&2 || true
		echo 'Restore did not finish; web and worker remain scaled to zero for inspection' >&2
	fi
	exit "$status"
}
trap cleanup EXIT

storage_credentials
app_stopping=true
stop_app
start_storage_forward
aws --endpoint-url "http://127.0.0.1:$s3_port" s3api head-bucket --bucket lumio

# Restore both stores while the application is offline. --delete removes objects
# created after the snapshot, so the bucket matches the saved database.
kube -n "$namespace" exec "$pod" -c postgres -- dropdb -U lumio --if-exists --force lumio
kube -n "$namespace" exec "$pod" -c postgres -- createdb -U lumio lumio
kube -n "$namespace" exec -i "$pod" -c postgres -- pg_restore -U lumio -d lumio --no-owner --exit-on-error < "$backup/lumio.dump"
aws --endpoint-url "http://127.0.0.1:$s3_port" s3 sync "$backup/objects" s3://lumio --delete --no-progress
stop_storage_forward
forward_pid=
forward_log=
start_app
app_stopping=false
echo 'Database and objects restored; web and worker are ready'
