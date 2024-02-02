This provider provides tools for running commands locally.

This provides two options:

- The datasource `Run` executes the command during the `Read` phase, every time you run Terraform (unlike `local-exec` with `null_resource`). This is for things like `Makefile` which should have quick no-op builds.

- The resource `Run` executes the command if any parameters or explicit dependencies change. You can also do this with the `local-run` provisioner, but this can be more convenient.

Both allow you to list file outputs and will calculate the hash for outputs if you need them for another command.

# Installation with Terraform CDK

Run

```
cdktf provider add andrewbaxter/localrun
```

# Installation with Terraform

See the dropdown on the Registry page.

# Documentation

See the Registry or look at `docs/`.

# Building

Make sure git submodules are cloned and up to date with `git submodule update --init`.

Run

```
./build.sh
```

This will generate the source files and render the docs.

# Technical Details

This uses a fake `_auto_update` attribute which synthetically changes every time the provider's `Read` operation is called, causing Terraform to believe the resource is out of sync and needs an update.
