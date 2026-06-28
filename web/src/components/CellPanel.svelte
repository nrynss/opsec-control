<!-- e:\opsec-control\web\src\components\CellPanel.svelte -->
<script>
  export var kind = "";
  export var data = null; // CellOutput DTO
  export var status = "idle"; // "idle" | "analyzing" | "done"
  export var isCommander = false;

  // Risky colors lookup
  var riskClasses = {
    "Low": "risk-low",
    "Medium": "risk-medium",
    "High": "risk-high",
    "Critical": "risk-critical"
  };
</script>

<div class="agent-card" class:analyzing={status === 'analyzing'}>
  <div class="agent-header">
    <div class="agent-name">
      <span class="agent-dot" class:analyzing={status === 'analyzing'} class:done={status === 'done'}></span>
      <span>{kind} Cell</span>
    </div>
    {#if status === 'done' && data}
      <span class="agent-risk {riskClasses[data.riskLevel] || 'risk-low'}">
        {data.riskLevel || 'Low'}
      </span>
    {/if}
  </div>

  {#if status === 'analyzing'}
    <!-- Pulsing loading state -->
    <div style="display: flex; flex-direction: column; gap: 8px; margin-top: 6px;">
      <div style="height: 12px; background: rgba(0, 242, 254, 0.1); border-radius: 4px; width: 85%; animation: pulse-skeleton 1s infinite alternate;"></div>
      <div style="height: 12px; background: rgba(0, 242, 254, 0.1); border-radius: 4px; width: 60%; animation: pulse-skeleton 1s infinite alternate;"></div>
      <div style="height: 12px; background: rgba(0, 242, 254, 0.1); border-radius: 4px; width: 75%; animation: pulse-skeleton 1s infinite alternate; margin-top: 8px;"></div>
    </div>
  {:else if status === 'done' && data}
    <!-- Completed Analysis state -->
    <div class="agent-summary">
      {data.summary}
    </div>

    {#if data.recommendations && data.recommendations.length > 0}
      <div class="agent-recommendations">
        {#each data.recommendations.slice(0, 2) as rec}
          <div class="agent-rec-item">
            {rec}
          </div>
        {/each}
      </div>
    {/if}

    <div style="display: flex; justify-content: space-between; font-size: 0.65rem; color: var(--text-muted); margin-top: auto; padding-top: 4px; border-top: 1px solid rgba(255, 255, 255, 0.02);">
      <span>Version: v{data.stateVersion || 0}</span>
      {#if data.confidence}
        <span>Conf: {(data.confidence * 100).toFixed(0)}%</span>
      {/if}
    </div>
  {:else}
    <!-- Idle state -->
    <div class="agent-summary" style="color: var(--text-muted); font-style: italic; display: flex; align-items: center; justify-content: center; height: 100%;">
      Nominal monitoring mode
    </div>
  {/if}
</div>

<style>
  @keyframes pulse-skeleton {
    from { opacity: 0.3; }
    to { opacity: 0.7; }
  }
</style>
