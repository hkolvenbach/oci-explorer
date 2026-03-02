// Referrer type → Tailwind classes for badges/cards
export const referrerTypeClasses: Record<string, string> = {
  signature: 'bg-amber-500/20 text-amber-300 border-amber-500/30',
  sbom: 'bg-cyan-500/20 text-cyan-300 border-cyan-500/30',
  attestation: 'bg-violet-500/20 text-violet-300 border-violet-500/30',
  vex: 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30',
  artifact: 'bg-slate-500/20 text-slate-300 border-slate-500/30',
};

// Referrer type → SVG fill color + label for graph nodes
export const referrerTypeGraph: Record<string, { fill: string; text: string }> = {
  signature: { fill: '#f59e0b', text: 'Signature' },
  sbom: { fill: '#06b6d4', text: 'SBOM' },
  attestation: { fill: '#8b5cf6', text: 'Attestation' },
  vex: { fill: '#2dd4bf', text: 'VEX' },
  artifact: { fill: '#64748b', text: 'Artifact' },
};

// VEX status → Tailwind classes
export const vexStatusClasses: Record<string, string> = {
  not_affected: 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30',
  affected: 'bg-red-500/20 text-red-300 border-red-500/30',
  fixed: 'bg-blue-500/20 text-blue-300 border-blue-500/30',
  under_investigation: 'bg-yellow-500/20 text-yellow-300 border-yellow-500/30',
};

// VEX status → human-readable labels
export const vexStatusLabels: Record<string, string> = {
  not_affected: 'Not Affected',
  affected: 'Affected',
  fixed: 'Fixed',
  under_investigation: 'Under Investigation',
};
