# OCI Image Explorer API Documentation

This document describes the REST API provided by OCI Image Explorer.

## Base URL

```
http://localhost:8080/api
```

The base URL depends on your deployment configuration. By default, the server runs on port 8080.

## Authentication

**No authentication required.** The API is designed for local use. Registry authentication is handled server-side using the Docker credential keychain.

## Endpoints

### GET /api/health

Health check endpoint to verify the server is running.

#### Request

```http
GET /api/health HTTP/1.1
Host: localhost:8080
```

#### Response

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "success": true,
  "data": {
    "status": "healthy",
    "platform": "darwin/arm64",
    "version": "dev"
  }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `success` | boolean | Always `true` for successful responses |
| `data.status` | string | Server status, always `"healthy"` |
| `data.platform` | string | Server platform (OS/architecture) |
| `data.version` | string | Application version |

---

### GET /api/inspect

Inspect a container image and retrieve its complete OCI structure.

#### Request

```http
GET /api/inspect?image=alpine:latest HTTP/1.1
Host: localhost:8080
```

#### Query Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `image` | Yes | Image reference (e.g., `alpine:latest`, `ghcr.io/org/repo:tag`, `registry.io/image@sha256:...`) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "success": true,
  "data": {
    "repository": "index.docker.io/library/alpine",
    "tag": "latest",
    "digest": "sha256:c5b1261d6d3e43071626931fc004f70149baeba2c8ec672bd4f27761f8e1ad6b",
    "created": "2024-01-27T00:30:48Z",
    "architecture": "amd64",
    "os": "linux",
    "platformDigest": "sha256:6457d53fb065d6f250e1504b9bc42d5b6c65941d57532c072d929dd0628977d0",
    "imageIndex": {
      "schemaVersion": 2,
      "mediaType": "application/vnd.oci.image.index.v1+json",
      "manifests": [
        {
          "mediaType": "application/vnd.oci.image.manifest.v1+json",
          "digest": "sha256:6457d53fb065d6f250e1504b9bc42d5b6c65941d57532c072d929dd0628977d0",
          "size": 585,
          "platform": {
            "architecture": "amd64",
            "os": "linux"
          }
        },
        {
          "mediaType": "application/vnd.oci.image.manifest.v1+json",
          "digest": "sha256:b229a85166aadbde58e73e03ebf2a5c0f0a4e6c23e2f5d9db2e4a7e9e65f6f6f",
          "size": 585,
          "platform": {
            "architecture": "arm64",
            "os": "linux"
          }
        }
      ]
    },
    "manifest": {
      "schemaVersion": 2,
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "config": {
        "mediaType": "application/vnd.oci.image.config.v1+json",
        "digest": "sha256:05455a08881ea9cf0e752bc48e61bbd71a34c029bb13df01e40e3e70e0d007bd",
        "size": 581
      },
      "layers": [
        {
          "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
          "digest": "sha256:4abcf20661432fb2d719aaf90656f55c287f8ca915dc1c92ec14ff61e67fbaf8",
          "size": 3408729
        }
      ]
    },
    "config": {
      "created": "2024-01-27T00:30:48Z",
      "architecture": "amd64",
      "os": "linux",
      "config": {
        "Env": [
          "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
        ],
        "Cmd": ["/bin/sh"]
      },
      "rootfs": {
        "type": "layers",
        "diff_ids": [
          "sha256:d4fc045c9e3a848011de66f34b81f052d4f2c15a17bb196d637e526349601820"
        ]
      },
      "history": [
        {
          "created": "2024-01-27T00:30:48Z",
          "created_by": "/bin/sh -c #(nop) ADD file:... in / "
        },
        {
          "created": "2024-01-27T00:30:48Z",
          "created_by": "/bin/sh -c #(nop)  CMD [\"/bin/sh\"]",
          "empty_layer": true
        }
      ]
    },
    "tags": ["latest"],
    "referrers": [
      {
        "type": "sbom",
        "mediaType": "application/vnd.oci.image.manifest.v1+json",
        "digest": "sha256:a1b2c3d4e5f6...",
        "size": 1234567,
        "artifactType": "https://spdx.dev/Document",
        "annotations": {
          "in-toto.io/predicate-type": "https://spdx.dev/Document",
          "vnd.docker.reference.digest": "sha256:6457d53fb065d6f250e1504b9bc42d5b6c65941d57532c072d929dd0628977d0",
          "vnd.docker.reference.type": "attestation-manifest"
        }
      }
    ]
  }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `repository` | string | Full repository name including registry |
| `tag` | string | Image tag (if provided in reference) |
| `digest` | string | Image digest (index digest for multi-platform) |
| `created` | string | ISO 8601 creation timestamp |
| `architecture` | string | Primary platform architecture |
| `os` | string | Primary platform OS |
| `platformDigest` | string | Digest of the resolved platform manifest |
| `imageIndex` | object | Image index (for multi-platform images) |
| `manifest` | object | Platform manifest |
| `config` | object | Image configuration |
| `tags` | array | List of tags |
| `referrers` | array | List of referrers (SBOMs, attestations, etc.) |

#### ImageIndex Object

| Field | Type | Description |
|-------|------|-------------|
| `schemaVersion` | integer | Schema version (always 2) |
| `mediaType` | string | Media type of the index |
| `manifests` | array | List of platform manifests |
| `annotations` | object | Optional annotations |

#### IndexManifest Object

| Field | Type | Description |
|-------|------|-------------|
| `mediaType` | string | Manifest media type |
| `digest` | string | Manifest digest |
| `size` | integer | Manifest size in bytes |
| `platform` | object | Platform specification |
| `annotations` | object | Optional annotations |
| `artifactType` | string | Artifact type (for OCI 1.1 artifacts) |

#### Platform Object

| Field | Type | Description |
|-------|------|-------------|
| `architecture` | string | CPU architecture (amd64, arm64, etc.) |
| `os` | string | Operating system (linux, windows, etc.) |
| `variant` | string | Optional variant (v7, v8, etc.) |

#### Manifest Object

| Field | Type | Description |
|-------|------|-------------|
| `schemaVersion` | integer | Schema version (always 2) |
| `mediaType` | string | Manifest media type |
| `config` | object | Config descriptor |
| `layers` | array | Layer descriptors |
| `annotations` | object | Optional annotations |

#### Descriptor Object

| Field | Type | Description |
|-------|------|-------------|
| `mediaType` | string | Content media type |
| `digest` | string | Content digest |
| `size` | integer | Content size in bytes |
| `annotations` | object | Optional annotations |

#### Config Object

| Field | Type | Description |
|-------|------|-------------|
| `created` | string | Creation timestamp |
| `author` | string | Image author |
| `architecture` | string | Target architecture |
| `os` | string | Target OS |
| `config` | object | Container configuration |
| `rootfs` | object | Root filesystem information |
| `history` | array | Build history entries |

#### ContainerConfig Object

| Field | Type | Description |
|-------|------|-------------|
| `User` | string | Default user |
| `ExposedPorts` | object | Exposed ports map |
| `Env` | array | Environment variables |
| `Entrypoint` | array | Container entrypoint |
| `Cmd` | array | Default command |
| `WorkingDir` | string | Working directory |
| `Labels` | object | Image labels |

#### Referrer Object

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Classified type: `sbom`, `signature`, `attestation`, `vex`, `vulnerability-scan`, `artifact` |
| `mediaType` | string | Artifact media type |
| `digest` | string | Artifact digest |
| `size` | integer | Artifact size in bytes |
| `artifactType` | string | OCI artifact type URI |
| `annotations` | object | Artifact annotations |
| `signatureInfo` | object | Cosign signature details (only for signatures with Sigstore certificates) |

#### SignatureInfo Object

Present on signature referrers that have a Sigstore certificate.

| Field | Type | Description |
|-------|------|-------------|
| `issuer` | string | OIDC issuer from Sigstore certificate extension (e.g., `https://token.actions.githubusercontent.com`) |
| `identity` | string | Certificate identity — email or URI SAN (e.g., GitHub Actions workflow URL) |

#### Error Response

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "success": false,
  "error": "image parameter is required"
}
```

```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "success": false,
  "error": "failed to fetch image: GET https://index.docker.io/v2/library/nonexistent/manifests/latest: MANIFEST_UNKNOWN: manifest unknown"
}
```

---

### GET /api/tags

List all tags in a repository.

#### Request

```http
GET /api/tags?repository=library/alpine HTTP/1.1
Host: localhost:8080
```

#### Query Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `repository` | Yes | Repository name (e.g., `library/alpine`, `ghcr.io/owner/repo`) |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "success": true,
  "data": [
    "3.14",
    "3.15",
    "3.16",
    "3.17",
    "3.18",
    "3.19",
    "edge",
    "latest"
  ]
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `success` | boolean | `true` for successful responses |
| `data` | array | List of tag strings |

#### Error Response

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "success": false,
  "error": "repository parameter is required"
}
```

```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "success": false,
  "error": "NAME_UNKNOWN: repository name not known to registry"
}
```

---

### GET /api/sbom

Download SBOM content from an attestation manifest.

#### Request

```http
GET /api/sbom?repository=index.docker.io/library/alpine&digest=sha256:a1b2c3d4... HTTP/1.1
Host: localhost:8080
```

#### Query Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `repository` | Yes | Full repository name |
| `digest` | Yes | Digest of the attestation manifest containing the SBOM |

#### Response (Success)

The response is the raw SBOM content (typically JSON), not wrapped in the standard API response format.

```http
HTTP/1.1 200 OK
Content-Type: application/json
Content-Disposition: attachment; filename="sbom-a1b2c3d4e5f6.json"

{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "alpine",
  "documentNamespace": "https://example.com/alpine",
  "packages": [
    {
      "SPDXID": "SPDXRef-Package-alpine-base",
      "name": "alpine-base",
      "versionInfo": "3.19.0-r0",
      ...
    }
  ],
  ...
}
```

#### Response Headers

| Header | Description |
|--------|-------------|
| `Content-Type` | `application/json` (or detected media type) |
| `Content-Disposition` | `attachment; filename="sbom-<digest-prefix>.json"` |

#### Processing Details

1. Fetches the attestation manifest at the specified digest
2. Searches layers for SBOM predicate types:
   - SPDX (`spdx`)
   - CycloneDX (`cyclonedx`)
   - Syft (`syft`, `sbom`)
3. Extracts the layer blob
4. If the blob is an in-toto attestation envelope, extracts the `predicate` field
5. Returns the SBOM content formatted as JSON

#### Error Response

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "success": false,
  "error": "repository and digest parameters are required"
}
```

```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "success": false,
  "error": "no SBOM layer found in attestation manifest"
}
```

---

### GET /api/vex

Fetch and parse a VEX (Vulnerability Exploitability eXchange) document from an attestation.

#### Request

```http
GET /api/vex?repository=ghcr.io/hkolvenbach/oci-explorer&digest=sha256:a1b2c3d4... HTTP/1.1
Host: localhost:8080
```

#### Query Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `repository` | Yes | Full repository name |
| `digest` | Yes | Digest of the attestation manifest containing the VEX document |

#### Response (Success)

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "success": true,
  "data": {
    "@context": "https://openvex.dev/ns/v0.2.0",
    "@id": "https://example.com/vex/2024-01-15/1",
    "author": "Example Security Team <security@example.com>",
    "timestamp": "2024-01-15T10:00:00Z",
    "last_updated": "2024-01-16T12:00:00Z",
    "version": 2,
    "statements": [
      {
        "vulnerability": {
          "name": "CVE-2023-44487"
        },
        "products": [{"@id": "pkg:oci/myimage@sha256:abc123"}],
        "status": "not_affected",
        "status_notes": "govulncheck confirms this code path is not reachable",
        "justification": "vulnerable_code_not_present",
        "impact_statement": "The HTTP/2 rapid reset vulnerability does not affect this image."
      }
    ]
  }
}
```

#### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `@context` | string | OpenVEX context URI |
| `@id` | string | Unique document identifier |
| `author` | string | Document author |
| `timestamp` | string | Document creation timestamp (ISO 8601) |
| `last_updated` | string | Document last updated timestamp |
| `version` | integer | Document version number |
| `statements` | array | VEX statements |

#### VEXStatement Object

| Field | Type | Description |
|-------|------|-------------|
| `vulnerability` | object | Contains `name` (CVE ID or vulnerability identifier) |
| `products` | array | Product identifiers, each with `@id` (typically a purl) |
| `status` | string | One of: `not_affected`, `affected`, `fixed`, `under_investigation` |
| `status_notes` | string | Additional notes about the status determination |
| `justification` | string | Justification when status is `not_affected` |
| `impact_statement` | string | Human-readable impact description |
| `timestamp` | string | Statement-level timestamp (if different from document) |

#### Processing Details

1. Fetches the attestation manifest at the specified digest
2. Searches layers for VEX/OpenVEX predicate types
3. Extracts the layer blob (handles DSSE envelope and in-toto attestation formats)
4. Parses the VEX document from the `predicate` field
5. Returns the structured VEX document

#### Error Response

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "success": false,
  "error": "repository and digest parameters are required"
}
```

```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{
  "success": false,
  "error": "no VEX layer found in attestation manifest"
}
```

---

## Error Handling

All API responses follow a consistent format:

### Success Response

```json
{
  "success": true,
  "data": { ... }
}
```

### Error Response

```json
{
  "success": false,
  "error": "Error message describing what went wrong"
}
```

### HTTP Status Codes

| Status Code | Meaning |
|-------------|---------|
| `200 OK` | Request successful |
| `400 Bad Request` | Missing or invalid parameters |
| `500 Internal Server Error` | Server-side error (registry errors, parsing failures) |

### Common Error Messages

| Error | Cause |
|-------|-------|
| `image parameter is required` | Missing `image` query parameter |
| `repository parameter is required` | Missing `repository` query parameter |
| `repository and digest parameters are required` | Missing SBOM download parameters |
| `invalid image reference: ...` | Malformed image reference |
| `invalid repository: ...` | Malformed repository name |
| `failed to fetch image: ...` | Registry communication error |
| `no SBOM layer found in attestation manifest` | Attestation doesn't contain SBOM |
| `no VEX layer found in attestation manifest` | Attestation doesn't contain VEX |

---

## CORS

The API includes CORS headers for browser access:

```http
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, OPTIONS
Access-Control-Allow-Headers: Content-Type
```

Preflight `OPTIONS` requests return `200 OK` with appropriate headers.

---

## Examples

### cURL Examples

**Health Check:**
```bash
curl http://localhost:8080/api/health
```

**Inspect Image:**
```bash
curl "http://localhost:8080/api/inspect?image=alpine:latest"
```

**List Tags:**
```bash
curl "http://localhost:8080/api/tags?repository=library/nginx"
```

**Download SBOM:**
```bash
curl -o sbom.json "http://localhost:8080/api/sbom?repository=index.docker.io/library/alpine&digest=sha256:abc123..."
```

**Fetch VEX:**
```bash
curl "http://localhost:8080/api/vex?repository=ghcr.io/hkolvenbach/oci-explorer&digest=sha256:abc123..."
```

### JavaScript Examples

**Fetch Image Info:**
```javascript
async function inspectImage(imageRef) {
  const response = await fetch(
    `/api/inspect?image=${encodeURIComponent(imageRef)}`
  );
  const result = await response.json();

  if (!result.success) {
    throw new Error(result.error);
  }

  return result.data;
}

// Usage
const imageInfo = await inspectImage('nginx:latest');
console.log(`Image has ${imageInfo.manifest.layers.length} layers`);
```

**Download SBOM:**
```javascript
async function downloadSBOM(repository, digest) {
  const response = await fetch(
    `/api/sbom?repository=${encodeURIComponent(repository)}&digest=${encodeURIComponent(digest)}`
  );

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error);
  }

  const blob = await response.blob();
  const url = URL.createObjectURL(blob);

  const a = document.createElement('a');
  a.href = url;
  a.download = `sbom-${digest.substring(7, 19)}.json`;
  a.click();

  URL.revokeObjectURL(url);
}
```
