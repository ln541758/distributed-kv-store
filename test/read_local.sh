#!/usr/bin/env bash

URL="http://localhost:8001/get"

CONCURRENCY=20

echo "=== Running $CONCURRENCY concurrent read requests to follower (8001) ==="

pids=()
for i in $(seq 1 $CONCURRENCY); do
  (
    START=$(gdate +%s%3N)

    curl -s -X GET "$URL?key=k1" > /dev/null

    END=$(gdate +%s%3N)
    DIFF=$((END - START))
    echo "Read $i latency: ${DIFF} ms"
  ) &

  pids+=($!)
done

# wait for all concurrent requests to complete
for pid in "${pids[@]}"; do
  wait $pid
done

echo "-------------------------------------------"
echo "All $CONCURRENCY concurrent requests finished."
