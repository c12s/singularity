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

## Layout

```
cmd/singularity/   file server (stdlib-only Go, PUT/GET /artifacts)
config/zot-config.json   Zot configuration (mounted into the zot container)
Dockerfile         file server image build
```

## Local run (via tools compose)

```bash
cd tools && bash start.sh    # starts singularity (:5000) + singularity-files (:8080)
```

## Direct usage

```bash
# push an OCI image (normally done by Supernova)
podman push localhost:5000/os/edge-base:v1.0

# upload a qcow2 artifact
curl -T disk.qcow2 http://singularity:8080/artifacts/edge-node-v1.0.qcow2
# -> {"name":"edge-node-v1.0.qcow2","checksum":"sha256:..."}

# node side: download + verify
curl -O http://singularity:8080/artifacts/edge-node-v1.0.qcow2
curl http://singularity:8080/artifacts/edge-node-v1.0.qcow2/sha256
```
