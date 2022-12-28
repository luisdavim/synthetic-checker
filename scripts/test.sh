#!/usr/bin/env bash

CFG_FILE="$(mktemp)"
cat << EOF > "${CFG_FILE}"
httpChecks:
  stat503:
    url: https://httpstat.us/503
    interval: 10s
  stat200:
    url: https://httpstat.us/200
    interval: 10s
    initialDelay: 2s
EOF

(go run main.go serve -c "${CFG_FILE}") &
# SRV_PID=$!

sleep 10
SRV_PID=$(lsof -tnPi TCP:8080)

function fail() {
  kill "${SRV_PID}"
  rm "${CFG_FILE}"
  echo "Error: ${1}"
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
curl -fs -X DELETE "http://localhost:8080/checks/http/stat503"
status="$(curl -s "http://localhost:8080/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "null" ]]; then
  fail "unexpected status: $status; wanted: null"
fi
echo -e "-- PASS\n"

echo "-- TEST: add status 504"
curl -fs -X POST "http://localhost:8080/checks/http/stat503" -d '{"url": "https://httpstat.us/504", "interval": "5s"}'
sleep 1
status="$(curl -s "http://localhost:8080/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "Unexpected status code: '504' expected: '200'" ]]; then
  fail "unexpected status: $status; wanted: false"
fi
echo -e "-- PASS\n"

echo "-- TEST: update status 503"
curl -fs -X POST "http://localhost:8080/checks/http/stat503" -d '{"url": "https://httpstat.us/503", "interval": "5s"}'
sleep 5
status="$(curl -s "http://localhost:8080/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "Unexpected status code: '503' expected: '200'" ]]; then
  fail "unexpected status: $status; wanted: false"
fi
echo -e "-- PASS\n"

echo "-- TEST: delete status 503 by name"
curl -fs -X DELETE "http://localhost:8080/checks/stat503-http"
status="$(curl -s "http://localhost:8080/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "null" ]]; then
  fail "unexpected status: $status; wanted: null"
fi
echo -e "-- PASS\n"

CFG_FILE2="$(mktemp)"
cat << EOF > "${CFG_FILE2}"
informer:
  informOnly: true # when set to true, will prevent the checks from being executed in the local instance
  upstreams:
    - url: http://127.0.0.1:8080
EOF

(go run main.go serve -c "${CFG_FILE2}" -p 8081) &
# SRV_PID=$!

sleep 10
SRV_PID2=$(lsof -tnPi TCP:8081)

function fail2() {
  kill "${SRV_PID2}"
  rm "${CFG_FILE2}"
  fail "${1}"
}

echo "-- TEST: add status 404 to informer"
curl -fs -X POST "http://localhost:8081/checks/http/stat404" -d '{"url": "https://httpstat.us/404", "interval": "5s"}'
sleep 1
status="$(curl -s "http://localhost:8080/" | jq -r '."stat404-http".error')"
if [[ "${status}" != "Unexpected status code: '404' expected: '200'" ]]; then
  fail2 "unexpected status: $status; wanted: false"
fi
echo -e "-- PASS\n"

kill "${SRV_PID2}"
rm "${CFG_FILE2}"
kill "${SRV_PID}"
rm "${CFG_FILE}"
