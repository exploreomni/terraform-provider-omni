#!/usr/bin/env bash
# Drives the provider through create, read, update and delete for every
# resource, logging each phase so results can be pasted into TESTING.md.
#
#   ./run-tests.sh            full cycle, stops before destroy for confirmation
#   ./run-tests.sh --destroy  non-interactive, destroys at the end
#
# Requires OMNI_BASE_URL and OMNI_API_TOKEN, a terraform.tfvars, and a
# dev_overrides entry in ~/.terraformrc.

set -uo pipefail

LOG="results-$(date +%Y%m%d-%H%M%S).log"
PASS=0
FAIL=0

log()  { printf '%s\n' "$*" | tee -a "$LOG"; }
head() { log ""; log "════ $* ════"; }

check() {
  # check <name> <command...>
  local name="$1"; shift
  if "$@" >>"$LOG" 2>&1; then
    log "PASS  $name"
    PASS=$((PASS + 1))
  else
    log "FAIL  $name"
    FAIL=$((FAIL + 1))
  fi
}

: "${OMNI_BASE_URL:?set OMNI_BASE_URL}"
: "${OMNI_API_TOKEN:?set OMNI_API_TOKEN}"

log "Omni Terraform provider test run"
log "instance: $OMNI_BASE_URL"
log "started:  $(date)"

head "TC-00 provider starts and config is valid"
check "terraform validate" terraform validate

head "TC-01..08 CREATE"
check "create: plan"  terraform plan  -input=false -out=create.tfplan
check "create: apply" terraform apply -input=false -auto-approve create.tfplan
log "--- ids ---"
terraform output -json ids 2>/dev/null | tee -a "$LOG"

head "TC-09 READ round-trips with no drift"
if terraform plan -input=false -detailed-exitcode >>"$LOG" 2>&1; then
  log "PASS  read: no drift after create"
  PASS=$((PASS + 1))
else
  case $? in
    2) log "FAIL  read: drift detected after create (see log)"; FAIL=$((FAIL + 1)) ;;
    *) log "FAIL  read: plan errored"; FAIL=$((FAIL + 1)) ;;
  esac
fi

head "TC-10 data sources resolve to the created objects"
terraform output -json data_sources | tee -a "$LOG"
if terraform output -json data_sources | grep -q '"user_ids_match": *true'; then
  log "PASS  data source omni_user matches"; PASS=$((PASS + 1))
else
  log "FAIL  data source omni_user mismatch"; FAIL=$((FAIL + 1))
fi
if terraform output -json data_sources | grep -q '"group_ids_match": *true'; then
  log "PASS  data source omni_user_group matches"; PASS=$((PASS + 1))
else
  log "FAIL  data source omni_user_group mismatch"; FAIL=$((FAIL + 1))
fi

head "TC-11..18 UPDATE"
UPDATE_VARS=(
  -var "suffix=v2"
  -var "role_name=VIEWER"
  -var "topic_description=Updated by the Terraform provider test suite."
  -var "connection_base_role=QUERIER"
)
check "update: plan"  terraform plan  -input=false "${UPDATE_VARS[@]}" -out=update.tfplan
check "update: apply" terraform apply -input=false -auto-approve update.tfplan

head "TC-19 READ round-trips after update"
if terraform plan -input=false "${UPDATE_VARS[@]}" -detailed-exitcode >>"$LOG" 2>&1; then
  log "PASS  read: no drift after update"; PASS=$((PASS + 1))
else
  log "FAIL  read: drift detected after update"; FAIL=$((FAIL + 1))
fi

head "TC-20 IMPORT round-trips"
FOLDER_ID=$(terraform output -json ids | python3 -c 'import json,sys; print(json.load(sys.stdin)["folder_parent"])')
check "import: folder into a scratch address" \
  terraform import "${UPDATE_VARS[@]}" -input=false omni_folder.imported "$FOLDER_ID"
terraform state rm omni_folder.imported >>"$LOG" 2>&1 || true

head "TC-21..28 DELETE"
if [[ "${1:-}" == "--destroy" ]]; then
  check "destroy" terraform destroy -input=false -auto-approve "${UPDATE_VARS[@]}"
  log ""
  log "Role assignments have no delete endpoint. Confirm they were downgraded:"
  log "  curl -s -H \"Authorization: Bearer \$OMNI_API_TOKEN\" \\"
  log "    \"\$OMNI_BASE_URL/api/v1/users/<user id>/model-roles\""
else
  log "SKIP  destroy not run. Re-run with --destroy, or run terraform destroy yourself."
fi

head "SUMMARY"
log "passed: $PASS"
log "failed: $FAIL"
log "log:    $LOG"
[[ $FAIL -eq 0 ]]
