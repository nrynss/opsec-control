<!-- e:\opsec-control\web\src\components\PlaybackControl.svelte -->
<script>
  import { createEventDispatcher } from 'svelte';

  export var state = {};
  export var activeEvent = null;
  export var stats = {};
  export var demoMode = true;
  export var isPlaying = false;
  export var speed = 1.0;

  const dispatch = createEventDispatcher();

  var scenarios = ["cerebro-cascade", "test-minimal"];
  var selectedScenario = "cerebro-cascade";

  // Dynamic bounds logic
  var observedMax = 0;

  $: currentTime = stats && stats.currentTime !== undefined ? stats.currentTime : (state && state.time !== undefined ? state.time : 0);

  $: if (currentTime === 0) {
    observedMax = 0;
  } else if (activeEvent && activeEvent.timestamp !== undefined) {
    observedMax = Math.max(observedMax, activeEvent.timestamp);
  }

  $: endBound = demoMode ? 60 : Math.max(170, observedMax || 0);
  $: progressPct = endBound > 0 ? Math.min(100, Math.max(0, (currentTime / endBound) * 100)) : 0;

  $: isPaused = !isPlaying;
  $: isComplete = !demoMode && stats && (stats.status === 'complete' || (currentTime >= endBound && endBound > 0));

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
    var nextState = !isPlaying;
    dispatch('play', nextState);
    if (!demoMode) {
      callEndpoint(nextState ? "/scenario/resume" : "/scenario/pause");
    }
  }

  function step() {
    dispatch('step');
    if (!demoMode) {
      callEndpoint("/scenario/step");
    }
  }

  function reset() {
    dispatch('reset');
    if (!demoMode) {
      callEndpoint("/scenario/reset");
    }
  }

  function load() {
    dispatch('load', selectedScenario);
    if (!demoMode) {
      callEndpoint("/scenario/load", "POST", { name: selectedScenario });
    }
  }

  function setSpeed(s) {
    dispatch('speed', s);
    if (!demoMode) {
      callEndpoint("/scenario/speed", "POST", { speed: s });
    }
  }

  function formatTime(simTime) {
    var baseHour = 9;
    var totalSecs = Number(simTime || 0);
    var hours = baseHour + Math.floor(totalSecs / 3600);
    var mins = Math.floor((totalSecs % 3600) / 60);
    var secs = totalSecs % 60;
    
    var pad = (n) => String(n).padStart(2, '0');
    return `${pad(hours)}:${pad(mins)}:${pad(secs)}`;
  }

  function formatWallTime(ms) {
    if (!ms) return "0.0s";
    return (ms / 1000).toFixed(1) + "s";
  }

  function formatTokens(inTok, outTok) {
    var total = (inTok || 0) + (outTok || 0);
    if (total === 0) return "0";
    if (total >= 1000) {
      return (total / 1000).toFixed(1) + "k";
    }
    return String(total);
  }
</script>

<div class="control-panel">
  <div class="panel-title">Playback Controller</div>
  
  <div class="playback-grid">
    <!-- Play/Pause Button -->
    <button class="playback-btn" class:active={isPlaying} on:click={togglePlay} title={isPlaying ? "Pause" : "Play"}>
      {#if isPlaying}
        <!-- Pause Icon -->
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="4" width="4" height="16"></rect><rect x="14" y="4" width="4" height="16"></rect></svg>
      {:else}
        <!-- Play Icon -->
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"></polygon></svg>
      {/if}
    </button>

    <!-- Step Button -->
    <button class="playback-btn" on:click={step} title="Step Forward">
      <!-- Step Icon -->
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 4 15 12 5 20 5 4"></polygon><line x1="19" y1="5" x2="19" y2="19"></line></svg>
    </button>

    <!-- Load Button -->
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
        <button on:click={() => setSpeed(s)} style="background: none; border: none; cursor: pointer; color: {speed === s ? '#00f2fe' : 'var(--text-muted)'}; font-weight: {speed === s ? '600' : 'normal'}; outline: none; font-size: 0.75rem;">
          {s}x
        </button>
      {/each}
    </div>
  </div>

  <!-- Prominent Green All Clear Button -->
  <button class="all-clear-btn" on:click={reset} title="All Clear / Reset Simulation">
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="display: inline-block;">
      <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path>
      <polyline points="22 4 12 14.01 9 11.01"></polyline>
    </svg>
    All Clear
  </button>

  <!-- Horizontal Simulation Progress timeline bar -->
  <div class="progress-section">
    <div class="progress-labels">
      <span>0s</span>
      <span class="progress-current">+{currentTime}s ({formatTime(currentTime)})</span>
      <span>+{endBound}s</span>
    </div>
    <div class="progress-bar-track" class:paused={isPaused} class:complete={isComplete}>
      <div class="progress-bar-fill" style="width: {progressPct}%"></div>
    </div>
  </div>

  <!-- Simulation Telemetry Metrics Grid -->
  <div class="telemetry-grid">
    <div class="telemetry-title">Simulation Telemetry</div>
    
    <div class="telemetry-card" title="Elapsed simulation seconds">
      <span class="telemetry-card-label">Elapsed Sim Time</span>
      <span class="telemetry-card-value">+{stats && stats.elapsedTime !== undefined ? stats.elapsedTime : currentTime}s</span>
    </div>

    <div class="telemetry-card" title="Elapsed wall-clock execution time">
      <span class="telemetry-card-label">Elapsed Wall Time</span>
      <span class="telemetry-card-value">{formatWallTime(stats && stats.wallElapsed)}</span>
    </div>

    <div class="telemetry-card" title="Total event logs processed">
      <span class="telemetry-card-label">Events Replayed</span>
      <span class="telemetry-card-value">{stats && stats.eventsReplayed !== undefined ? stats.eventsReplayed : 0}</span>
    </div>

    <div class="telemetry-card" title="LLM tokens: {stats && stats.tokensIn || 0} in | {stats && stats.tokensOut || 0} out">
      <span class="telemetry-card-label">LLM Tokens</span>
      <span class="telemetry-card-value">{formatTokens(stats && stats.tokensIn, stats && stats.tokensOut)}</span>
    </div>

    <div class="telemetry-card" title="Total inferences executed by cells">
      <span class="telemetry-card-label">Inferences Run</span>
      <span class="telemetry-card-value">{stats && stats.inferences !== undefined ? stats.inferences : 0}</span>
    </div>

    <div class="telemetry-card" title="Replay speed multiplier">
      <span class="telemetry-card-label">Replay Speed</span>
      <span class="telemetry-card-value">{stats && stats.speed !== undefined ? stats.speed : speed}x</span>
    </div>
  </div>
</div>
