<!-- e:\opsec-control\web\src\components\HUD.svelte -->
<script>
  export var state = {};
  export var metrics = {};

  function formatTime(simTime) {
    var baseHour = 9;
    var totalSecs = Number(simTime || 0);
    var hours = baseHour + Math.floor(totalSecs / 3600);
    var mins = Math.floor((totalSecs % 3600) / 60);
    var secs = totalSecs % 60;
    
    var pad = (n) => String(n).padStart(2, '0');
    return `${pad(hours)}:${pad(mins)}:${pad(secs)}`;
  }
</script>

<div class="hud-strip">
  <div class="hud-logo">
    <span>CEREBRO EOC</span>
  </div>

  <div class="hud-metrics">
    <!-- Active Cells -->
    <div class="hud-metric">
      <span class="hud-metric-label">Active Cells</span>
      <span class="hud-metric-value" class:status-nominal={metrics.activeCells === 0} class:status-critical={metrics.activeCells > 0}>
        {metrics.activeCells || 0} / 6
      </span>
    </div>

    <!-- Token Throughput -->
    <div class="hud-metric">
      <span class="hud-metric-label">Throughput</span>
      <span class="hud-metric-value">
        {#if metrics.tokensPerSec}
          {metrics.tokensPerSec.toLocaleString(undefined, { maximumFractionDigits: 0 })} tok/s
        {:else}
          0 tok/s
        {/if}
      </span>
    </div>

    <!-- Fan-out Latency -->
    <div class="hud-metric">
      <span class="hud-metric-label">Fan-out Latency</span>
      <span class="hud-metric-value">
        {#if metrics.latencyMs}
          {metrics.latencyMs} ms
        {:else}
          -- ms
        {/if}
      </span>
    </div>

    <!-- Tokens this tick -->
    <div class="hud-metric">
      <span class="hud-metric-label">Tick Tokens</span>
      <span class="hud-metric-value">
        {metrics.tickTokens || 0}
      </span>
    </div>

    <!-- State Version -->
    <div class="hud-metric">
      <span class="hud-metric-label">State Version</span>
      <span class="hud-metric-value" style="color: #00ff87;">
        v{state.version || 0}
      </span>
    </div>

    <!-- Simulation Time -->
    <div class="hud-metric" style="border-left: 1px solid rgba(255, 255, 255, 0.1); padding-left: 20px;">
      <span class="hud-metric-label">Simulation Time</span>
      <span class="hud-metric-value" style="color: #00f2fe; font-size: 1.1rem; font-weight: 700;">
        {formatTime(state.time)}
      </span>
    </div>
  </div>
</div>
