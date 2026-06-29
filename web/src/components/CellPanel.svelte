<!-- e:\opsec-control\web\src\components\CellPanel.svelte -->
<script>
  export var kind = "";
  export var history = []; // List of CellOutput DTOs
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
      <span class="agent-dot" class:analyzing={status === 'analyzing'} class:done={history.length > 0}></span>
      <span>{kind} Cell</span>
    </div>
    {#if history.length > 0}
      <span class="agent-risk {riskClasses[history[0].riskLevel] || 'risk-low'}">
        {history[0].riskLevel || 'Low'}
      </span>
    {/if}
  </div>

  <div class="agent-history-list">
    {#if status === 'analyzing'}
      <!-- Pulsing loading state -->
      <div style="display: flex; flex-direction: column; gap: 8px; margin-top: 6px; padding-bottom: 8px; border-bottom: 1px dashed rgba(0, 242, 254, 0.2);">
        <div style="font-size: 0.7rem; color: #00f2fe; font-family: var(--font-mono); font-weight: 600;">ANALYZING NEW TELEMETRY...</div>
        <div style="height: 12px; background: rgba(0, 242, 254, 0.1); border-radius: 4px; width: 85%; animation: pulse-skeleton 1s infinite alternate;"></div>
        <div style="height: 12px; background: rgba(0, 242, 254, 0.1); border-radius: 4px; width: 60%; animation: pulse-skeleton 1s infinite alternate;"></div>
      </div>
    {/if}

    {#each history as data}
      <div class="history-entry">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px;">
          <span style="font-size: 0.65rem; color: var(--text-muted); font-family: var(--font-mono);">v{data.stateVersion || 0}</span>
          {#if data.riskLevel}
            <span class="agent-risk {riskClasses[data.riskLevel] || 'risk-low'}" style="font-size: 0.6rem; padding: 1px 4px;">
              {data.riskLevel}
            </span>
          {/if}
        </div>
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
      </div>
    {/each}

    {#if history.length === 0 && status !== 'analyzing'}
      <!-- Idle state -->
      <div class="agent-summary" style="color: var(--text-muted); font-style: italic; display: flex; align-items: center; justify-content: center; height: 100%;">
        Nominal monitoring mode
      </div>
    {/if}
  </div>
</div>

<style>
  @keyframes pulse-skeleton {
    from { opacity: 0.3; }
    to { opacity: 0.7; }
  }
</style>

