<!-- e:\opsec-control\web\src\components\MatrixFeed.svelte -->
<script>
  import { afterUpdate } from 'svelte';

  export var logs = [];
  export var currentProvider = "cerebras";

  var feedElement;

  afterUpdate(() => {
    if (feedElement) {
      var threshold = 30;
      var isNearBottom = feedElement.scrollTop + feedElement.clientHeight >= feedElement.scrollHeight - threshold;
      if (isNearBottom || feedElement.scrollHeight <= feedElement.clientHeight + 10) {
        feedElement.scrollTop = feedElement.scrollHeight;
      }
    }
  });

  function capitalize(str) {
    if (!str) return "";
    return str.charAt(0).toUpperCase() + str.slice(1);
  }
</script>

<div class="matrix-panel">
  <div class="matrix-header">
    <div class="matrix-title">Telemetry Matrix Feed</div>
    <span style="font-family: var(--font-mono); font-size: 0.65rem; color: var(--text-muted);">JSON STREAM</span>
  </div>

  <div class="matrix-feed" bind:this={feedElement}>
    {#if logs.length === 0}
      <div style="color: var(--text-muted); font-style: italic; font-size: 0.75rem; text-align: center; margin-top: 20px;">
        Listening for {capitalize(currentProvider)} telemetry stream...
      </div>
    {:else}
      {#each logs as log, index}
        <div class="matrix-line">
          <span style="color: var(--text-muted);">[{new Date().toLocaleTimeString()}]</span>
          <span style="color: #00ff87;">{log.prefix || 'SYSTEM'}:</span>
          <span style="color: var(--text-primary);">{log.content}</span>
        </div>
      {/each}
    {/if}
  </div>
</div>

