#!/usr/bin/env bash

URL="http://localhost:8000/set"

CONCURRENCY=20

echo "=== Running $CONCURRENCY concurrent write requests to leader (8000) ==="

pids=()

for i in $(seq 1 $CONCURRENCY); do
(
    START=$(gdate +%s%3N)

    curl -s -X POST "$URL" \
      -H "Content-Type: application/json" \
      -d "{\"key\":\"k$i\", \"value\":\"v$i\"}" > /dev/null

    END=$(gdate +%s%3N)
    DIFF=$((END - START))
    echo "Write $i latency: ${DIFF} ms"
) &
pids+=($!)
done

# wait for all concurrent requests to complete
for pid in "${pids[@]}"; do
  wait $pid
done

echo "-------------------------------------------"
echo "All $CONCURRENCY concurrent write requests finished."
