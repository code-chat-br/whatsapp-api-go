#!/usr/bin/env bash

set -Eeuo pipefail

DEFAULT_PLATFORMS="linux/amd64,linux/arm64"

usage() {
  cat <<'USAGE'
Build and push the whatsapp-go-api Docker image to Docker Hub.

Required variables:
  DOCKERHUB_USERNAME  Docker Hub namespace, for example codechat
  DOCKER_IMAGE_NAME   Docker Hub repository name, for example whatsapp-go-api
  IMAGE_VERSION       Required version tag, for example v1.0.0

Optional variables:
  GO_VERSION          Go image version. Defaults to the go.mod directive.
  APP_NAME            Defaults to whatsapp-go-api.
  APP_DESCRIPTION     Defaults to the project README description.
  APP_DEVELOPER       Defaults to CodeChat.
  APP_REPOSITORY      Defaults to the normalized origin remote URL when available.
  VCS_URL             Defaults to the normalized origin remote URL when available.
  PLATFORMS           Defaults to linux/amd64,linux/arm64.
  BUILDER_NAME        Defaults to codechat-multiarch.
  PUSH_LATEST         true publishes the latest tag. Defaults to false.
  PUSH_COMMIT_TAG     true publishes sha-<short commit>. Defaults to false.

Example:
  DOCKERHUB_USERNAME=codechat \
  DOCKER_IMAGE_NAME=whatsapp-go-api \
  IMAGE_VERSION=v1.0.0 \
  APP_NAME="whatsapp-go-api" \
  APP_DEVELOPER="CodeChat" \
  PUSH_LATEST=true \
  PUSH_COMMIT_TAG=true \
  ./scripts/build-and-push.sh
USAGE
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    fail "$name is required"
  fi
}

normalize_git_url() {
  local raw="$1"
  if [[ -z "$raw" ]]; then
    return 0
  fi
  if [[ "$raw" =~ ^git@([^:]+):(.+)$ ]]; then
    local host="${BASH_REMATCH[1]}"
    local path="${BASH_REMATCH[2]}"
    path="${path%.git}"
    printf 'https://%s/%s\n' "$host" "$path"
    return 0
  fi
  if [[ "$raw" =~ ^ssh://git@([^/]+)/(.+)$ ]]; then
    local host="${BASH_REMATCH[1]}"
    local path="${BASH_REMATCH[2]}"
    path="${path%.git}"
    printf 'https://%s/%s\n' "$host" "$path"
    return 0
  fi
  printf '%s\n' "$raw"
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

require_command docker
require_command git

docker info >/dev/null 2>&1 || fail "Docker daemon is not accessible"
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail "current directory is not a Git repository"
docker buildx version >/dev/null 2>&1 || fail "Docker Buildx is required"

require_env DOCKERHUB_USERNAME
require_env DOCKER_IMAGE_NAME
require_env IMAGE_VERSION

GO_VERSION="${GO_VERSION:-$(awk '/^go / {print $2; exit}' go.mod)}"
[[ -n "$GO_VERSION" ]] || fail "GO_VERSION could not be detected from go.mod"

VCS_REF="$(git rev-parse HEAD)"
VCS_REF_SHORT="$(git rev-parse --short HEAD)"
BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
REMOTE_URL="$(git config --get remote.origin.url || true)"
NORMALIZED_REMOTE_URL="$(normalize_git_url "$REMOTE_URL")"

APP_NAME="${APP_NAME:-whatsapp-go-api}"
APP_DESCRIPTION="${APP_DESCRIPTION:-API HTTP em Go para gerenciar instancias do WhatsApp com Whatsmeow}"
APP_DEVELOPER="${APP_DEVELOPER:-CodeChat}"
APP_REPOSITORY="${APP_REPOSITORY:-$NORMALIZED_REMOTE_URL}"
VCS_URL="${VCS_URL:-$NORMALIZED_REMOTE_URL}"
PLATFORMS="${PLATFORMS:-$DEFAULT_PLATFORMS}"
BUILDER_NAME="${BUILDER_NAME:-codechat-multiarch}"
PUSH_LATEST="${PUSH_LATEST:-false}"
PUSH_COMMIT_TAG="${PUSH_COMMIT_TAG:-false}"

FULL_IMAGE="${DOCKERHUB_USERNAME}/${DOCKER_IMAGE_NAME}"

if ! docker info 2>/dev/null | grep -q 'Username:'; then
  printf 'warning: Docker Hub login was not detected in docker info. Run docker login before pushing if this build fails with authentication errors.\n' >&2
fi

docker buildx inspect "$BUILDER_NAME" >/dev/null 2>&1 || \
  docker buildx create --name "$BUILDER_NAME" --use
docker buildx use "$BUILDER_NAME"
docker buildx inspect --bootstrap >/dev/null

tags=(--tag "${FULL_IMAGE}:${IMAGE_VERSION}")
if [[ "$PUSH_LATEST" == "true" ]]; then
  tags+=(--tag "${FULL_IMAGE}:latest")
fi
if [[ "$PUSH_COMMIT_TAG" == "true" ]]; then
  tags+=(--tag "${FULL_IMAGE}:sha-${VCS_REF_SHORT}")
fi

docker buildx build \
  --platform "$PLATFORMS" \
  --file Dockerfile \
  "${tags[@]}" \
  --build-arg GO_VERSION="$GO_VERSION" \
  --build-arg APP_NAME="$APP_NAME" \
  --build-arg APP_VERSION="$IMAGE_VERSION" \
  --build-arg APP_DESCRIPTION="$APP_DESCRIPTION" \
  --build-arg APP_DEVELOPER="$APP_DEVELOPER" \
  --build-arg APP_REPOSITORY="$APP_REPOSITORY" \
  --build-arg BUILD_DATE="$BUILD_DATE" \
  --build-arg VCS_REF="$VCS_REF" \
  --build-arg VCS_URL="$VCS_URL" \
  --push \
  .

printf '\nPublished image: %s:%s\n' "$FULL_IMAGE" "$IMAGE_VERSION"
printf 'Platforms: %s\n' "$PLATFORMS"
printf 'Revision: %s\n' "$VCS_REF"
printf 'Build date: %s\n' "$BUILD_DATE"
if [[ "$PUSH_LATEST" == "true" ]]; then
  printf 'Also published: %s:latest\n' "$FULL_IMAGE"
fi
if [[ "$PUSH_COMMIT_TAG" == "true" ]]; then
  printf 'Also published: %s:sha-%s\n' "$FULL_IMAGE" "$VCS_REF_SHORT"
fi
