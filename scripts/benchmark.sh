#!/bin/bash

output_folder=${1:-"profile"}
packages="$(go list ./... | grep -v /vendor/ | xargs echo)"
mkdir -p "$output_folder"

for package_path in $packages; do
  package_name="$(basename "$package_path")"
  cpu_profile_file="${output_folder}/${package_name}_cpu.profile"
  memory_profile_file="${output_folder}/${package_name}_memory.profile"
  go test \
    --benchmem \
    -cpuprofile="$cpu_profile_file" \
    -memprofile="$memory_profile_file" \
    -bench=. \
    -count 5 \
    "${package_path}"
  if [ -e "$cpu_profile_file" ]; then
    go tool pprof -png "$cpu_profile_file" >"${cpu_profile_file}.png"
  fi
  if [ -e "$memory_profile_file" ]; then
    go tool pprof -png "$memory_profile_file" >"${memory_profile_file}.png"
  fi
done
