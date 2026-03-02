export interface Platform {
  architecture: string;
  os: string;
  variant?: string;
}

export interface Descriptor {
  mediaType: string;
  digest: string;
  size: number;
  annotations?: Record<string, string>;
  platform?: Platform;
}

export interface IndexManifest {
  mediaType: string;
  digest: string;
  size: number;
  platform?: Platform;
  annotations?: Record<string, string>;
  artifactType?: string;
  config?: ImageConfig;
}

export interface ImageIndex {
  schemaVersion: number;
  mediaType: string;
  manifests: IndexManifest[];
  annotations?: Record<string, string>;
}

export interface Manifest {
  schemaVersion: number;
  mediaType: string;
  config: Descriptor;
  layers: Descriptor[];
  annotations?: Record<string, string>;
}

export interface ContainerConfig {
  User?: string;
  ExposedPorts?: Record<string, object>;
  Env?: string[];
  Entrypoint?: string[];
  Cmd?: string[];
  WorkingDir?: string;
  Labels?: Record<string, string>;
}

export interface RootFS {
  type: string;
  diff_ids: string[];
}

export interface HistoryEntry {
  created: string;
  created_by: string;
  empty_layer?: boolean;
  comment?: string;
}

export interface ImageConfig {
  created: string;
  author?: string;
  architecture: string;
  os: string;
  config?: ContainerConfig;
  rootfs?: RootFS;
  history?: HistoryEntry[];
}

export interface SignatureInfo {
  issuer: string;
  identity: string;
}

export interface Referrer {
  type: string;
  mediaType: string;
  digest: string;
  size: number;
  artifactType: string;
  annotations?: Record<string, string>;
  signatureInfo?: SignatureInfo;
}

export interface ImageInfo {
  repository: string;
  tag: string;
  digest: string;
  created: string;
  architecture: string;
  os: string;
  imageIndex?: ImageIndex;
  manifest?: Manifest;
  config?: ImageConfig;
  tags: string[];
  referrers: Referrer[];
  platformDigest?: string;
}

export interface APIResponse<T = unknown> {
  success: boolean;
  data?: T;
  error?: string;
}

export interface HealthData {
  status: string;
  platform: string;
  version: string;
}

// VEX types
export interface VEXIdentifiers {
  purl?: string;
  cpe22?: string;
  cpe23?: string;
}

export interface VEXProduct {
  '@id'?: string;
  identifiers?: VEXIdentifiers;
  hashes?: Record<string, string>;
}

export interface VEXVulnerability {
  '@id'?: string;
  name: string;
  description?: string;
  aliases?: string[];
}

export interface VEXStatement {
  '@id'?: string;
  version?: number;
  vulnerability: VEXVulnerability;
  timestamp?: string;
  last_updated?: string;
  products?: VEXProduct[];
  status: string;
  supplier?: string;
  status_notes?: string;
  justification?: string;
  impact_statement?: string;
  action_statement?: string;
  action_statement_timestamp?: string;
}

export interface VEXDocument {
  '@context': string;
  '@id': string;
  author: string;
  role?: string;
  timestamp: string;
  last_updated?: string;
  version?: number;
  tooling?: string;
  statements: VEXStatement[];
}

// Matching tags types
export interface MatchingTagsResult {
  repository: string;
  digest: string;
  tags: string[];
  note?: string;
}

// Security score types
export interface SecurityCriterion {
  key: string;
  label: string;
  desc: string;
  present: boolean;
}

export interface MinimalBaseDetails {
  fewLayers: boolean;
  smallSize: boolean;
  nonRoot: boolean;
  noShellEntrypoint: boolean;
}

export interface SecurityScoreResult {
  score: number;
  maxScore: number;
  grade: string;
  color: string;
  colorClass: string;
  criteria: SecurityCriterion[];
  minimalBaseDetails: MinimalBaseDetails;
}
