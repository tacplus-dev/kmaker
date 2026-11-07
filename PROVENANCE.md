# Provenance

Copied **wholesale** on 2026-10-23 from `gitlab.com/seabeam/schematic/platform/services/kmaker` at commit
`0d4be84` (its own nested git repository; history carried over), with `proto/services/kmaker.proto` and
`proto/dependencies.yaml` from `platform/proto` at `ca7c322` on branch `big8-p1-messaging`. Build outputs not copied.

Jody, 2026-10-23: *"We will copy kernel over to its github home and only make changes to it. we will
keep these as ships in the night for now. For the new stuff we want to do it the right way."* and
*"the following will also need to be ported over to tacplus-dev ... blobstore gateway kmaker letsencrypt
eventbus oauth"*. The private copy runs the launch fleet unchanged; new-design changes land here.

Divergences: headers in tacplus.yaml (Jody Scott, Apache-2.0; LICENSE added); `src: proto` (this repo
holds the service's own proto); `proto/registry.yaml` is a lock of github.com/tacplus-dev/registry.
Since 2026-11-07 (#3) nothing refers to SeaBeam: the module is `github.com/tacplus-dev/kmaker` (was
`gitlab.com/seabeam/schematic/platform/services/kmaker`), generated into `tacplus/` (was `model/`); the proto stays
vendored here and the registry's contract is this repository (was `platform/proto`); the proto's header and the
image labels name Jody Scott (was Seabeam LLC); `proto/dependencies.yaml` lost its SeaBeam `registry:` block; and
SeaBeam's `.gitlab-ci.yml` (its registry, runners and private modules) is removed, builds being tacplus-dev's.
