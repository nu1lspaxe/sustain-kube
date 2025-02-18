export DEBIAN_FRONTEND=noninteractive

# 1. Update & install firewall
sudo apt-get update
sudo apt-get upgrade
sudo apt-get install ufw
sudo ufw enable

# 2. Configure firewall rules
sudo ufw allow 22 # Vagrant ssh
sudo ufw allow 8285
sudo ufw allow 8472
sudo ufw allow 10250/tcp
sudo ufw allow 30000:32767/tcp
sudo ufw enable
sudo ufw reload

# 3. Enable kernel modules and disable swap 
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
overlay
br-netfilter
EOF

cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

sudo sysctl --system
sudo swapoff -a

# 4. Install containerd
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
sudo apt-get install containerd.io

# 5. Configure /etc/containerd/config.toml
systemctl enable containerd
mkdir -p /etc/containerd
sudo containerd config default | sudo tee /etc/containerd/config.toml


### Use sed to insert the line after the pattern
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml

### Restart the containerd service to apply changes
systemctl restart containerd
systemctl enable containerd


# 6. Install kubeadm
sudo apt-get install -y apt-transport-https ca-certificates curl gnupg
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.32/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
sudo chmod 644 /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.32/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list
sudo chmod 644 /etc/apt/sources.list.d/kubernetes.list
sudo apt-get update
sudo apt-get install -y kubectl kubeadm kubelet
sudo apt-mark hold kubelet kubeadm kubectl
