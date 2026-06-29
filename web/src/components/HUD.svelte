<!-- e:\opsec-control\web\src\components\HUD.svelte -->
<script>
  import { createEventDispatcher } from 'svelte';

  export var state = {};
  export var metrics = {};
  export var demoMode = true;
  export var currentProvider = "cerebras";
  export var switchingProvider = false;

  const dispatch = createEventDispatcher();

  function formatTime(simTime) {
    var baseHour = 9;
    var totalSecs = Number(simTime || 0);
    var hours = baseHour + Math.floor(totalSecs / 3600);
    var mins = Math.floor((totalSecs % 3600) / 60);
    var secs = totalSecs % 60;
    
    var pad = (n) => String(n).padStart(2, '0');
    return `${pad(hours)}:${pad(mins)}:${pad(secs)}`;
  }

  function handleProviderChange(event) {
    dispatch('changeProvider', event.target.value);
  }
</script>

<div class="hud-strip">
  <div class="hud-logo" style="display: flex; align-items: center;">
    <span>CEREBRO EOC</span>
    {#if demoMode}
      <span class="hud-badge demo">Offline / Demo</span>
    {:else}
      <span class="hud-badge live">Live / Connected</span>
    {/if}
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

    <!-- LLM Provider Selector -->
    <div class="hud-metric" style="align-items: flex-start; justify-content: center; min-width: 120px;">
      <span class="hud-metric-label">LLM Provider</span>
      <select 
        id="provider-select" 
        class="provider-select" 
        value={currentProvider} 
        disabled={switchingProvider}
        on:change={handleProviderChange}
      >
        <option value="cerebras">Cerebras</option>
        <option value="openrouter">OpenRouter</option>
      </select>
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

<style>
  .provider-select {
    background: rgba(7, 9, 14, 0.6);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 4px;
    color: #00f2fe;
    font-family: 'Inter', sans-serif;
    font-size: 0.85rem;
    font-weight: 600;
    padding: 2px 24px 2px 8px;
    cursor: pointer;
    outline: none;
    transition: all 0.2s ease;
    appearance: none;
    -webkit-appearance: none;
    -moz-appearance: none;
    background-image: url("data:image/svg+xml;utf8,<svg fill='%2300f2fe' height='24' viewBox='0 0 24 24' width='24' xmlns='http://www.w3.org/2000/svg'><path d='M7 10l5 5 5-5z'/><path d='M0 0h24v24H0z' fill='none'/></svg>");
    background-repeat: no-repeat;
    background-position: right 4px center;
    background-size: 16px;
  }

  .provider-select:hover:not(:disabled), .provider-select:focus:not(:disabled) {
    border-color: rgba(0, 242, 254, 0.5);
    box-shadow: 0 0 8px rgba(0, 242, 254, 0.2);
  }

  .provider-select:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    border-color: rgba(255, 255, 255, 0.08);
  }

  .provider-select option {
    background: #0d111e;
    color: #f1f5f9;
  }
</style>

