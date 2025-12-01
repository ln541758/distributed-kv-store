#!/usr/bin/env bash

LEADER_IP="54.187.84.33"
URL="http://$LEADER_IP:8000/get/k1"

CONCURRENCY=20

echo "=== Running $CONCURRENCY concurrent read requests to $LEADER_IP:8000 ==="

START_ALL=$(gdate +%s%3N)

pids=()
for i in $(seq 1 $CONCURRENCY); do
  (
    START=$(gdate +%s%3N)
    curl -s "$URL" > /dev/null
    END=$(gdate +%s%3N)
    echo "Request $i latency: $((END-START)) ms"
  ) &
  pids+=($!)
done

# wait for all concurrent requests to complete
for pid in "${pids[@]}"; do
  wait $pid
done

END_ALL=$(gdate +%s%3N)

echo "-------------------------------------------"
echo "Total time for $CONCURRENCY concurrent requests: $((END_ALL - START_ALL)) ms"
