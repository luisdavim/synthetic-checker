#!/usr/bin/env bash

SERVER_CFG="$(mktemp)"
cat << EOF > "${SERVER_CFG}"
httpChecks:
  stat503:
    url: https://httpstat.us/503
    interval: 10s
  stat200:
    url: https://httpstat.us/200
    interval: 10s
    initialDelay: 2s
EOF

SRV_URL='http://localhost:8080'

(go run main.go serve -c "${SERVER_CFG}") &
# SRV_PID=$!

sleep 10
SRV_PID=$(lsof -tnPi TCP:8080)

function fail() {
  kill "${SRV_PID}"
  rm "${SERVER_CFG}"
  echo "-- FAIL: ${1}"
  exit 1
}

echo "-- TEST: status 200"
status="$(curl -s "${SRV_URL}/" | jq -r '."stat200-http".ok')"
if [[ "${status}" != "true" ]]; then
  fail "unexpected status: $status; wanted: true"
fi
echo -e "-- PASS\n"

echo "-- TEST: status 503"
status="$(curl -s "${SRV_URL}/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "Unexpected status code: '503' expected: '200'" ]]; then
  fail "unexpected status: $status; wanted: 503"
fi
echo -e "-- PASS\n"

echo "-- TEST: delete status 503"
curl -fs -X DELETE "${SRV_URL}/checks/http/stat503"
status="$(curl -s "${SRV_URL}/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "null" ]]; then
  fail "unexpected status: $status; wanted: null"
fi
echo -e "-- PASS\n"

echo "-- TEST: add status 504"
curl -fs -X POST "${SRV_URL}/checks/http/stat503" -d '{"url": "https://httpstat.us/504", "interval": "5s"}'
sleep 1
status="$(curl -s "${SRV_URL}/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "Unexpected status code: '504' expected: '200'" ]]; then
  fail "unexpected status: $status; wanted: 504"
fi
echo -e "-- PASS\n"

echo "-- TEST: update status 503"
curl -fs -X POST "${SRV_URL}/checks/http/stat503" -d '{"url": "https://httpstat.us/503", "interval": "5s"}'
sleep 6
status="$(curl -s "${SRV_URL}/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "Unexpected status code: '503' expected: '200'" ]]; then
  fail "unexpected status: $status; wanted: 503"
fi
echo -e "-- PASS\n"

echo "-- TEST: delete status 503 by name"
curl -fs -X DELETE "${SRV_URL}/checks/stat503-http"
status="$(curl -s "${SRV_URL}/" | jq -r '."stat503-http".error')"
if [[ "${status}" != "null" ]]; then
  fail "unexpected status: $status; wanted: null"
fi
echo -e "-- PASS\n"

echo -e "\n-- INFORMER TESTs --\n"

INFORMER_CFG="$(mktemp)"
cat << EOF > "${INFORMER_CFG}"
informer:
  informOnly: true
  upstreams:
    - url: http://127.0.0.1:8080
EOF

INFORMER_URL='http://localhost:8081'

(go run main.go serve -c "${INFORMER_CFG}" -p 8081) &
# INFORMER_PID=$!

sleep 10
INFORMER_PID=$(lsof -tnPi TCP:8081)

function fail2() {
  kill "${INFORMER_PID}"
  rm "${INFORMER_CFG}"
  fail "${1}"
}

echo "-- TEST: add status 404 to informer"
curl -fs -X POST "${INFORMER_URL}/checks/http/stat404" -d '{"url": "https://httpstat.us/404", "interval": "5s"}'
sleep 1
status="$(curl -s "${SRV_URL}/" | jq -r '."stat404-http".error')"
if [[ "${status}" != "Unexpected status code: '404' expected: '200'" ]]; then
  fail2 "unexpected status: $status; wanted: 404"
fi
echo -e "-- PASS\n"

echo "-- TEST: delete status 404 from informer"
curl -fs -X DELETE "${INFORMER_URL}/checks/http/stat404"
sleep 1
status="$(curl -s "${SRV_URL}/" | jq -r '."stat404-http".error')"
if [[ "${status}" != "null" ]]; then
  fail "unexpected status: $status; wanted: null"
fi
echo -e "-- PASS\n"

kill "${INFORMER_PID}"
rm "${INFORMER_CFG}"
kill "${SRV_PID}"
rm "${SERVER_CFG}"
