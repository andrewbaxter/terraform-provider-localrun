#!/usr/bin/bash -xeu
rm -rf docs
go run -modfile tools.mod github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name localrun --rendered-provider-name "Local Run" --rendered-website-dir docs