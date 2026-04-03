#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: bash scripts/watch-ci.sh [options]

Watch GitHub Actions workflow progress for the current branch.

Options:
  --branch <name>          Branch to watch (defaults to current git branch)
  --sha <commit>           Commit SHA to watch (defaults to HEAD)
  --workflow <name>        Workflow name to watch (default: ci)
  --interval <seconds>     Poll interval in seconds (default: 5)
  --latest-branch          Follow the newest matching branch run instead of a
                           single commit SHA
  -h, --help               Show this help text
EOF
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

log() {
  printf '[%s] %s\n' "$(date '+%H:%M:%S')" "$*"
}

workflow="ci"
interval="5"
branch=""
sha=""
watch_latest_branch=0

while (($# > 0)); do
  case "$1" in
    --branch)
      shift
      (($# > 0)) || die "--branch requires a value"
      branch="$1"
      ;;
    --sha)
      shift
      (($# > 0)) || die "--sha requires a value"
      sha="$1"
      ;;
    --workflow)
      shift
      (($# > 0)) || die "--workflow requires a value"
      workflow="$1"
      ;;
    --interval)
      shift
      (($# > 0)) || die "--interval requires a value"
      interval="$1"
      ;;
    --latest-branch)
      watch_latest_branch=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
  shift
done

command -v gh >/dev/null 2>&1 || die "GitHub CLI (gh) is required"

if [[ -z "$branch" ]]; then
  branch="$(git branch --show-current 2>/dev/null || true)"
fi
[[ -n "$branch" ]] || die "could not determine branch; pass --branch explicitly"

if [[ "$watch_latest_branch" -eq 0 && -z "$sha" ]]; then
  sha="$(git rev-parse HEAD 2>/dev/null || true)"
fi

if [[ "$watch_latest_branch" -eq 0 && -z "$sha" ]]; then
  die "could not determine commit SHA; pass --sha explicitly"
fi

resolve_run_id() {
  if [[ "$watch_latest_branch" -eq 1 ]]; then
    gh run list \
      --workflow "$workflow" \
      --branch "$branch" \
      --limit 20 \
      --json databaseId,event \
      --jq 'map(select(.event == "push")) | .[0].databaseId // empty'
    return
  fi

  gh run list \
    --workflow "$workflow" \
    --branch "$branch" \
    --limit 20 \
    --json databaseId,event,headSha \
    --jq 'map(select(.event == "push" and (.headSha | startswith("'"$sha"'")))) | .[0].databaseId // empty'
}

read_run_snapshot() {
  gh run view "$1" \
    --json displayTitle,headSha,status,conclusion,jobs \
    --jq '[.displayTitle, (.headSha[0:7]), .status, (.conclusion // ""), (.jobs | length), ([.jobs[] | select(.status == "completed")] | length), ([.jobs[] | select(.status == "in_progress")] | length), ([.jobs[] | select(.status == "queued" or .status == "waiting" or .status == "requested" or .status == "pending")] | length), ([.jobs[] | select(.status == "completed" and .conclusion != "success")] | length)] | join("\u001f")'
}

print_failed_jobs() {
  gh run view "$1" \
    --json jobs \
    --jq '.jobs[] | select(.status == "completed" and .conclusion != "success") | "- " + .name + " [" + .conclusion + "]"'
}

if [[ "$watch_latest_branch" -eq 1 ]]; then
  log "Watching workflow \"$workflow\" for the newest push on branch \"$branch\""
else
  log "Watching workflow \"$workflow\" for branch \"$branch\" at ${sha:0:7}"
fi

current_run_id=""
last_state=""
announced_wait=0

while true; do
  run_id="$(resolve_run_id || true)"
  if [[ -z "$run_id" ]]; then
    if [[ "$announced_wait" -eq 0 ]]; then
      if [[ "$watch_latest_branch" -eq 1 ]]; then
        log "No matching workflow run exists yet. Polling every ${interval}s."
      else
        log "No matching workflow run exists yet for ${sha:0:7}. Polling every ${interval}s."
      fi
      announced_wait=1
    fi
    sleep "$interval"
    continue
  fi

  announced_wait=0
  if [[ "$run_id" != "$current_run_id" ]]; then
    current_run_id="$run_id"
    last_state=""
    log "Tracking run $current_run_id"
  fi

  snapshot="$(read_run_snapshot "$current_run_id")"
  IFS=$'\x1f' read -r title short_sha status conclusion total_jobs completed_jobs running_jobs queued_jobs failed_jobs <<<"$snapshot"

  state_key="${current_run_id}:${status}:${conclusion}:${completed_jobs}:${running_jobs}:${queued_jobs}:${failed_jobs}"
  if [[ "$state_key" != "$last_state" ]]; then
    log "$title [$short_sha] status=$status completed=${completed_jobs}/${total_jobs} running=${running_jobs} queued=${queued_jobs}"
    last_state="$state_key"
  fi

  if [[ "$status" == "completed" ]]; then
    log "Run $current_run_id finished with conclusion=$conclusion"
    if [[ "$conclusion" != "success" && "$failed_jobs" != "0" ]]; then
      print_failed_jobs "$current_run_id"
    fi

    if [[ "$conclusion" == "success" ]]; then
      exit 0
    fi
    exit 1
  fi

  sleep "$interval"
done
