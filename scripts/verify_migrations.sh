#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
migrations_dir="${root_dir}/migrations"

if [[ ! -d "${migrations_dir}" ]]; then
  printf 'Migration directory not found: %s\n' "${migrations_dir}" >&2
  exit 1
fi

mapfile -t up_files < <(find "${migrations_dir}" -maxdepth 1 -type f -name '*.up.sql' -printf '%f\n' | sort)
mapfile -t down_files < <(find "${migrations_dir}" -maxdepth 1 -type f -name '*.down.sql' -printf '%f\n' | sort)

if (( ${#up_files[@]} == 0 )); then
  printf 'No up migrations found in %s\n' "${migrations_dir}" >&2
  exit 1
fi

if (( ${#up_files[@]} != ${#down_files[@]} )); then
  printf 'Migration pair count mismatch: %d up, %d down\n' "${#up_files[@]}" "${#down_files[@]}" >&2
  exit 1
fi

expected=1
for up_file in "${up_files[@]}"; do
  version="${up_file%%_*}"
  numeric_version=$((10#${version}))

  if (( numeric_version != expected )); then
    printf 'Migration sequence gap: expected %06d but found %s\n' "${expected}" "${version}" >&2
    exit 1
  fi

  down_file="${up_file%.up.sql}.down.sql"
  if [[ ! -f "${migrations_dir}/${down_file}" ]]; then
    printf 'Missing down migration for %s\n' "${up_file}" >&2
    exit 1
  fi

  expected=$((expected + 1))
done

printf 'Migration verification passed: %d ordered up/down pairs.\n' "${#up_files[@]}"
