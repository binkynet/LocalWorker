# BinkyNet Local Worker

This is an implementation of the BinkyNet Local Worker's "brains", intended to be
run of Raspberry PI like platforms.

## Raspberry Pi Configuration

In `/boot/config.txt` add:

```text
# Enabled i2c
dtparam=i2c_arm=on
```

In `/etc/modules` add:

```text
i2c-bcm2708
i2c-dev
```

## Orange Pi Zero Configuration

In `/boot/armbianEnv.txt` add:

```text
overlays=<what is already there> w1-gpio uart1 i2c0 spi-spidev
```

In `/etc/modules` add:

```text
i2c-bcm2708
i2c-dev
```

## Generic Configuration

Add current user to i2c group

```bash
sudo adduser ${USER} i2c
```

Optionally install these tools:

```bash
sudo apt-get update
sudo apt-get install python-smbus i2c-tools
```

## Run local worker

```bash
./bnLocalWorker -b opz|rpi [-i ethX]
```
