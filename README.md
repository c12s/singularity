# singularity — c12s artifact registry

One store for everything a node consumes over the network:

| Part | Image / code | Port | Serves |
|---|---|---|---|
| **OCI registry** | [`project-zot/zot`](https://github.com/project-zot/zot) (off-the-shelf, OCI-compliant, cosign-aware) | `5000` | bootc OS images (`bootc switch` pulls from here) |
| **Artifact file server** | this repo (`cmd/singularity`) | `8080` | qcow2 images + checksums (VM provisioning) |

Zot is deliberately kept boring and stateless-in-config: anonymous read/write
on the internal network (an auth config slot is ready in `config/`), storage in
`/var/lib/registry`. The file server computes a `sha256:` checksum on upload
and stores it next to the artifact — Star's VM reconciler verifies integrity
before booting a qcow2.

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
HTTP on the internal network; the host ports come from `tools/.env` (registry
`5001`, files `8081` by default). Nodes reach them through the addresses in
their image's `node.env` (`IMAGE_REGISTRY`, `DISK_REGISTRY_URL`).

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
