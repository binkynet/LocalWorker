#!/bin/bash

# Setup access rights
sudo usermod -aG sudo ${USER}
echo "${USER} ALL=(ALL) NOPASSWD: ALL" | sudo tee /etc/sudoers.d/${USER}
sudo chmod 0440 /etc/sudoers.d/${USER}
sudo usermod -aG i2c ${USER}

sudo apt-get update -y
sudo apt-get install -y python-smbus i2c-tools

# Download worker binary
sudo mkdir -p /opt/binky
sudo chown ${USER} /opt/binky/
# TODO make release available

# Create worker service
cat | sudo tee /etc/systemd/system/bnLocalWorker.service <<EOF
[Unit]
Description=bnLocalWorker

[Service]
Type=simple
Restart=always
RestartSec=5s
LimitNOFILE=40000
TimeoutStartSec=0
ExecStart=/opt/binky/bnLocalWorker --bridge=opz

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl restart bnLocalWorker
