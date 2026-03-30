# Hosting Alternatives Research (March 2026)

Current setup: Fly.io shared-cpu-1x, 512MB, Frankfurt — ~$3.60/mo.

## Problem

Shared CPU is fine for inspect/SBOM/VEX (metadata-only, <200KB per request). Trivy vulnerability scans are CPU-intensive (layer decompression + filesystem walk) and time out on shared CPU for images >100MB (nginx times out at 5 min, twice).

## Managed PaaS Comparison

| Platform | Built-in metrics? | Dashboard | Dedicated CPU | Price (dedicated) | Deploy model |
|---|---|---|---|---|---|
| **Fly.io** | Yes | Full Grafana + Prometheus endpoint | Yes | $32/mo (1 vCPU, 2GB) | fly.toml, `fly deploy` |
| **Northflank** | Yes | Real-time metrics + logs, all plans | Yes | $24/mo (1 vCPU, 2GB) | Dockerfile from git |
| **Railway** | Basic | Logs + usage metrics | No | $5-20 usage-based (shared) | Git push |
| **Render** | Basic | Logs + deploy history | No (all tiers shared) | $25/mo (1 shared, 2GB) | render.yaml from git |
| **Koyeb** | Basic | Logs + basic metrics | Partial (0.25 dedicated/GB) | $11/mo (1 vCPU, 1GB) | Git push or Docker |

### Notes

- **Fly.io** has the best integrated observability (Grafana dashboards, Prometheus scraping, live log tailing). No other managed PaaS offers a full Grafana instance.
- **Northflank** is the closest alternative — built-in metrics on all plans, dedicated CPU at $24/mo ($8 cheaper than Fly.io for equivalent specs).
- **Railway** has the simplest DX but all CPU is shared/burstable — unsuitable for sustained Trivy scans.
- **Render** — all tiers are shared CPU, even the $225/mo Pro Max.
- **Namespace.so** — CI/CD and ephemeral compute only (instances auto-shut after 15 min). Not suitable for long-running services.

## Self-Hosted Option: Hetzner + Coolify

| Hetzner tier | Spec | Price/mo |
|---|---|---|
| CX23 (shared) | 2 vCPU, 4GB RAM, 40GB | €3.99 |
| CPX22 (shared, AMD) | 2 vCPU, 4GB RAM, 80GB | €7.99 |
| **CCX13 (dedicated)** | **2 dedicated vCPU, 8GB RAM, 80GB** | **€15.99 (~$17)** |

Coolify (free, self-hosted PaaS) provides git-push deploys, auto-SSL, Docker-native deployment, and a basic log/metrics UI. Install with one curl command.

**Trade-off**: Best CPU per dollar (2 dedicated vCPU + 8GB for $17 vs $32 on Fly.io), but you manage OS updates, backups, and monitoring yourself. No integrated Grafana — you'd wire up Prometheus + Grafana manually.

## Decision (March 2026)

Stay on Fly.io shared-cpu-1x for now. Mitigate scan performance via:
1. Trivy DB S3 caching (eliminates cold-start DB download penalty)
2. Response caching (repeat scans served from S3)
3. Consider SBOM-first scanning (avoids full image download for images with SBOM referrers)

Revisit if scan timeouts become a frequent user-facing problem — upgrade to Fly.io performance-1x ($32/mo) or migrate to Hetzner CCX13 + Coolify ($17/mo).
