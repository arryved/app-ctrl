# app-controld

Part of Deploy 2.0. This is a daemon that can execute app-control actions on regular VMs.

## Build

```
make
```
...this builds a binary w/o specifying arch; defaults to system arch.

## Test

```
make test
```

You can additionally see a coverage report with tests, run:
```
make coverage
```

## Package

We build this as a debian package in the build/ directory:
```
make package
```
This task specifically builds a linux/amd64 binary.

## Deploy
This will deploy the package to the `arryved-apt` repo:
```
make deploy
```

## Install

See the app_controld task in the cfg-mgmt repo:
```
ap -i inventory.yaml tasks/app_controld.yaml --limit [expression]
```

...this includes a config. You will have to create an SSL keypair and put it in /etc/ssl/daemon/; this
can/should be automated when we set up internal ACME.
