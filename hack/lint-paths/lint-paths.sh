#!/bin/bash
#
# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright The KubeVirt Authors.
#

set -e

BASEDIR=$(dirname $0)

suffix=""

# Check if more than one parameter is provided
if [[ $# -gt 1 ]]; then
  echo "Error: Only one parameter is allowed."
  exit 1
fi

if [[ $# -eq 1 ]]; then
  suffix="$1"
fi

coveredpaths=""
while IFS= read -r line; do
    # read directory from the file and append a wildcard
    coveredpaths+="${line}${suffix} "
done <$BASEDIR/lint-paths.txt

# Iterate through directories within the current directory
for dir in $BASEDIR/*/; do
  if [[ -d "$dir" ]]; then
    file_path="$dir/lint-paths.txt"

    if [[ -f "$file_path" ]]; then
      while IFS= read -r line; do
        coveredpaths+="${line}${suffix} "
      done < "$file_path"
    fi
  fi
done
echo $coveredpaths