#!/bin/bash
#
# This script downloads the latest schedule feed from Network Rail and refreshes
# the database with the new data.
# It is intended to be run as a cron job.
#
# Required environment variables:
#   UKRA_USERNAME     - Network Rail open data portal username
#   UKRA_PASSWORD     - Network Rail open data portal password
#
# Optional environment variables:
#   SCHEDULE_FEED_FILENAME  - path to write the decompressed feed
#                             (default: ./data/schedule.json)
#   LISTEN_ON               - host:port of the running web service
#                             (default: localhost:1333)

set -euo pipefail

log() {
  echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] $*"
}

# Change to the directory of the script so relative paths work correctly
cd "$(dirname "$0")"

if [ -z "${UKRA_USERNAME:-}" ] || [ -z "${UKRA_PASSWORD:-}" ]; then
  log "ERROR: UKRA_USERNAME or UKRA_PASSWORD not set"
  exit 1
fi

FEED_FILE="${SCHEDULE_FEED_FILENAME:-./data/schedule.json}"
WEB_ADDR="${LISTEN_ON:-localhost:1333}"
FEED_DIR="$(dirname "$FEED_FILE")"
TMP_GZ="$(mktemp "${FEED_DIR}/.schedule.json.gz.XXXXXX")"
TMP_JSON="$(mktemp "${FEED_DIR}/.schedule.json.XXXXXX")"

cleanup() {
  rm -f "$TMP_GZ" "$TMP_JSON"
}
trap cleanup EXIT

log "Downloading schedule feed to ${FEED_FILE}..."
if ! curl --silent --show-error --fail --location \
     -u "${UKRA_USERNAME}:${UKRA_PASSWORD}" \
     -o "$TMP_GZ" \
     'https://publicdatafeeds.networkrail.co.uk/ntrod/CifFileAuthenticate?type=CIF_ALL_FULL_DAILY&day=toc-full'; then
  log "ERROR: Failed to download schedule feed"
  exit 1
fi

log "Decompressing feed..."
if ! gunzip --stdout "$TMP_GZ" > "$TMP_JSON"; then
  log "ERROR: Failed to decompress schedule feed"
  exit 1
fi

# Copy over the feed file. An atomic mv is not possible when the destination
# is a Docker bind-mounted file (single-file mounts fix the inode), so we
# use cp instead. The refresh API is called only after this completes, so the
# service never reads a partial file.
cp "$TMP_JSON" "$FEED_FILE"
log "Feed written to ${FEED_FILE}"

log "Triggering database refresh..."
if ! curl --silent --show-error --fail "http://${WEB_ADDR}/api/refresh"; then
  log "ERROR: Failed to trigger database refresh"
  exit 1
fi

log "Schedule feed update complete"
