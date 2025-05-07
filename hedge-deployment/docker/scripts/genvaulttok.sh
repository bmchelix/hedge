#!/bin/bash
#set -x
set -e

isk8s=false
KUBECONFIG=""
K8S_NAMESPACE=""

# Print usage if invalid arguments passed
usage() {
  echo "Usage: $0 [-k true/false] [-c <KUBECONFIG_PATH>] [-n <K8S_NAMESPACE>]"
  exit 1
}

# Parse command-line arguments
while getopts "k:c:n:" opt; do
  case "$opt" in
    k) isk8s="$OPTARG" ;;
    c) KUBECONFIG="$OPTARG" ;;
    n) K8S_NAMESPACE="$OPTARG" ;;
    *) usage ;;
  esac
done

if [ "$isk8s" = true ]; then
  if [ -z "$KUBECONFIG" ] || [ -z "$K8S_NAMESPACE" ]; then
    echo "Error: KUBECONFIG and K8S_NAMESPACE must be set when -k is true (indicates Kubernetes env)"
    exit 1
  fi
fi

# Vault configuration
VAULT_ADDR="http://edgex-vault:8200"
VAULT_SETUP_CONTAINER="edgex-security-secretstore-setup"
VAULT_API_CONTAINER="hedge-init"

# Function to retrieve unseal keys
get_unseal_keys() {
  if $isk8s; then
    kubectl exec --kubeconfig="$KUBECONFIG" -n "$K8S_NAMESPACE" "deploy/$VAULT_SETUP_CONTAINER" \
      -- cat /vault/config/assets/resp-init.json | jq -r '.keys[]'
  else
    docker exec "$VAULT_SETUP_CONTAINER" cat /vault/config/assets/resp-init.json | jq -r '.keys[]'
  fi
}

# Function to execute commands in the appropriate container
exec_command() {
  local cmd="$1"
  if $isk8s; then
    kubectl exec -i --kubeconfig="$KUBECONFIG" -n "$K8S_NAMESPACE" job/"$VAULT_API_CONTAINER" -c "$VAULT_API_CONTAINER" -- env UNSEAL_KEYS="$UNSEAL_KEYS" VAULT_ADDR="$VAULT_ADDR" /bin/sh <<EOF
$(cat)
EOF
  else
    docker exec -i -e UNSEAL_KEYS="$UNSEAL_KEYS" -e VAULT_ADDR="$VAULT_ADDR" "$VAULT_API_CONTAINER" /bin/sh <<EOF
$(cat)
EOF
  fi
}

# Function to generate a root token
generate_root_token() {
  local result
  result=$(exec_command <<'COMMANDS'
    #set -x
    set -e

    echo "Cancel any ongoing root token generation process..."
    curl -s --request DELETE "$VAULT_ADDR/v1/sys/generate-root/attempt"

    echo "Initializing root token generation..."
    INIT_OUTPUT=$(curl -s --request PUT "$VAULT_ADDR/v1/sys/generate-root/attempt")
    if echo "$INIT_OUTPUT" | jq -e '.errors' > /dev/null; then
      echo "Error during root token generation initialization: $(echo "$INIT_OUTPUT" | jq -r '.errors[]')"
      exit 1
    fi

    NONCE=$(echo "$INIT_OUTPUT" | jq -r '.nonce')
    OTP=$(echo "$INIT_OUTPUT" | jq -r '.otp')

    echo "Signing root token request with unseal keys..."
    PROGRESS="0/3"
    for KEY in $UNSEAL_KEYS; do
      SIGN_OUTPUT=$(curl -s --request PUT "$VAULT_ADDR/v1/sys/generate-root/update" \
        --data "{\"key\": \"$KEY\", \"nonce\": \"$NONCE\"}")
      if echo "$SIGN_OUTPUT" | jq -e '.errors' > /dev/null; then
        echo "Error during root token signing: $(echo "$SIGN_OUTPUT" | jq -r '.errors[]')"
        exit 1
      fi

      PROGRESS=$(echo "$SIGN_OUTPUT" | jq -r '.progress')
      echo "Progress: $PROGRESS/3"

      if [[ "$PROGRESS" == 3 ]]; then
        ENCODED_TOKEN=$(echo "$SIGN_OUTPUT" | jq -r '.encoded_token')
        break
      fi
    done

    echo "Decoding the generated root token..."
    DECODE_OUTPUT=$(curl -s --request PUT "$VAULT_ADDR/v1/sys/decode-token" \
      --data "{\"otp\": \"$OTP\", \"encoded_token\": \"$ENCODED_TOKEN\"}")
    if echo "$DECODE_OUTPUT" | jq -e '.errors' > /dev/null; then
      echo "Error during root token decoding: $(echo "$DECODE_OUTPUT" | jq -r '.errors[]')"
      exit 1
    fi

    DECODED_TOKEN=$(echo "$DECODE_OUTPUT" | jq -r '.data.token')
    echo "FINAL_DECODED_TOKEN=$DECODED_TOKEN"
COMMANDS
  )
  echo "$result"
}

# Function to revoke a root token
revoke_root_token() {
  #set -x
  local token="$1"
  echo "Revoking root token..." #$token
  exec_command <<COMMANDS
    #set -x
    token="$token"
    response=\$(curl -s --request POST --header "X-Vault-Token: \$token" "\$VAULT_ADDR/v1/auth/token/revoke" --data '{"token": "'\$token'"}' -w "%{http_code}")
    if [ -z "\$response" ]; then
      echo "Error: No response received. The server might be down or unreachable."
      return 1
    fi
    status_code="\${response: -3}"                # Extract the status code from response
    # Check for curl connection failure (status_code is 000)
    if [ "\$status_code" = "000" ]; then
      echo "Error: Unable to connect to the server. The server might be down or unreachable."
      return 1
    fi
    if [ "\$status_code" -ne 204 ]; then
      echo "Error: Curl request failed with status code \$status_code."
      return 1
    fi

    echo -e "Token revoked!"
COMMANDS
}

# Main script execution
UNSEAL_KEYS=$(get_unseal_keys)
if [ -z "$UNSEAL_KEYS" ]; then
  echo "Error: Failed to retrieve Vault unseal keys"
  exit 1
fi

echo "Starting root token generation process..."
RESULT=$(generate_root_token)
echo "$RESULT" | grep -v "FINAL_DECODED_TOKEN"    #print logs from generate_root_token()
if [ -z "$RESULT" ]; then
  echo "Error: Failed to generate root token"
  exit 1
fi

DECODED_TOKEN=$(echo "$RESULT" | grep "FINAL_DECODED_TOKEN=" | cut -d'=' -f2 | xargs)
if [ -z "$DECODED_TOKEN" ]; then
  echo "Error: Failed to extract root token"
  exit 1
fi

echo -e "\n===========================================================================\n"
echo -e "\tVAULT TOKEN (temporary) = $DECODED_TOKEN"
echo -e "\n===========================================================================\n"

echo -e "\nPress \"ENTER\" to REVOKE the token."
read
revoke_root_token "$DECODED_TOKEN"