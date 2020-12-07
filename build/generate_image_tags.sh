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

# Tag for the driver image under which the current build is to be pushed.
IMAGE_TAG=

RELEASE_REGEX="^release-*"
# For an automatic build trigger by cloudbuild, PULL_BASE_REF will contain the ref that
# was pushed to trigger this build - a branch like 'master' or 'release-0.x', or a tag like 'v0.x'.
# See instructions here: https://github.com/kubernetes/test-infra/blob/master/config/jobs/image-pushing/README.md
# Three type of tags can be generated based on PULL_BASE_REF: canary, X.Y.Z-canary, vX.Y.Z.
if [[ -n $PULL_BASE_REF ]]; then
  if [[ $PULL_BASE_REF = "master" ]]; then
    IMAGE_TAG="canary"
  elif [[ $PULL_BASE_REF =~ $RELEASE_REGEX ]]; then
    IMAGE_TAG=$(echo $PULL_BASE_REF | cut -f2 -d '-')-canary
  else
    IMAGE_TAG=$PULL_BASE_REF
  fi
fi

# If we did not detect any IMAGE_TAG, then use the latest git head
# commit as the image tag.
if [[ -z $IMAGE_TAG ]]; then
  IMAGE_TAG=$(git rev-list -n1 HEAD)
fi

echo $IMAGE_TAG
