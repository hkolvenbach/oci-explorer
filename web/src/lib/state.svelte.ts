import type { ImageInfo } from './types';

export type ViewMode = 'details' | 'graph';

export const appState = $state({
  currentView: 'details' as ViewMode,
  currentData: null as ImageInfo | null,
  selectedPlatform: 'all',
  platformDigestMap: {} as Record<string, string>,
  collapseStates: {} as Record<string, boolean>,
  viewToggles: {
    config: false,
    annotations: false,
    imageIndex: false,
  },
  isLoading: false,
  error: '',
  searchQuery: 'alpine:latest',
  version: '',
});

export const graphState = $state({
  zoom: 1,
  panX: 0,
  panY: 0,
  isPanning: false,
  panStartX: 0,
  panStartY: 0,
});

export function resetGraphState() {
  graphState.zoom = 1;
  graphState.panX = 0;
  graphState.panY = 0;
  graphState.isPanning = false;
}

export function buildPlatformDigestMap(data: ImageInfo) {
  const map: Record<string, string> = {};
  if (data.imageIndex?.manifests) {
    for (const m of data.imageIndex.manifests) {
      if (m.platform) {
        const platformStr = `${m.platform.os}/${m.platform.architecture}${m.platform.variant ? '/' + m.platform.variant : ''}`;
        if (platformStr !== 'unknown/unknown') {
          map[platformStr] = m.digest;
        }
      }
    }
  }
  appState.platformDigestMap = map;
}

export function getFilteredReferrers(data: ImageInfo) {
  const referrers = data.referrers || [];
  if (appState.selectedPlatform === 'all') return referrers;
  return referrers.filter((r) => {
    const refDigest = r.annotations?.['vnd.docker.reference.digest'];
    if (!refDigest) return true;
    return refDigest === appState.selectedPlatform;
  });
}

export function getSelectedPlatformConfig(data: ImageInfo) {
  if (appState.selectedPlatform === 'all' || !data.imageIndex?.manifests) {
    return data.config;
  }
  const platformManifest = data.imageIndex.manifests.find(
    (m) => m.digest === appState.selectedPlatform,
  );
  return platformManifest?.config || data.config;
}

export function getSelectedPlatformName(): string | null {
  if (appState.selectedPlatform === 'all') return null;
  for (const [platform, digest] of Object.entries(appState.platformDigestMap)) {
    if (digest === appState.selectedPlatform) return platform;
  }
  return null;
}
