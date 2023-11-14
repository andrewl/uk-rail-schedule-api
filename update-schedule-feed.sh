#!/bin/bash
#
# This script downloads the latest schedule feed from Network Rail and refreshes
# the database with the new data.
# It is intended to be run as a cron job.
# It requires the following environment variables to be set:
# - UKRA_USERNAME
# - UKRA_PASSWORD

# if username or password are empty, exit
if [ -z "$UKRA_USERNAME" ] || [ -z "$UKRA_PASSWORD" ]; then
  echo "UKRA_USERNAME or UKRA_PASSWORD not set"
  exit 1
fi

curl -L -u "$UKRA_USERNAME:$UKRA_PASSWORD" -o schedule.json.gz 'https://publicdatafeeds.networkrail.co.uk/ntrod/CifFileAuthenticate?type=CIF_ALL_FULL_DAILY&day=toc-full'

# if curl failed to download the file then exit
if [ $? -ne 0 ]; then
  echo "curl failed to download the file"
  exit 1
fi

gunzip -f schedule.json.gz

# if gunzip failed to unzip the file then exit
if [ $? -ne 0 ]; then
  echo "gunzip failed to unzip the file"
  exit 1
fi

curl 127.0.0.1:3333/refresh

# if curl failed to refresh the database then exit
if [ $? -ne 0 ]; then
  echo "curl failed to refresh the database"
  exit 1
fi
