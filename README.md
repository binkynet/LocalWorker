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

Run setup:

```bash
curl https://raw.githubusercontent.com/binkynet/LocalWorker/master/scripts/setup.sh | bash
```
