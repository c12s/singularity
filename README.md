# singularity — c12s artifact registry

One store for everything a node consumes over the network (decision **D6**):

| Part | Image / code | Port | Serves |
|---|---|---|---|
| **OCI registry** | [`project-zot/zot`](https://github.com/project-zot/zot) (off-the-shelf, OCI-compliant, cosign-aware) | `5000` | bootc OS images (`bootc switch` pulls from here) |
| **Artifact file server** | this repo (`cmd/singularity`) | `8080` | qcow2 images + checksums (VM provisioning, Phase 4) |

Zot is deliberately kept boring and stateless-in-config: anonymous read/write
on the internal network (PoC; auth config slot ready in `config/`), storage in
`/var/lib/registry`. The file server computes a `sha256:` checksum on upload
and stores it next to the artifact — Star's VM reconciler verifies integrity
before booting a qcow2 (plan §architecture, `qcow2.go`).

> **OCI-only:** Zot v2 rejects docker schema2 manifests (HTTP 415). Push with
> podman/skopeo (OCI on the wire). The Supernova engine path handles this
> automatically (skopeo conversion).

## Layout

```
cmd/singularity/   file server (stdlib-only Go: GET /artifacts list,
                   PUT/GET /artifacts/{name}, GET /artifacts/{name}/sha256)
config/zot-config.json   Zot configuration (mounted into the zot container)
Dockerfile         file server image build
```

## Local run (via tools compose)

```bash
cd tools && bash start.sh    # starts singularity (:5000) + singularity-files (:8080)
```

In compose, `singularity` is the off-the-shelf Zot image (`ghcr.io/project-zot/zot-linux-amd64:v2.1.2`)
with `config/zot-config.json` mounted and a persistent volume
(`zotregistry`) for `/var/lib/registry`; `singularity-files` is this
repository's Go file server with the `artifacts` volume. Both are plain
HTTP on the internal network; the host ports come from `tools/.env` (lab:
registry `5001`, files `8081`). Nodes reach them as `192.168.122.1:5001`
(the image's `IMAGE_REGISTRY`) and `http://192.168.122.1:8081` (its
`DISK_REGISTRY_URL`).

## Direct usage

```bash
# push an OCI image (normally done by Supernova)
podman push localhost:5000/os/edge-base:v1.0

# list artifacts (JSON array of names)
curl http://singularity:8080/artifacts

# upload a qcow2 artifact
curl -T disk.qcow2 http://singularity:8080/artifacts/edge-node-v1.0.qcow2
# -> {"name":"edge-node-v1.0.qcow2","checksum":"sha256:..."}

# node side: download + verify
curl -O http://singularity:8080/artifacts/edge-node-v1.0.qcow2
curl http://singularity:8080/artifacts/edge-node-v1.0.qcow2/sha256
```

## Status

Stable for the lab: every node image and VM disk is served from here (Zot
registry + artifact file server). Zot garbage-collects blobs no tag refers
to (`"gc": true`). Auth, TLS and artifact deletion are deliberately out of
scope for the thesis.

## Bugs, disabled and unfinished features

### Confirmed bugs

None known in the file server; the reserved-suffix guard (review S5) and the
sha256-sidecar write path are covered by the checksum tests in Star and were
exercised by every VM provisioning in the lab.

### Disabled / unfinished features

- **No authentication anywhere** — Zot accepts anonymous push/pull and the
  file server accepts anonymous `PUT` (review S1/S5): anyone on the node
  network can replace an artifact's bytes. Checksums are recomputed on
  upload, so the *sidecar cannot be forged directly* (reserved suffixes are
  rejected), but an artifact can still be **overwritten** together with its
  checksum — a spec without a pinned digest would trust the new checksum.
  Lab mitigation: `cockpit vms set` requires a digest per VM, and
  `cockpit image apply` pins the checksum it reads when the spec is applied.
- **No artifact delete**: the file server has no `DELETE`, so qcow2 disks
  (~1.3–1.7 GB each) accumulate in the `artifacts` volume; the lab reclaims
  space via `stop.sh --clean` (`tools/image-cache.sh` saves and restores
  what is worth keeping). Registry blobs are reclaimed by Zot's GC once their
  tag is deleted.
- **No TLS**: registry and file server are plain HTTP; nodes are configured
  with `insecure` pulls for the lab registry (`192.168.122.1:5001`).
- Zot auth config slot exists but is empty (`config/zot-config.json` has no
  `http.auth` section) — enabling auth is a v2 task.

### PoC boundaries (by design, not bugs)

- Zot is OCI-only here: docker schema2 pushes are rejected (HTTP 415) —
  always push via podman/skopeo (DEV-8).
- `registry.internal:5001` appears in older node output but does not resolve;
  use `192.168.122.1:5001` from guests, `singularity:5000` inside compose.
