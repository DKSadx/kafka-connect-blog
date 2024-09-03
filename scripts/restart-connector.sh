#!/usr/bin/env bash

config_path="${1}"
connector_name="$(jq -r .name "${config_path}")"

# Check status code of the connector
status_code="$(curl -s -o /dev/null -I -w "%{http_code}" http://localhost:8083/connectors/"${connector_name}"/status)"

# Create the snowflake connector if it does not exist. If the connector exists, update it with the same config aka restart it
if [[ ${status_code} -ne 200 ]]; then
  curl -X POST -H 'Content-Type: application/json' --data "@${config_path}" http://localhost:8083/connectors
else
  curl -X PUT -H 'Content-Type: application/json' --data "$(jq -r .config "${config_path}")" http://localhost:8083/connectors/"${connector_name}"/config
fi

