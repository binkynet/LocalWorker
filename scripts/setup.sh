#!/bin/bash

# Setup access rights
sudo usermod -aG sudo ${USER}
echo "${USER} ALL=(ALL) NOPASSWD: ALL" | sudo tee /etc/sudoers.d/${USER}
sudo chmod 0440 /etc/sudoers.d/${USER}
sudo usermod -aG i2c ${USER}

# Download worker binary
sudo mkdir -p /opt/binky
sudo chown ${USER} /opt/binky/
# TODO make release available

# Create worker service
cat >/etc/systemd/system/bnLocalWorker.service <<EOF
[Unit]
Description=bnLocalWorker

[Service]
Type=notify
Restart=always
RestartSec=5s
LimitNOFILE=40000
TimeoutStartSec=0
Environment=BRIDGETYPE=opz

ExecStart=/opt/binky/bnLocalWorker --bridge=${BRIDGETYPE}

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl restart bnLocalWorker
