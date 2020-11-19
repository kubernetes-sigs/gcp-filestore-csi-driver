#!/bin/bash

# Copyright 2020 The Kubernetes Authors.
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

# A space-separated list of image tags under which the current build is to be pushed.
# Determined dynamically.
IMAGE_TAGS=

# A "canary" image gets built if the current commit is the head of the remote "master" branch.
if [ $(git rev-list -n1 HEAD) == $(git rev-list -n1 origin/master 2>/dev/null) ]; then
  IMAGE_TAGS+="canary "
fi

# A "X.Y.Z-canary" image gets built if the current commit is the head of a "origin/release-X.Y.Z" branch.
# The actual suffix does not matter, only the "release-" prefix is checked.
IMAGE_TAGS+=$(git branch -r --points-at=HEAD | grep 'origin/release-' | grep -v -e ' -> ' | sed -e 's;.*/release-\(.*\);\1-canary;')

# A release image "vX.Y.Z" gets built if there is a tag of that format for the current commit.
# --abbrev=0 suppresses long format, only showing the closest tag.
LATEST_GIT_TAG=$(git describe --tags --match='v*' --abbrev=0)
HEAD_REV=$(git rev-list -n1 HEAD)

# This variable stores the revision corresponding to the latest tag detected.
LATEST_TAG_REV=
if [ $LATEST_GIT_TAG ]; then
  LATEST_TAG_REV=$(git rev-list -n1 $LATEST_GIT_TAG)
fi

if [ $LATEST_TAG_REV ] && [ $LATEST_TAG_REV == $HEAD_REV ]; then
  IMAGE_TAGS+=$LATEST_GIT_TAG
  IMAGE_TAGS+=" "
fi

# If we did not detect any IMAGE_TAGS, then use the latest git head
# commit as the image tag.
if [[ -z $IMAGE_TAGS ]]; then
  IMAGE_TAGS+=$HEAD_REV
fi

echo $IMAGE_TAGS
