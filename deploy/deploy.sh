#!/usr/bin/env bash
set -euo pipefail

############################################
# Config
############################################
IMAGE_NAME="${IMAGE_NAME}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
PLATFORMS="linux/amd64,linux/arm64"
DOCKERFILE="../coordinator/Dockerfile"
BUILD_CONTEXT=".."
ENV_FILE="../.env.prod"

############################################
# 1. Load .env and export variables
############################################
if [ ! -f "$ENV_FILE" ]; then
  echo "❌ .env file not found"
  exit 1
fi

echo "🔐 Loading environment variables from .env"
set -a
source "$ENV_FILE"
set +a

############################################
# 2. Docker login (only if not logged in)
############################################
if ! docker info 2>/dev/null | grep -q "Username"; then
  echo "🔑 Docker login required"

  : "${DOCKER_USERNAME:?DOCKER_USERNAME not set}"
  : "${DOCKER_PAT:?DOCKER_PAT not set}"

  echo "$DOCKER_PAT" | docker login \
    -u "$DOCKER_USERNAME" \
    --password-stdin
else
  echo "✅ Docker already logged in"
fi

############################################
# 3. Ensure buildx builder
############################################
BUILDER_NAME="clwclw-builder"

if ! docker buildx inspect "$BUILDER_NAME" >/dev/null 2>&1; then
  echo "🛠 Creating buildx builder: $BUILDER_NAME"
  docker buildx create --name "$BUILDER_NAME" --use
else
  echo "🛠 Using existing buildx builder: $BUILDER_NAME"
  docker buildx use "$BUILDER_NAME"
fi

docker buildx inspect --bootstrap >/dev/null

############################################
# 4. Pre-pull base images (stability)
############################################
echo "📦 Pre-pulling base images for stability"
docker pull --platform=linux/amd64 golang:1.22-alpine || true
docker pull --platform=linux/arm64 golang:1.22-alpine || true

############################################
# 5. Build & Push multi-arch image
############################################
echo "🚀 Building and pushing multi-arch image"
echo "   Image: $IMAGE_NAME:$IMAGE_TAG"
echo "   Platforms: $PLATFORMS"

docker buildx build \
  --platform "$PLATFORMS" \
  -f "$DOCKERFILE" \
  -t "$IMAGE_NAME:$IMAGE_TAG" \
  --push \
  "$BUILD_CONTEXT"

echo "✅ Build & push completed successfully"
