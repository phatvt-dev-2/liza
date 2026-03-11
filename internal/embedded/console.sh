#!/bin/bash
# Display a structure view of the .liza/state.yaml blackboard
# Usage: watch -n 2 'scripts/console.sh'

liza get tasks --format table
echo ; echo ====== Agents ======
liza get agents --format table
echo ; echo ====== Metrics ======
liza get metrics
echo ; echo ====== Anomalies ======
liza get anomalies
echo ; echo ====== Logs ======
yq '.[] | [.timestamp, .agent, .action, (.task // "-"), (.detail // "-")] | @tsv' .liza/log.yaml |
  awk -F'\t' '{ts=$1; sub(/T/," ",ts); sub(/\.[0-9]+Z$/,"",ts); sub(/Z$/,"",ts); d=substr($5,1,80); printf "%s | %-15s | %-20s | %-25s | %s\n", ts, $2, $3, $4, d}'
