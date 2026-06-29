<!-- e:\opsec-control\web\src\components\PlaybackControl.svelte -->
<script>
  import { createEventDispatcher } from 'svelte';

  export var state = {};
  export var activeEvent = null;
  export var stats = {};
  export var isDisconnected = true;
  export var isInitialized = false;
  export var isPlaying = false;
  export var speed = 1.0;
  export var latencyMs = 0;                 // current provider's latest decision latency
  export var currentProvider = "cerebras";
  export var latencyByProvider = {};        // { cerebras, openrouter } remembered latencies
  export var totalDecisionMs = 0;           // cumulative reasoning time for the current run
  export var totalByProvider = {};          // { cerebras, openrouter } remembered run totals

  const dispatch = createEventDispatcher();

  // Dynamic bounds logic
  var observedMax = 0;

  $: currentTime = stats && stats.currentTime !== undefined ? stats.currentTime : (state && state.time !== undefined ? state.time : 0);

  $: if (currentTime === 0) {
    observedMax = 0;
  } else if (activeEvent && activeEvent.timestamp !== undefined) {
    observedMax = Math.max(observedMax, activeEvent.timestamp);
  }

  $: endBound = Math.max(170, observedMax || 0);
  $: progressPct = endBound > 0 ? Math.min(100, Math.max(0, (currentTime / endBound) * 100)) : 0;

  $: isPaused = !isPlaying;
  $: isComplete = stats && (stats.status === 'complete' || (currentTime >= endBound && endBound > 0));

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
    callEndpoint(nextState ? "/scenario/resume" : "/scenario/pause");
  }

  // Step the simulation time by 1 step
  function step() {
    dispatch('step');
    callEndpoint("/scenario/step");
  }

  // Reset the simulation to start
  // This will trigger backend reset and optimistic clear
  function reset() {
    dispatch('reset');
    callEndpoint("/scenario/reset");
  }

  function setSpeed(s) {
    dispatch('speed', s);
    callEndpoint("/scenario/speed", "POST", { speed: s });
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

  // Format a decision latency (anomaly -> COP fan-out time) for display.
  function formatLatency(ms) {
    var n = Number(ms || 0);
    if (n <= 0) return "--";
    if (n < 1000) return Math.round(n) + " ms";
    if (n < 60000) return (n / 1000).toFixed(1) + "s";
    var mins = Math.floor(n / 60000);
    var secs = Math.round((n % 60000) / 1000);
    return mins + "m " + secs + "s";
  }

  // Comparison tooltips: both providers on the same model, plus the speed ratio.
  $: latencyTooltip = compareTooltip(
    "Anomaly -> COP, same Gemma 4 31B",
    (latencyByProvider && latencyByProvider.cerebras) || 0,
    (latencyByProvider && latencyByProvider.openrouter) || 0
  );
  $: totalTooltip = compareTooltip(
    "Total AI reasoning time this run",
    (totalByProvider && totalByProvider.cerebras) || 0,
    (totalByProvider && totalByProvider.openrouter) || 0
  );

  function compareTooltip(prefix, cb, or) {
    var parts = [
      prefix,
      "Cerebras: " + (cb ? formatLatency(cb) : "--"),
      "OpenRouter: " + (or ? formatLatency(or) : "--")
    ];
    if (cb > 0 && or > 0) {
      var ratio = or >= cb ? or / cb : cb / or;
      var faster = or >= cb ? "Cerebras" : "OpenRouter";
      parts.push(faster + " " + ratio.toFixed(ratio < 10 ? 1 : 0) + "x faster");
    }
    return parts.join("  ·  ");
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
    <button class="playback-btn" class:active={isPlaying} on:click={togglePlay} title={isPlaying ? "Pause" : "Play"} disabled={isDisconnected || !isInitialized || isComplete}>
      {#if isPlaying}
        <!-- Pause Icon -->
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="4" width="4" height="16"></rect><rect x="14" y="4" width="4" height="16"></rect></svg>
      {:else}
        <!-- Play Icon -->
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"></polygon></svg>
      {/if}
    </button>

    <!-- Step Button -->
    <button class="playback-btn" on:click={step} title="Step Forward" disabled={isDisconnected || !isInitialized || isComplete}>
      <!-- Step Icon -->
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 4 15 12 5 20 5 4"></polygon><line x1="19" y1="5" x2="19" y2="19"></line></svg>
    </button>
  </div>

  <div style="display: flex; justify-content: space-between; font-size: 0.75rem; color: var(--text-muted); margin-top: 8px;">
    <span>Simulation Speed:</span>
    <div style="display: flex; gap: 6px;">
      {#each [1, 2, 5, 10] as s}
        <button on:click={() => setSpeed(s)} style="background: none; border: none; cursor: pointer; color: {speed === s ? '#00f2fe' : 'var(--text-muted)'}; font-weight: {speed === s ? '600' : 'normal'}; outline: none; font-size: 0.75rem;" disabled={isDisconnected || !isInitialized}>
          {s}x
        </button>
      {/each}
    </div>
  </div>

  <!-- Prominent Green All Clear Button -->
  <button class="all-clear-btn" on:click={reset} title="All Clear / Reset Simulation" disabled={isDisconnected || !isInitialized}>
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
    {#if isComplete}
      <div style="font-size: 0.7rem; color: var(--color-nominal); margin-top: 6px; text-align: center; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em;">
        Simulation complete. Click "All Clear" to reset.
      </div>
    {/if}
  </div>

  <!-- Simulation Telemetry Metrics Grid -->
  <div class="telemetry-grid">
    <div class="telemetry-title">Simulation Telemetry</div>
    
    <div class="telemetry-card" title={totalTooltip}>
      <span class="telemetry-card-label">Total Decision Time</span>
      <span class="telemetry-card-value">{formatLatency(totalDecisionMs)}</span>
    </div>

    <div class="telemetry-card" title={latencyTooltip}>
      <span class="telemetry-card-label">Decision Latency</span>
      <span class="telemetry-card-value">{formatLatency(latencyMs)}</span>
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
