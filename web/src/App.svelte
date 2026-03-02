<script lang="ts">
  import { appState, buildPlatformDigestMap, getFilteredReferrers, getSelectedPlatformConfig, getSelectedPlatformName } from './lib/state.svelte';
  import type { MatchingTagsResult } from './lib/types';
  import * as api from './lib/api';
  import Header from './components/Header.svelte';
  import Footer from './components/Footer.svelte';
  import WelcomeView from './components/WelcomeView.svelte';
  import ErrorView from './components/ErrorView.svelte';
  import LoadingOverlay from './components/LoadingOverlay.svelte';
  import ImageSummary from './components/ImageSummary.svelte';
  import PlatformFilter from './components/PlatformFilter.svelte';
  import ImageIndexSection from './components/ImageIndexSection.svelte';
  import LayersSection from './components/LayersSection.svelte';
  import AnnotationsSection from './components/AnnotationsSection.svelte';
  import ConfigSection from './components/ConfigSection.svelte';
  import ReferrersSection from './components/ReferrersSection.svelte';
  import TagsSection from './components/TagsSection.svelte';
  import GraphView from './components/GraphView.svelte';

  let matchingTags = $state<MatchingTagsResult | null>(null);

  // Fetch version from backend
  $effect(() => {
    api.fetchHealth().then((h) => {
      appState.version = h.version;
    }).catch(() => {});
  });

  function loadFromURL() {
    const q = new URLSearchParams(window.location.search).get('q');
    if (q) {
      appState.searchQuery = q;
      doInspect(true);
    } else {
      appState.searchQuery = 'alpine:latest';
      appState.currentData = null;
      appState.error = '';
    }
  }

  // Read ?q= from URL on mount
  $effect(() => { loadFromURL(); });

  async function doInspect(skipURLUpdate = false) {
    const imageRef = appState.searchQuery.trim();
    if (!imageRef) return;

    appState.selectedPlatform = 'all';
    appState.platformDigestMap = {};
    appState.collapseStates = {};
    appState.isLoading = true;
    appState.error = '';
    matchingTags = null;

    try {
      const data = await api.inspectImage(imageRef);
      appState.currentData = data;
      buildPlatformDigestMap(data);
      if (!skipURLUpdate) {
        const url = new URL(window.location.href);
        url.searchParams.set('q', imageRef);
        history.pushState({ image: imageRef }, '', url.toString());
      }
      // Fetch matching tags in background (non-blocking)
      api.fetchMatchingTags(imageRef).then((result) => {
        matchingTags = result;
      }).catch(() => {});
    } catch (err) {
      appState.error = (err as Error).message;
      appState.currentData = null;
    } finally {
      appState.isLoading = false;
    }
  }

  function quickInspect(image: string) {
    appState.searchQuery = image;
    doInspect();
  }

  let hasMultiplePlatforms = $derived(Object.keys(appState.platformDigestMap).length > 1);
  let filteredReferrers = $derived(appState.currentData ? getFilteredReferrers(appState.currentData) : []);
  let selectedConfig = $derived(appState.currentData ? getSelectedPlatformConfig(appState.currentData) : null);
  let selectedPlatformName = $derived(getSelectedPlatformName());
</script>

<svelte:window onpopstate={loadFromURL} />

<div id="app">
  <Header oninspect={() => doInspect()} />

  <main class="max-w-7xl mx-auto px-4 py-6">
    {#if appState.error}
      <ErrorView message={appState.error} />
    {/if}

    {#if !appState.currentData && !appState.isLoading && !appState.error}
      <WelcomeView oninspect={quickInspect} />
    {/if}

    <div class="relative">
      {#if appState.currentData}
        {#if appState.currentView === 'graph'}
          <GraphView data={appState.currentData} />
        {:else}
          <ImageSummary data={appState.currentData} />

          {#if hasMultiplePlatforms}
            <PlatformFilter data={appState.currentData} />
          {/if}

          <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <!-- Left Column -->
            <div>
              {#if appState.currentData.imageIndex}
                <ImageIndexSection imageIndex={appState.currentData.imageIndex} />
              {/if}
              <ReferrersSection referrers={filteredReferrers} totalCount={appState.currentData.referrers?.length || 0} />
              {#if appState.currentData.manifest?.annotations}
                <AnnotationsSection annotations={appState.currentData.manifest.annotations} />
              {/if}
            </div>

            <!-- Right Column -->
            <div>
              {#if selectedConfig}
                <ConfigSection config={selectedConfig} platformName={selectedPlatformName} />
              {/if}
              {#if appState.currentData.manifest}
                <LayersSection layers={appState.currentData.manifest.layers} />
              {/if}
              <TagsSection tags={appState.currentData.tags || []} {matchingTags} oninspect={quickInspect} />
            </div>
          </div>
        {/if}
      {/if}

      <LoadingOverlay visible={appState.isLoading} />
    </div>
  </main>

  <Footer />
</div>
