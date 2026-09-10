#!/usr/bin/env bash
# Removes objects left behind by a test run that failed before destroy.
#
#   ./cleanup-orphans.sh          list what would be removed
#   ./cleanup-orphans.sh --delete actually remove it
#
# Matches on the naming prefixes the suite uses. Nothing else is touched.

set -uo pipefail

: "${OMNI_BASE_URL:?set OMNI_BASE_URL}"
: "${OMNI_API_TOKEN:?set OMNI_API_TOKEN}"

DELETE=false
[[ "${1:-}" == "--delete" ]] && DELETE=true

api() { curl -s -H "Authorization: Bearer $OMNI_API_TOKEN" "$@"; }

echo "== models matching tf_test_ext / tf_yaml_test =="
api "$OMNI_BASE_URL/api/v1/models?pageSize=100" \
| python3 -c "
import sys, json
for m in json.load(sys.stdin).get('records', []):
    name = m.get('name') or ''
    if name.startswith(('tf_test_ext', 'tf_yaml_test')):
        print(m['id'], name)
" | while read -r id name; do
  if $DELETE; then
    api -X DELETE "$OMNI_BASE_URL/api/v1/models/$id" -w " -> %{http_code}\n" -o /dev/null
    echo "  deleted model $name"
  else
    echo "  would delete model $id $name"
  fi
done

echo "== folders matching tf-test =="
api "$OMNI_BASE_URL/api/v1/folders?pageSize=100" \
| python3 -c "
import sys, json
records = json.load(sys.stdin).get('records', [])
# Deepest paths first, so children go before their parents.
for f in sorted(records, key=lambda f: -f.get('path', '').count('/')):
    if f.get('name', '').startswith('tf-test'):
        print(f['id'], f['name'])
" | while read -r id name; do
  if $DELETE; then
    api -X DELETE "$OMNI_BASE_URL/api/v1/folders/$id?force=true" -w " -> %{http_code}\n" -o /dev/null
    echo "  deleted folder $name"
  else
    echo "  would delete folder $id $name"
  fi
done

echo "== groups matching tf-test-group =="
api "$OMNI_BASE_URL/api/scim/v2/groups" \
| python3 -c "
import sys, json
for g in json.load(sys.stdin).get('Resources', []):
    if g.get('displayName', '').startswith('tf-test-group'):
        print(g['id'], g['displayName'])
" | while read -r id name; do
  if $DELETE; then
    api -X DELETE "$OMNI_BASE_URL/api/scim/v2/groups/$id" -w " -> %{http_code}\n" -o /dev/null
    echo "  deleted group $name"
  else
    echo "  would delete group $id $name"
  fi
done

echo "== users matching tf-provider-test =="
api "$OMNI_BASE_URL/api/scim/v2/users?count=200" \
| python3 -c "
import sys, json
for u in json.load(sys.stdin).get('Resources', []):
    if u.get('userName', '').startswith('tf-provider-test'):
        print(u['id'], u['userName'])
" | while read -r id name; do
  if $DELETE; then
    api -X DELETE "$OMNI_BASE_URL/api/scim/v2/users/$id" -w " -> %{http_code}\n" -o /dev/null
    echo "  deleted user $name"
  else
    echo "  would delete user $id $name"
  fi
done

$DELETE || echo "
Nothing was removed. Re-run with --delete to apply."
