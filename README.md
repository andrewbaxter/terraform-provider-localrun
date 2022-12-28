This provider runs a command locally.

Unlike using `null_resource` with `local-exec`, this is an "always-dirty" resource which will be updated every time you apply the stack. This makes it good for connecting to local builders like make, where the build is very fast if nothing changed.

It has an optional `output` path attribute which is a file that will be hashed when the build completes. You can use this as an input for other resources.

If the command has multiple outputs, you should use `hashicorp/local` resources with a `depends_on` the `run` resource and ignore `output`.

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
