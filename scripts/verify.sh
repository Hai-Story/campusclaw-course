#!/bin/sh
set -eu

base_url="${BASE_URL:-http://localhost:8080}"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

teacher_password="$(docker compose exec -T api printenv SEED_TEACHER_A_PASSWORD)"
student_password="$(docker compose exec -T api printenv SEED_STUDENT_A1_PASSWORD)"
sample_file="$work_dir/verify-material.md"
printf '# 验收材料\n\n由自动验收脚本上传。\n' > "$sample_file"

assert_status() {
  expected="$1"
  actual="$2"
  label="$3"
  if [ "$actual" != "$expected" ]; then
    printf 'FAIL %-28s expected=%s actual=%s\n' "$label" "$expected" "$actual"
    exit 1
  fi
  printf 'PASS %-28s status=%s\n' "$label" "$actual"
}

status="$(curl -sS -o "$work_dir/health" -w '%{http_code}' "$base_url/health")"
assert_status 200 "$status" "public health"

status="$(curl -sS -o "$work_dir/unauth" -w '%{http_code}' "$base_url/api/materials")"
assert_status 401 "$status" "unauthenticated list"

status="$(curl -sS -o "$work_dir/cookie-only" -w '%{http_code}' -H 'Cookie: campus_session=legacy-session' "$base_url/api/materials")"
assert_status 401 "$status" "legacy cookie rejected"

status="$(curl -sS -D "$work_dir/teacher-headers" -o "$work_dir/teacher-login" -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"teacher_a\",\"password\":\"$teacher_password\"}" \
  "$base_url/api/login")"
assert_status 200 "$status" "teacher login"
if grep -qi '^set-cookie:' "$work_dir/teacher-headers"; then
  printf 'FAIL login issued a cookie\n'
  exit 1
fi
printf 'PASS login issued no cookie\n'
teacher_token="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["token"])' "$work_dir/teacher-login")"

status="$(curl -sS -o "$work_dir/teacher-me" -w '%{http_code}' -H "Authorization: Bearer $teacher_token" "$base_url/api/me")"
assert_status 200 "$status" "bearer identity"

status="$(curl -sS -o "$work_dir/tampered" -w '%{http_code}' -H "Authorization: Bearer ${teacher_token}tampered" "$base_url/api/materials")"
assert_status 401 "$status" "tampered bearer rejected"

status="$(curl -sS -o "$work_dir/upload" -w '%{http_code}' -H "Authorization: Bearer $teacher_token" \
  -F 'title=自动验收材料' -F "file=@$sample_file;type=text/markdown" \
  "$base_url/api/materials")"
assert_status 201 "$status" "teacher upload"

status="$(curl -sS -o "$work_dir/student-login" -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"student_a1\",\"password\":\"$student_password\"}" \
  "$base_url/api/login")"
assert_status 200 "$status" "student login"
student_token="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["token"])' "$work_dir/student-login")"

status="$(curl -sS -o "$work_dir/student-upload" -w '%{http_code}' -H "Authorization: Bearer $student_token" \
  -F "file=@$sample_file;type=text/markdown" "$base_url/api/materials")"
assert_status 403 "$status" "student upload denied"

class_b_id="$(docker compose exec -T db sh -c 'mysql -N -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -e "SELECT m.id FROM materials m JOIN users u ON u.class_id=m.class_id WHERE u.username=\"student_b1\" ORDER BY m.id LIMIT 1"' 2>/dev/null)"
status="$(curl -sS -o "$work_dir/cross-class" -w '%{http_code}' -H "Authorization: Bearer $teacher_token" "$base_url/api/materials/$class_b_id")"
assert_status 404 "$status" "cross-class detail hidden"

status="$(curl -sS -o "$work_dir/missing" -w '%{http_code}' -H "Authorization: Bearer $teacher_token" "$base_url/api/materials/999999999")"
assert_status 404 "$status" "missing detail hidden"

if ! cmp -s "$work_dir/cross-class" "$work_dir/missing"; then
  printf 'FAIL cross-class and missing bodies differ\n'
  exit 1
fi
printf 'PASS cross-class and missing bodies are identical\n'

status="$(curl -sS -o "$work_dir/logout" -w '%{http_code}' -X POST -H "Authorization: Bearer $teacher_token" "$base_url/api/logout")"
assert_status 204 "$status" "bearer logout"

status="$(curl -sS -o "$work_dir/revoked" -w '%{http_code}' -H "Authorization: Bearer $teacher_token" "$base_url/api/materials")"
assert_status 401 "$status" "revoked bearer rejected"

printf '\nAll critical scenarios passed.\n'
