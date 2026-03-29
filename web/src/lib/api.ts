import type { APIResponse, ImageInfo, HealthData, VEXDocument, MatchingTagsResult, ScanResult, ScanResponse } from './types';

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

export async function scanImage(imageRef: string, force = false): Promise<ScanResponse> {
  const url = `/api/scan?image=${encodeURIComponent(imageRef)}${force ? '&force=1' : ''}`;
  const response = await fetch(url);
  const json: APIResponse<ScanResult> = await response.json();
  if (!json.success) {
    throw new Error(json.error || 'Scan failed');
  }
  return {
    result: json.data as ScanResult,
    cachedAt: response.headers.get('X-Cached-At') || undefined,
  };
}

/**
 * Stream a Trivy scan via SSE. Calls onProgress with actual Trivy log
 * messages. Falls back to regular JSON on cache hit.
 */
export async function scanImageStream(
  imageRef: string,
  force: boolean,
  onProgress: (msg: string) => void,
): Promise<ScanResponse> {
  const url = `/api/scan?image=${encodeURIComponent(imageRef)}&stream=1${force ? '&force=1' : ''}`;
  const response = await fetch(url);
  const contentType = response.headers.get('Content-Type') || '';

  // Cache hit — server returns JSON directly
  if (contentType.includes('application/json')) {
    const json: APIResponse<ScanResult> = await response.json();
    if (!json.success) throw new Error(json.error || 'Scan failed');
    return {
      result: json.data as ScanResult,
      cachedAt: response.headers.get('X-Cached-At') || undefined,
    };
  }

  // SSE stream
  const reader = response.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let finalResult: ScanResponse | null = null;
  let streamError: string | null = null;

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    const parts = buffer.split('\n\n');
    buffer = parts.pop()!;

    for (const part of parts) {
      let event = '';
      let data = '';
      for (const line of part.split('\n')) {
        if (line.startsWith('event: ')) event = line.slice(7);
        else if (line.startsWith('data: ')) data = line.slice(6);
      }
      if (!event || !data) continue;

      if (event === 'progress') {
        const parsed = JSON.parse(data);
        onProgress(parsed.message);
      } else if (event === 'result') {
        const parsed: APIResponse<ScanResult> = JSON.parse(data);
        if (!parsed.success) throw new Error(parsed.error || 'Scan failed');
        finalResult = { result: parsed.data as ScanResult };
      } else if (event === 'error') {
        const parsed = JSON.parse(data);
        streamError = parsed.error;
      }
    }
  }

  if (streamError) throw new Error(streamError);
  if (!finalResult) throw new Error('Scan stream ended without result');
  return finalResult;
}

/** Check if cached scan results exist without triggering a Trivy scan. Returns null on miss or timeout. */
export async function peekScan(imageRef: string): Promise<ScanResponse | null> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 3000);
  try {
    const url = `/api/scan?image=${encodeURIComponent(imageRef)}&peek=1`;
    const response = await fetch(url, { signal: controller.signal });
    if (!response.ok) return null;
    const json: APIResponse<ScanResult> = await response.json();
    if (!json.success) return null;
    return {
      result: json.data as ScanResult,
      cachedAt: response.headers.get('X-Cached-At') || undefined,
    };
  } catch {
    return null;
  } finally {
    clearTimeout(timeout);
  }
}
