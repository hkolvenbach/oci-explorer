import type { APIResponse, ImageInfo, HealthData, VEXDocument, MatchingTagsResult, ScanResult } from './types';

async function fetchJSON<T>(url: string): Promise<T> {
  const response = await fetch(url);
  const result: APIResponse<T> = await response.json();
  if (!result.success) {
    throw new Error(result.error || 'Request failed');
  }
  return result.data as T;
}

export async function inspectImage(imageRef: string): Promise<ImageInfo> {
  return fetchJSON<ImageInfo>(`/api/inspect?image=${encodeURIComponent(imageRef)}`);
}

export async function listTags(repository: string): Promise<string[]> {
  return fetchJSON<string[]>(`/api/tags?repository=${encodeURIComponent(repository)}`);
}

export async function downloadSBOM(repository: string, digest: string): Promise<Blob> {
  const response = await fetch(
    `/api/sbom?repository=${encodeURIComponent(repository)}&digest=${encodeURIComponent(digest)}`,
  );
  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error || 'Failed to download SBOM');
  }
  return response.blob();
}

export async function fetchVEX(repository: string, digest: string): Promise<VEXDocument> {
  return fetchJSON<VEXDocument>(
    `/api/vex?repository=${encodeURIComponent(repository)}&digest=${encodeURIComponent(digest)}`,
  );
}

export async function fetchMatchingTags(imageRef: string): Promise<MatchingTagsResult> {
  return fetchJSON<MatchingTagsResult>(`/api/matching-tags?image=${encodeURIComponent(imageRef)}`);
}

export async function fetchHealth(): Promise<HealthData> {
  return fetchJSON<HealthData>('/api/health');
}

export async function scanImage(imageRef: string): Promise<ScanResult> {
  return fetchJSON<ScanResult>(`/api/scan?image=${encodeURIComponent(imageRef)}`);
}
