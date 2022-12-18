#!/usr/bin/env bash

FILE_NAME="$(mktemp)"
cat << EOF > "${FILE_NAME}"
httpChecks:
  stat503:
    url: https://httpstat.us/503
    interval: 10s
  stat200:
    url: https://httpstat.us/200
    interval: 10s
    initialDelay: 2s
EOF

(go run main.go serve -c "${FILE_NAME}") &
# SRV_PID=$!

sleep 10
SRV_PID=$(lsof -nP | grep 'TCP \*:8080 (LISTEN)' | awk '{print $2}')

function fail() {
  echo "Error: ${1}"
  kill "${SRV_PID}"
  rm "${FILE_NAME}"
  exit 1
}

echo "-- TEST: status 200"
status="$(curl -s "http://localhost:8080/" | jq -r '."stat200-http".ok')"
if [[ "${status}" != "true" ]]; then
  fail "unexpected status: $status; wanted: true"
fi
echo -e "-- PASS\n"

echo "-- TEST: status 503"
status="$(curl -s "http://localhost:8080/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "Unexpected status code: '503' expected: '200'" ]]; then
  fail "unexpected status: $status; wanted: false"
fi
echo -e "-- PASS\n"

echo "-- TEST: delete status 503"
curl -s -X DELETE "http://localhost:8080/checks/http/stat503"
status="$(curl -s "http://localhost:8080/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "null" ]]; then
  fail "unexpected status: $status; wanted: null"
fi
echo -e "-- PASS\n"

echo "-- TEST: add status 504"
curl -s -X POST "http://localhost:8080/checks/http/stat503" -d '{"url": "https://httpstat.us/504", "interval": "5s"}'
sleep 1
status="$(curl -s "http://localhost:8080/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "Unexpected status code: '504' expected: '200'" ]]; then
  fail "unexpected status: $status; wanted: false"
fi
echo -e "-- PASS\n"

echo "-- TEST: update status 503"
curl -s -X POST "http://localhost:8080/checks/http/stat503" -d '{"url": "https://httpstat.us/503", "interval": "5s"}'
sleep 5
status="$(curl -s "http://localhost:8080/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "Unexpected status code: '503' expected: '200'" ]]; then
  fail "unexpected status: $status; wanted: false"
fi
echo -e "-- PASS\n"

kill "${SRV_PID}"
rm "${FILE_NAME}"
