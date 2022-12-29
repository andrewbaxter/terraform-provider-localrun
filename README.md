This provider runs a command locally.

Unlike using `null_resource` with `local-exec`, this runs the build every time terraform is invoked. This works well with external caching build systems with fast no-op builds, like `make`, `go build`, `cargo`, etc.

It has an optional `outputs` path attribute which is a list of files that will be hashed when the build completes, placed in `output_hashes`. You can use these as an input for other resources.

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
