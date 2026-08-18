#!/bin/sh
# scripts/verify.sh — DelegationProof one-command release-gate verification.
#
# Runs every objective v1.0 release gate (docs/v1-release-plan.md §3) against
# the current checkout, in order, fail-fast. Each step prints a "==> <step>"
# header followed by PASS or the failing tool's own output. The first
# failing step aborts the script with a non-zero exit and a one-line summary
# naming which gate failed.
#
# Requires only the Go toolchain already needed to build the project (plus
# POSIX sh, git, diff — no network access, no non-stdlib dependency).
#
# Usage: ./scripts/verify.sh   (run from anywhere in the repo checkout)

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd) || exit 1
cd "$ROOT_DIR" || exit 1

fail() {
    echo "FAIL: $1" >&2
    echo ""
    echo "==> release gate failed: $1" >&2
    exit 1
}

step() {
    echo "==> $1"
}

# 1. Formatting
step "gofmt"
GOFMT_OUT=$(gofmt -l .)
if [ -n "$GOFMT_OUT" ]; then
    echo "$GOFMT_OUT" >&2
    fail "gofmt -l . reported unformatted files"
fi
echo "PASS"

# 2. Vet
step "go vet ./..."
if ! go vet ./...; then
    fail "go vet ./..."
fi
echo "PASS"

# 3. Race + unit tests
step "go test ./... -race -count=1"
if ! go test ./... -race -count=1; then
    fail "go test ./... -race -count=1"
fi
echo "PASS"

# 4. Build
step "go build -o bin/delegationproof ./cmd/delegationproof"
if ! go build -o bin/delegationproof ./cmd/delegationproof; then
    fail "go build -o bin/delegationproof ./cmd/delegationproof"
fi
echo "PASS"

BIN="$ROOT_DIR/bin/delegationproof"

# 5. Deterministic example verification (shipped binary, not the test binary)
step "deterministic example verification (examples/*.json)"
for f in examples/*.json; do
    out1=$("$BIN" verify "$f" --format json)
    code1=$?
    out2=$("$BIN" verify "$f" --format json)
    code2=$?
    if [ "$code1" -ne 1 ]; then
        fail "$f: expected exit code 1 (DENY) on first run, got $code1"
    fi
    if [ "$code2" -ne 1 ]; then
        fail "$f: expected exit code 1 (DENY) on second run, got $code2"
    fi
    if [ "$out1" != "$out2" ]; then
        fail "$f: two runs of the shipped binary produced different --format json output"
    fi
done
echo "PASS"

# 6. Malformed-input fail-closed check (shipped binary)
step "malformed-input fail-closed check (testdata/malformed/*.json)"
for f in testdata/malformed/*.json; do
    "$BIN" verify "$f" >/dev/null 2>&1
    code=$?
    if [ "$code" -ne 2 ]; then
        fail "$f: expected exit code 2 (invalid model/input), got $code"
    fi
done
echo "PASS"

# 7. Repository hygiene
step "repository hygiene (git status --porcelain)"
DIRTY=$(git status --porcelain)
if [ -n "$DIRTY" ]; then
    echo "$DIRTY" >&2
    fail "working tree has uncommitted/untracked changes after verification (bin/ should be gitignored; testdata/golden/ should never change)"
fi
echo "PASS"

echo ""
echo "==> all release gates passed"
exit 0
