# BinkyNet Local Worker 

This is an implementation of the BinkyNet Local Worker's "brains", intended to be
run of Raspberry PI like platforms.

## Usage 

```
docker run -it -v /sys:/sys -v /dev/i2c-1:/dev/i2c-1 binkynet/localworker 
```

## Raspberry Pi Configuration 

In `/boot/config.txt` add:

```
# Enabled i2c
dtparam=i2c_arm=on
```

In `/etc/modules` add:

```
i2c-bcm2708
i2c-dev
```

Optionally install these tools:

```
sudo apt-get update
sudo apt-get install python-smbus i2c-tools
```
