#!/bin/bash
set -xe
set -o pipefail
cat << 'INNER_EOF' | sudo tee /etc/sysctl.d/k8s.conf
net.ipv4.ip_forward = 1

INNER_EOF
sudo sysctl --system
sysctl net.ipv4.ip_forward

safe_apt_update() {
    local attempts=0
    local max_attempts=18  # Set the maximum number of attempts

    while [ $attempts -lt $max_attempts ]; do
        if sudo apt update; then  # Use sudo to run apt update
            echo "apt update succeeded!"
            return 0  # Exit the function successfully
        else
            echo "apt update failed, retrying..."
            attempts=$((attempts + 1))  # Increment the attempt counter
            sleep 10  # Wait for 5 seconds before retrying
        fi
    done

    echo "Reached the maximum number of attempts, exiting."
    return 1  # Exit the function with an error
}

# install containerd
safe_apt_update
sudo apt-get install ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
safe_apt_update
sudo apt-get install containerd.io
# install kubelet
sudo apt-get install -y apt-transport-https ca-certificates curl gpg
curl -fsSL https://pkgs.k8s.io/core:/stable:/{{ .KubernetesVersion }}/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
sudo chmod 644 /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/{{ .KubernetesVersion }}/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list
safe_apt_update
sudo apt-get install -y kubelet={{ .KubeletVersion }}
sudo apt-mark hold kubelet
# install jq
sudo apt-get install -y jq
# install nfs
sudo apt-get install -y nfs-common
