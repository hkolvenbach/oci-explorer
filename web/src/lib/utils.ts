import type { ImageInfo, SecurityScoreResult, MinimalBaseDetails } from './types';

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export function truncateDigest(digest: string, length = 12): string {
  if (!digest) return '';
  const parts = digest.split(':');
  if (parts.length === 2) {
    return `${parts[0]}:${parts[1].substring(0, length)}...`;
  }
  return digest.substring(0, length) + '...';
}

export function downloadBlob(blob: Blob, filename: string): void {
  const url = window.URL.createObjectURL(blob);
  try {
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  } finally {
    window.URL.revokeObjectURL(url);
  }
}

function computeMinimalBaseScore(data: ImageInfo): {
  score: number;
  details: MinimalBaseDetails;
} {
  let score = 0;
  const details: MinimalBaseDetails = {
    fewLayers: false,
    smallSize: false,
    nonRoot: false,
    noShellEntrypoint: false,
  };
  const layerCount = data.manifest?.layers?.length || 0;
  const totalSize = data.manifest?.layers?.reduce((sum, l) => sum + l.size, 0) || 0;
  const config = data.config?.config;

  if (layerCount <= 5) {
    score += 0.5;
    details.fewLayers = true;
  }
  if (totalSize <= 50 * 1024 * 1024) {
    score += 0.5;
    details.smallSize = true;
  }
  if (config?.User && config.User !== '0' && config.User !== 'root') {
    score += 0.5;
    details.nonRoot = true;
  }
  const ep = (config?.Entrypoint || []).join(' ');
  if (ep && !/\b(sh|bash|ash|zsh)\b/.test(ep)) {
    score += 0.5;
    details.noShellEntrypoint = true;
  }

  return { score, details };
}

export function computeSecurityScore(data: ImageInfo): SecurityScoreResult {
  const referrers = data.referrers || [];
  const criteria = [
    {
      key: 'signature',
      label: 'Signature',
      desc: 'Image is signed (Cosign/Notary)',
      present: referrers.some((r) => r.type === 'signature'),
    },
    {
      key: 'attestation',
      label: 'Attestation',
      desc: 'Build provenance attestation (SLSA)',
      present: referrers.some((r) => r.type === 'attestation'),
    },
    {
      key: 'sbom',
      label: 'SBOM',
      desc: 'Software Bill of Materials attached',
      present: referrers.some((r) => r.type === 'sbom'),
    },
    {
      key: 'vex',
      label: 'VEX',
      desc: 'Vulnerability Exploitability eXchange document',
      present: referrers.some((r) => r.type === 'vex'),
    },
  ];

  let score = 0;
  criteria.forEach((c) => {
    if (c.present) score += 2;
  });

  const minimalBase = computeMinimalBaseScore(data);
  score += minimalBase.score;
  criteria.push({
    key: 'minimalBase',
    label: 'Minimal Base',
    desc: 'Few layers, small size, non-root, no shell entrypoint',
    present: minimalBase.score >= 1,
  });

  const maxScore = 10;
  let grade: string, color: string, colorClass: string;
  if (score >= 10) {
    grade = 'A+';
    color = '#22c55e';
    colorClass = 'text-green-500';
  } else if (score >= 8) {
    grade = 'A';
    color = '#4ade80';
    colorClass = 'text-green-400';
  } else if (score >= 6) {
    grade = 'B';
    color = '#facc15';
    colorClass = 'text-yellow-400';
  } else if (score >= 4) {
    grade = 'C';
    color = '#fb923c';
    colorClass = 'text-orange-400';
  } else {
    grade = 'D';
    color = '#f87171';
    colorClass = 'text-red-400';
  }

  return {
    score,
    maxScore,
    grade,
    color,
    colorClass,
    criteria,
    minimalBaseDetails: minimalBase.details,
  };
}
