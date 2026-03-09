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
