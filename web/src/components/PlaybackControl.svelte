<!-- e:\opsec-control\web\src\components\PlaybackControl.svelte -->
<script>
  export var state = {};
  export var activeEvent = null;

  var isPlaying = false;
  var speed = 1.0;
  var scenarios = ["cerebro-cascade", "test-minimal"];
  var selectedScenario = "cerebro-cascade";

  async function callEndpoint(path, method = "POST", body = null) {
    try {
      var options = { method: method };
      if (body) {
        options.headers = { "Content-Type": "application/json" };
        options.body = JSON.stringify(body);
      }
      var res = await fetch(path, options);
      return res.ok;
    } catch (e) {
      console.error("API call failed:", path, e);
      return false;
    }
  }

  function togglePlay() {
    isPlaying = !isPlaying;
    callEndpoint(isPlaying ? "/scenario/resume" : "/scenario/pause");
  }

  function step() {
    isPlaying = false;
    callEndpoint("/scenario/step");
  }

  function reset() {
    isPlaying = false;
    callEndpoint("/scenario/reset");
  }

  function load() {
    isPlaying = false;
    callEndpoint("/scenario/load", "POST", { name: selectedScenario });
  }

  function setSpeed(s) {
    speed = s;
    callEndpoint("/scenario/speed", "POST", { speed: s });
  }
</script>

<div class="control-panel">
  <div class="panel-title">Playback Controller</div>
  
  <div class="playback-grid">
    <button class="playback-btn" class:active={isPlaying} on:click={togglePlay} title={isPlaying ? "Pause" : "Play"}>
      {#if isPlaying}
        <!-- Pause Icon -->
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="4" width="4" height="16"></rect><rect x="14" y="4" width="4" height="16"></rect></svg>
      {:else}
        <!-- Play Icon -->
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"></polygon></svg>
      {/if}
    </button>

    <button class="playback-btn" on:click={step} title="Step Forward">
      <!-- Step Icon -->
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 4 15 12 5 20 5 4"></polygon><line x1="19" y1="5" x2="19" y2="19"></line></svg>
    </button>

    <button class="playback-btn" on:click={reset} title="Reset Simulation">
      <!-- Reset Icon -->
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M2.5 2v6h6M21.5 22v-6h-6"></path><path d="M22 11.5A10 10 0 0 0 9.5 3.5M2 12.5a10 10 0 0 0 12.5 8.5"></path></svg>
    </button>

    <button class="playback-btn" on:click={load} title="Load Scenario">
      <!-- Load Icon -->
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M17 8l-5-5-5 5M12 3v12"></path></svg>
    </button>
  </div>

  <div style="display: flex; gap: 8px; align-items: center; margin-top: 4px;">
    <select bind:value={selectedScenario} style="flex: 1; background: rgba(0,0,0,0.2); border: 1px solid var(--panel-border); color: var(--text-primary); border-radius: 4px; padding: 4px; font-size: 0.75rem; outline: none;">
      {#each scenarios as s}
        <option value={s}>{s}</option>
      {/each}
    </select>
  </div>

  <div style="display: flex; justify-content: space-between; font-size: 0.75rem; color: var(--text-muted); margin-top: 4px;">
    <span>Simulation Speed:</span>
    <div style="display: flex; gap: 6px;">
      {#each [1, 2, 5, 10] as s}
        <button on:click={() => setSpeed(s)} style="background: none; border: none; cursor: pointer; color: {speed === s ? '#00f2fe' : 'var(--text-muted)'}; font-weight: {speed === s ? '600' : 'normal'}; outline: none;" font-size="0.75rem">
          {s}x
        </button>
      {/each}
    </div>
  </div>
</div>
