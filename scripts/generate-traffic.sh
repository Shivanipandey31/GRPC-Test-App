#!/usr/bin/env bash
# generate-traffic.sh – Drive grpc-dedup-test to produce 11 unique test cases.
#
# Expected result after Keploy static dedup:
#
#   TC-1   GetItem      (id=1,2,3 – same structure, different scalars)  → 1 TC
#   TC-2   GetWidget    mode=full    (config nested field IS on wire)    → 1 TC
#   TC-3   GetWidget    mode=minimal (config absent from wire)           → 1 TC
#   TC-4   RiskyCall    should_fail=false  grpc-status=0                → 1 TC
#   TC-5   RiskyCall    should_fail=true   grpc-status=3                → 1 TC
#   TC-6   GetReport    (Q1,Q2,Q3 – same deep structure)                → 1 TC
#   TC-7   GetVariant   type=text   (oneof → field 1, wire type 2)      → 1 TC
#   TC-8   GetVariant   type=number (oneof → field 2, wire type 0)      → 1 TC
#   TC-9   GetVariant   type=flag   (oneof → field 3, wire type 0)      → 1 TC
#   TC-10  ListItems    include=true  (repeated field 1 present)        → 1 TC
#   TC-11  ListItems    include=false (repeated field 1 absent)         → 1 TC
#
#   TOTAL calls: 28   Dropped by dedup: 17   Kept: 11

set -euo pipefail

HOST="${1:-localhost:50051}"
PROTO_DIR="/home/shivani/grpc-dedup-test/proto"
GRPCURL="${GRPCURL:-grpcurl}"
G="$GRPCURL -plaintext -import-path $PROTO_DIR -proto dedup.proto"

ok()  { echo "    ✓ $*"; }
hdr() { echo; echo "── $* ──"; }
call() {
  local method="$1" data="$2"
  $G -d "$data" "$HOST" "deduptest.DedupService/$method" 2>&1 | head -6
}
call_silent() {
  local method="$1" data="$2"
  $G -d "$data" "$HOST" "deduptest.DedupService/$method" > /dev/null 2>&1 || true
}

# ── Group A ──────────────────────────────────────────────────────────────────
hdr "Group A: GetItem – same structure, different scalars → 1 TC"
# 3 different ids, 2 extra duplicates – all must dedup to 1
for id in 1 2 3; do
  call GetItem "{\"id\": $id}"; ok "id=$id (first call)"
done
call_silent GetItem '{"id": 1}'; ok "id=1 duplicate (dropped)"
call_silent GetItem '{"id": 2}'; ok "id=2 duplicate (dropped)"

# ── Group B ──────────────────────────────────────────────────────────────────
hdr "Group B: GetWidget – optional nested field present vs absent → 2 TCs"
call GetWidget '{"mode": "full"}';    ok "full (config nested present)"
call GetWidget '{"mode": "minimal"}'; ok "minimal (config absent from wire)"
call_silent GetWidget '{"mode": "full"}';    ok "full duplicate (dropped)"
call_silent GetWidget '{"mode": "minimal"}'; ok "minimal duplicate (dropped)"

# ── Group C ──────────────────────────────────────────────────────────────────
hdr "Group C: RiskyCall – different grpc-status in trailer → 2 TCs"
call RiskyCall '{"should_fail": false}'; ok "grpc-status=0 (OK)"
$G -d '{"should_fail": true}' "$HOST" "deduptest.DedupService/RiskyCall" 2>&1 | head -4 || true
ok "grpc-status=3 (InvalidArgument, error expected)"
call_silent RiskyCall '{"should_fail": false}'; ok "success duplicate (dropped)"
$G -d '{"should_fail": true}' "$HOST" "deduptest.DedupService/RiskyCall" > /dev/null 2>&1 || true
ok "error duplicate (dropped)"

# ── Group D ──────────────────────────────────────────────────────────────────
hdr "Group D: GetReport – deeply nested (Report→Summary→Meta→Env+map, repeated Section→Row) → 1 TC"
for period in 2024-Q1 2024-Q2 2024-Q3; do
  call GetReport "{\"period\": \"$period\"}"; ok "period=$period"
done

# ── Group E ──────────────────────────────────────────────────────────────────
hdr "Group E: GetVariant – oneof puts different field numbers on wire → 3 TCs"
for t in text number flag; do
  call GetVariant "{\"type\": \"$t\"}"; ok "type=$t"
done
for t in text number flag; do
  call_silent GetVariant "{\"type\": \"$t\"}"; ok "$t duplicate (dropped)"
done

# ── Group F ──────────────────────────────────────────────────────────────────
hdr "Group F: ListItems – non-empty vs empty repeated field → 2 TCs"
call ListItems '{"include_items": true}';  ok "with items (field 1 on wire)"
call ListItems '{"include_items": false}'; ok "empty list (field 1 absent from wire)"
call_silent ListItems '{"include_items": true}';  ok "with-items duplicate (dropped)"
call_silent ListItems '{"include_items": false}'; ok "empty duplicate (dropped)"

echo
echo "══════════════════════════════════════════════════"
echo "  Done.  28 calls sent."
echo "  Expected unique test cases after dedup: 11"
echo "    Group A (GetItem):    3 calls  → 1 TC"
echo "    Group B (GetWidget):  2 calls  → 2 TCs"
echo "    Group C (RiskyCall):  2 calls  → 2 TCs"
echo "    Group D (GetReport):  3 calls  → 1 TC"
echo "    Group E (GetVariant): 3 calls  → 3 TCs"
echo "    Group F (ListItems):  2 calls  → 2 TCs"
echo "══════════════════════════════════════════════════"
