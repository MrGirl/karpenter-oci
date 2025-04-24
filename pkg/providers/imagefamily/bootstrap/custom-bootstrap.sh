#!/bin/bash
set -xe
set -o pipefail

# function start
function curl_instance_metadata() {
   local url="$1"
   local retries=10
   while [[ $retries -gt 0 ]]; do
     output=$(curl --fail -H "Authorization: Bearer Oracle" -L0 "$url") && break
     if [[ $retries -eq 0 ]]; then
        echo "Failed to call IMDS"
        exit 1
     fi
     retries=$((retries-1))
   done
   echo "$output"
}
function get_pause_image() {
  local region=$1
  local realm=$2

  if [[ "$realm" == "oc1" ]]; then
    pause_image="${region}.ocir.io/axoxdievda5j/oke-public-pause@sha256:742e9460caed7154efc56528a548a5c256a5ca1d2d3d6060b0b279bc881ed83a"
  elif [[ "$realm" == "oc2" ]]; then
    pause_image="ocir.${region}.oci.oraclegovcloud.com/oke-native-steward/oke-public-pause-amd64@sha256:c59a2c705ca2db2878aa1804b31127beb1ff70f33db3a0e8a7190ba38972d6bf"
  elif [[ "$realm" == "oc3" ]]; then
    pause_image="ocir.${region}.oci.oraclegovcloud.com/oke-native-steward/oke-public-pause-amd64@sha256:c59a2c705ca2db2878aa1804b31127beb1ff70f33db3a0e8a7190ba38972d6bf"
  elif [[ "$realm" == "oc4" ]]; then
    pause_image="ocir.${region}.oci.oraclegovcloud.uk/axam0kg7wsji/oke-public-pause-amd64@sha256:c59a2c705ca2db2878aa1804b31127beb1ff70f33db3a0e8a7190ba38972d6bf"
  elif [[ "$realm" == "oc5" || "$realm" == "oc6" || "$realm" == "oc7" || "$realm" == "oc11" ]]; then
    oci_tld=$(curl_instance_metadata 'http://169.254.169.254/opc/v2/instance/regionInfo/realmDomainComponent')
    pause_image="ocir.${region}.oci.${oci_tld}/oke-native-steward/oke-public-pause@sha256:35e110013be8acb37b8623457f6db6e9f21ef1e7a0e9fa73507e04bc4410457c"
  else
    oci_tld=$(curl_instance_metadata 'http://169.254.169.254/opc/v2/instance/regionInfo/realmDomainComponent')
    pause_image="ocir.${region}.oci.${oci_tld}/oke-native-steward/oke-public-pause-amd64@sha256:c59a2c705ca2db2878aa1804b31127beb1ff70f33db3a0e8a7190ba38972d6bf"
  fi

  echo "$pause_image"
}
function enable_and_restart() {
  sudo systemctl enable "$1"
  sudo systemctl restart "$1"
}
function daemon_reload() {
  systemctl daemon-reload || log "Error reloading systemd units"
}

#######################################
# Functions to get useful IMDS fields during node provisioning
#######################################

export INSTANCE_FILE=/etc/self-k8s/imds_instance.json
export PRIVATE_IP_FILE=/etc/self-k8s/imds_private_ip.json

function get_imds_instance() {
  find "${INSTANCE_FILE}" -mmin -1 -not -empty > /dev/null 2>&1 || (curl_instance_metadata 'http://169.254.169.254/opc/v2/instance' | jq -rcM '.' > "${INSTANCE_FILE}")
  INSTANCE="$(cat "${INSTANCE_FILE}" || echo -n '')"
  export INSTANCE
  echo "${INSTANCE}"
}

function get_imds_metadata() {
  get_imds_instance | jq -rcM '.["metadata"]' || echo '{}'
}

function get_oke_k8version() {
  local k8_version
  k8_version=$(get_imds_metadata | jq -rcM '.["oke-k8version"] // ""' || echo "")
  if [[ -n "$k8_version" ]]; then
    echo "$k8_version"
  else
    kubelet --version | cut -d " " -f2
  fi
}

function get_instance_hostname() {
  get_imds_instance | jq -rcM '.["hostname"] // "unknown" ' || echo "unknown"
}

function get_instance_id() {
  get_imds_instance | jq -rcM '.["id"] // "unknown" ' || echo "unknown"
}

function get_compartment_name() {
  get_imds_metadata | jq -rcM '.["oke-compartment-name"] // "unknown"' || echo "unknown"
}

function get_is_on_private_subnet() {
  get_imds_metadata | jq -rcM '.["oke-is-on-private-subnet"] // "unknown"' || echo "unknown"
}

function get_realm() {
  get_imds_instance | jq -rcM  '.["regionInfo"]["realmKey"]'
}

function get_region() {
  get_imds_instance | jq -rcM  '.["canonicalRegionName"]'
}

function get_initial_node_labels() {
  get_imds_metadata | jq -rcM '.["oke-initial-node-labels"] // ""' || echo ""
}

function get_private_ip() {
  find "${PRIVATE_IP_FILE}" > /dev/null 2>&1 || (curl_instance_metadata 'http://169.254.169.254/opc/v2/vnics/0/privateIp' > "${PRIVATE_IP_FILE}")
  PRIVATE_IP="$(cat "${PRIVATE_IP_FILE}" || echo -n '')"
  export PRIVATE_IP
  echo "${PRIVATE_IP}"
}

function service_env_rule() {
  echo -e "[Service]\nEnvironment=\"${1}=${2}\""
}

function get_container_runtime_endpoint() {
  echo "unix:///run/containerd/containerd.sock"
}

function get_node_labels() {
  local labels=""
  labels="$(get_initial_node_labels),"
  labels="${labels}hostname=$(get_instance_hostname)"
  labels="${labels},internal_addr=$(get_private_ip)"
  labels="${labels},displayName=$(get_instance_hostname)"
  labels="${labels},node.info/compartment.name=$(get_compartment_name)"
  labels="${labels},node.info/kubeletVersion=$(get_oke_k8version | cut -d. -f1-2)"
  labels="${labels},node.info.ds_proxymux_client=true"
  echo "${labels}"
}

function get_kubelet_default_args() {
  local kubelet_default_args=""
  kubelet_default_args="${kubelet_default_args} --hostname-override=$(get_instance_hostname)"
  kubelet_default_args="${kubelet_default_args} --node-ip=$(get_private_ip)"
  kubelet_default_args="${kubelet_default_args} --pod-infra-container-image=${PAUSE_IMAGE}"
  kubelet_default_args="${kubelet_default_args} --node-labels=${NODE_LABELS}"
  echo "${kubelet_default_args}"
}

# function end

echo "$(date) Starting Custom kubelet bootstrap"

# Allow user to specify arguments through custom cloud-init
while [[ $# -gt 0 ]]; do
  key="$1"
  case "$key" in
    --kubelet-extra-args)
        export KUBELET_EXTRA_ARGS="$2"
        shift
        shift
        ;;
    --cluster-dns)
        export CLUSTER_DNS="$2"
        shift
        shift
        ;;
    --apiserver-endpoint)
        export APISERVER_ENDPOINT="$2"
        shift
        shift
        ;;
    --kubelet-ca-cert)
        export KUBELET_CA_CERT="$2"
        shift
        shift
        ;;
    *) # Ignore unsupported args
        shift
        ;;
  esac
done

KUBELET_EXTRA_ARGS="${KUBELET_EXTRA_ARGS:-}"
CLUSTER_DNS="${CLUSTER_DNS:-}"
APISERVER_ENDPOINT="${APISERVER_ENDPOINT:-}"
KUBELET_CA_CERT="${KUBELET_CA_CERT:-}"

# Use the bootstrap endpoint to allow the BYON node to attempt to join the cluster
echo "$KUBELET_CA_CERT" | base64 -d > /etc/kubernetes/ca.crt
get_oke_k8version > /etc/self-k8s/oke-k8s-version

# Get the pause image for the given region/realm/k8s version and populate the crio config
REGION="$(get_region)"
REALM="$(get_realm)"
K8S_VERSION=$(get_oke_k8version | awk -F'v' '{print $NF}')
PAUSE_IMAGE=$(get_pause_image "$REGION" "$REALM")
export K8S_VERSION PAUSE_IMAGE

# Get instance ocid for proxymux and kubelet configs
INSTANCE_ID="$(get_instance_id)"
export INSTANCE_ID

# Get info needed to populate the kubelet config
KUBELET_CONFIG=/etc/kubernetes/kubelet-config.json

# Get node labels placed by OKE, including user-specified initial node labels
NODE_LABELS="$(get_node_labels)"
export NODE_LABELS

# Get default kubelet args
KUBELET_DEFAULT_ARGS="$(get_kubelet_default_args)"
export KUBELET_DEFAULT_ARGS

# Path for kubelet drop-in files
KUBELET_SERVICE_D_PATH="/etc/systemd/system/kubelet.service.d"
mkdir -p "$KUBELET_SERVICE_D_PATH"

# Store default kubelet args and extra kubelet args in environment variables to be used by kubelet
service_env_rule "KUBELET_DEFAULT_ARGS" "$KUBELET_DEFAULT_ARGS" > "${KUBELET_SERVICE_D_PATH}"/kubelet-default-args.conf
service_env_rule "KUBELET_EXTRA_ARGS" "$KUBELET_EXTRA_ARGS" > "${KUBELET_SERVICE_D_PATH}"/kubelet-extra-args.conf

# Disable swap volumes
sed -i '/swap/ s/^\(.*\)$/# \1/g' /etc/fstab
swapoff -a

# Enable and restart necessary systemd services
daemon_reload
enable_and_restart "containerd"

export CLUSTER_DNS
echo "$(jq --arg CLUSTER_DNS "$CLUSTER_DNS" --arg INSTANCE_ID "$INSTANCE_ID" '. += {"clusterDNS": [$CLUSTER_DNS], "providerID": $INSTANCE_ID}' $KUBELET_CONFIG)" > ${KUBELET_CONFIG}

daemon_reload
enable_and_restart 'kubelet'
enable_and_restart 'systemd-journald'
sudo /usr/bin/sh -c '/usr/bin/rm -rf /var/lib/cni/networks/* && swapoff -a'

echo "$(date) Finished Custom kubelet bootstrap"