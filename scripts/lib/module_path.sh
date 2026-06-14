#!/usr/bin/env bash

rtagent_is_final_module_path() {
  local path="$1"
  local first
  first="${path%%/*}"
  [[ "$path" == */* ]] &&
    [[ "$path" != *"://"* ]] &&
    [[ "$path" != *":"* ]] &&
    [[ "$path" != *"@"* ]] &&
    [[ "$path" != *"\\"* ]] &&
    [[ "$path" != *"?"* ]] &&
    [[ "$path" != *"#"* ]] &&
    [[ "$path" != *" "* ]] &&
    [[ "$path" != /* ]] &&
    [[ "$path" != .* ]] &&
    [[ "$path" != *"//"* ]] &&
    [[ "$path" != */ ]] &&
    [[ "$first" == *.* ]] &&
    [[ "$first" != "localhost" ]]
}
