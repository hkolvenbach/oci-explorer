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

/** Maps a supply chain grade to its hex color. */
export function gradeColor(grade: string): string {
  switch (grade) {
    case 'A+': return '#22c55e';
    case 'A': return '#4ade80';
    case 'B': return '#facc15';
    case 'C': return '#fb923c';
    default: return '#f87171';
  }
}
