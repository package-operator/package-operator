set -o pipefail

if [[ -n "$(git status --porcelain)" ]]; then
  git diff
  echo "Repo is dirty! Propably because gofmt or make generate touched something...";
  exit 1
fi
