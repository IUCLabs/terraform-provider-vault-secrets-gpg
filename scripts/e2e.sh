#!/usr/bin/env bash
#
# End-to-end test for terraform-provider-vault-secrets-gpg. Boots a Vault dev server
# running the vault-plugin-secrets-gpg engine, then applies real terraform that
# registers + mounts the backend, creates a key, and reads it via the data
# source. Verifies the outputs and that the mounted engine actually works, then
# destroys and confirms cleanup.
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLUGIN_BIN="${PLUGIN_BIN:-${ROOT_DIR}/../vault-plugin-secrets-gpg/dist/vault-plugin-secrets-gpg}"

red()   { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
blue()  { printf '\033[34m==> %s\033[0m\n' "$*"; }
fail()  { red "FAIL: $*"; exit 1; }

command -v vault >/dev/null || fail "vault binary not found"
command -v terraform >/dev/null || fail "terraform binary not found"
[ -x "${PLUGIN_BIN}" ] || fail "plugin binary not found at ${PLUGIN_BIN} (build it: make -C ../vault-plugin-secrets-gpg build)"

WORK="$(mktemp -d)"
PLUGIN_DIR="${WORK}/plugins"
mkdir -p "${PLUGIN_DIR}"
cp "${PLUGIN_BIN}" "${PLUGIN_DIR}/vault-plugin-secrets-gpg"
PLUGIN_SHA="$(sha256sum "${PLUGIN_DIR}/vault-plugin-secrets-gpg" | awk '{print $1}')"

export VAULT_ADDR="http://127.0.0.1:8282"
export VAULT_TOKEN="root"

cleanup() {
  [ -n "${VAULT_PID:-}" ] && kill "${VAULT_PID}" 2>/dev/null || true
  rm -rf "${WORK}"
}
trap cleanup EXIT

blue "Building provider"
make -C "${ROOT_DIR}" build >/dev/null
PROVIDER_BIN="${ROOT_DIR}/terraform-provider-vault-secrets-gpg"

blue "Starting Vault dev server (plugin_directory=${PLUGIN_DIR})"
vault server -dev -dev-root-token-id=root -dev-listen-address=127.0.0.1:8282 \
  -dev-plugin-dir="${PLUGIN_DIR}" >"${WORK}/vault.log" 2>&1 &
VAULT_PID=$!
for i in $(seq 1 30); do
  vault status >/dev/null 2>&1 && break
  sleep 0.5
  [ "$i" = 30 ] && fail "vault did not become ready"
done
green "vault ready"

# Dev override so terraform uses the freshly built provider binary directly.
cat > "${WORK}/dev.tfrc" <<EOF
provider_installation {
  dev_overrides {
    "registry.terraform.io/iuclabs/vault-secrets-gpg" = "$(dirname "${PROVIDER_BIN}")"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="${WORK}/dev.tfrc"

TFDIR="${ROOT_DIR}/test/e2e"
export TF_IN_AUTOMATION=1
tfvars=(-var "address=${VAULT_ADDR}" -var "token=root" -var "plugin_sha256=${PLUGIN_SHA}")

blue "terraform apply"
# dev_overrides skips init; apply directly.
terraform -chdir="${TFDIR}" apply -auto-approve "${tfvars[@]}" >/dev/null
green "apply succeeded"

fpr="$(terraform -chdir="${TFDIR}" output -raw resource_fingerprint)"
pub="$(terraform -chdir="${TFDIR}" output -raw data_public_key)"
ver="$(terraform -chdir="${TFDIR}" output -raw latest_version)"
[ -n "${fpr}" ] || fail "no fingerprint output"
printf '%s' "${pub}" | grep -q 'BEGIN PGP PUBLIC KEY BLOCK' || fail "data source public_key not armored"
[ "${ver}" = "1" ] || fail "latest_version != 1 (got ${ver})"
green "outputs OK: version=${ver} fpr=${fpr:0:16}..."

blue "engine is live: encrypt + decrypt via the mounted backend"
ct="$(vault write -field=ciphertext gpg/encrypt/app plaintext="$(printf 'tf-secret' | base64)")"
pt="$(vault write -field=plaintext gpg/decrypt/app ciphertext="${ct}" | base64 -d)"
[ "${pt}" = "tf-secret" ] || fail "encrypt/decrypt through the tf-managed engine failed"
green "encrypt/decrypt OK"

blue "terraform destroy"
terraform -chdir="${TFDIR}" destroy -auto-approve "${tfvars[@]}" >/dev/null
if vault secrets list -format=json 2>/dev/null | grep -q '"gpg/"'; then
  fail "mount still present after destroy"
fi
green "destroy removed the mount + key"

# tidy generated state so the tree stays clean
rm -f "${TFDIR}"/terraform.tfstate* "${TFDIR}"/.terraform.lock.hcl
rm -rf "${TFDIR}"/.terraform

green "ALL PROVIDER END-TO-END CHECKS PASSED"
