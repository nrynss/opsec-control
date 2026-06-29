<!-- e:\opsec-control\web\src\components\Dashboard.svelte -->
<script>
  import { onMount, afterUpdate } from 'svelte';
  import HUD from './HUD.svelte';
  import Map from './Map.svelte';
  import CellPanel from './CellPanel.svelte';
  import MatrixFeed from './MatrixFeed.svelte';
  import PlaybackControl from './PlaybackControl.svelte';
  import PerceptionUpload from './PerceptionUpload.svelte';

  // State snapshot
  var state = {
    version: 0,
    time: 0,
    sectors: {},
    bridges: {},
    dam: { status: "normal", reservoirPct: 0.5, stressRating: 0.1 },
    levee: { status: "intact", height: 4.5, integrity: 1.0 },
    hospitals: {},
    shelters: {},
    fireZones: {},
    flood: { polygons: [] },
    resources: {}
  };

  // Metrics HUD
  var metrics = {
    activeCells: 0,
    tokensPerSec: 0,
    latencyMs: 0,
    tickTokens: 0
  };

  // Last decision (fan-out) latency remembered per provider, so the telemetry
  // card can show a Cerebras-vs-OpenRouter comparison that survives the toggle.
  // Persisted across resets on purpose — it's a benchmark receipt, not live state.
  var lastLatencyByProvider = { cerebras: 0, openrouter: 0 };

  // Cumulative AI reasoning time for the CURRENT run (sum of every fan-out's
  // latency). Reset on All Clear. Mirrored per provider for the comparison tooltip.
  var totalDecisionMs = 0;
  var totalDecisionByProvider = { cerebras: 0, openrouter: 0 };

  // Commander COP
  var cop = {
    summary: "EOC system nominal. Standing by for telemetry.",
    overallRisk: "Low",
    prioritizedActions: [],
    cellOutputs: []
  };
  var copHistory = [];

  // Specialist Cell statuses and data
  var cellStatuses = {
    "Intelligence": "idle",
    "Infrastructure": "idle",
    "Medical": "idle",
    "Population": "idle",
    "Communications": "idle"
  };

  var cellHistory = {
    "Intelligence": [],
    "Infrastructure": [],
    "Medical": [],
    "Population": [],
    "Communications": []
  };

  var timelineEvents = [];
  var matrixLogs = [];
  var timelineElement;

  var currentProvider = "cerebras";
  var switchingProvider = false;
  var isDisconnected = true;
  var isInitialized = false;
  var overlayFading = false;
  var ws = null;

  // P25 Playback and Telemetry State
  var isPlaying = false;
  var speed = 1.0;
  var stats = {
    status: "paused",
    currentTime: 0,
    elapsedTime: 0,
    wallElapsed: 0,
    eventsReplayed: 0,
    tokensIn: 0,
    tokensOut: 0,
    inferences: 0,
    speed: 1.0
  };
  var lastResetTime = 0;
  var pendingReset = false;

  // Initialize EOC default nominal state
  function loadNominalState() {
    state = {
      version: 0,
      time: 0,
      sectors: {
        "westbank": { id: "westbank", name: "Westside", power: "on", comms: "up", water: "up", gas: "up", population: 45000 },
        "greenfield": { id: "greenfield", name: "Greenfield", power: "on", comms: "up", water: "up", gas: "up", population: 30000 },
        "harborside": { id: "harborside", name: "Harborside", power: "on", comms: "up", water: "up", gas: "up", population: 15000 },
        "central": { id: "central", name: "Central", power: "on", comms: "up", water: "up", gas: "up", population: 80000 },
        "highgate": { id: "highgate", name: "Highgate", power: "on", comms: "up", water: "up", gas: "up", population: 50000 },
        "southport": { id: "southport", name: "Southport", power: "on", comms: "up", water: "up", gas: "up", population: 35000 },
        "ironworks": { id: "ironworks", name: "Ironworks", power: "on", comms: "up", water: "up", gas: "up", population: 10000 }
      },
      bridges: {
        "vora": { id: "vora", name: "Vora Bridge", status: "open" },
        "iron": { id: "iron", name: "Iron Bridge", status: "open" },
        "south-span": { id: "south-span", name: "South Span", status: "open" }
      },
      dam: { id: "mainor", status: "normal", reservoirPct: 0.45, stressRating: 0.1 },
      levee: { id: "southport", status: "intact", height: 4.5, integrity: 1.0 },
      hospitals: {
        "central-general": { id: "central-general", name: "Central General", sector: "central", beds: 400, icu: 40, er: 60, occupancy: 120, band: "normal", onGenerator: false }
      },
      shelters: {
        "greenfield-arena": { id: "greenfield-arena", name: "Greenfield Arena", sector: "greenfield", capacity: 2000, occupancy: 150, full: false }
      },
      fireZones: {},
      flood: { polygons: [] },
      resources: {
        "amb-1": { id: "amb-1", kind: "ambulance", homeBase: "central", count: 10, deployed: 0 }
      }
    };

    cop = {
      summary: "EOC system nominal. All channels clear. Monitoring seismograph.",
      overallRisk: "Low",
      prioritizedActions: [],
      cellOutputs: []
    };
    copHistory = [];

    for (var k in cellStatuses) {
      cellStatuses[k] = "idle";
      cellHistory[k] = [];
    }
    metrics = { activeCells: 0, tokensPerSec: 0, latencyMs: 0, tickTokens: 0 };
    totalDecisionMs = 0; // new run — clear the cumulative reasoning total
    timelineEvents = [];
    matrixLogs = [{ prefix: "SYSTEM", content: "Cerebro command center ready. Listening on /stream." }];

    // P25: Reset stats to nominal faked or zero
    stats = {
      status: "paused",
      currentTime: 0,
      elapsedTime: 0,
      wallElapsed: 0,
      eventsReplayed: 0,
      tokensIn: 0,
      tokensOut: 0,
      inferences: 0,
      speed: speed
    };
  }

  afterUpdate(() => {
    if (timelineElement) {
      var threshold = 30;
      var isNearBottom = timelineElement.scrollTop + timelineElement.clientHeight >= timelineElement.scrollHeight - threshold;
      if (isNearBottom || timelineElement.scrollHeight <= timelineElement.clientHeight + 10) {
        timelineElement.scrollTop = timelineElement.scrollHeight;
      }
    }
  });

  onMount(() => {
    loadNominalState();
    connectWebSocket();

    // Fallback to fetch initial state
    fetchInitialState();
    fetchProvider();

    return () => {
      if (ws) ws.close();
      stopStatsPolling();
    };
  });

  var statsInterval = null;

  function startStatsPolling() {
    if (statsInterval) return;
    statsInterval = setInterval(async () => {
      if (isDisconnected) {
        return;
      }
      try {
        var res = await fetch("/scenario/stats");
        if (res.ok) {
          var data = await res.json();
          if (data && data.status !== "not_wired") {
            if (pendingReset || (Date.now() - lastResetTime < 800)) {
              // ignore potentially stale stats right after reset
              return;
            }
            stats = data;
            if (pendingReset) pendingReset = false;
            // Sync isPlaying with stats status
            if (stats.status === "running") {
              isPlaying = true;
            } else if (stats.status === "paused" || stats.status === "complete") {
              isPlaying = false;
            }
            if (stats.speed !== undefined) {
              speed = stats.speed;
            }
          }
        }
      } catch (e) {
        console.error("Stats polling failed:", e);
      }
    }, 1000);
  }

  function stopStatsPolling() {
    if (statsInterval) {
      clearInterval(statsInterval);
      statsInterval = null;
    }
  }

  async function fetchInitialState() {
    try {
      var res = await fetch("/state");
      if (res.ok) {
        state = await res.json();
        isDisconnected = false; // Real server is available
        addLog("SYSTEM", "Loaded snapshot from /state backend.");
        if (state.version > 0 || state.time > 0) {
          isInitialized = true;
        }
        startStatsPolling();
      }
    } catch (e) {
      // Keep disconnected
    }
  }

  async function fetchProvider() {
    try {
      var res = await fetch("/provider");
      if (res.ok) {
        var data = await res.json();
        if (data.provider) {
          currentProvider = data.provider;
        }
      }
    } catch (e) {
      // Keep default
    }
  }

  async function changeProvider(e) {
    var newProvider = e.detail;
    var previousProvider = currentProvider;
    currentProvider = newProvider; // Optimistic update
    switchingProvider = true;
    try {
      var res = await fetch("/provider", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ provider: newProvider })
      });
      if (res.ok) {
        addLog("SYSTEM", `Active provider set to: ${newProvider.toUpperCase()}`);
      } else {
        throw new Error(await res.text() || "unknown error");
      }
    } catch (err) {
      currentProvider = previousProvider; // Revert on failure
      addLog("SYSTEM_ERR", `Failed to switch provider: ${err.message}`);
    } finally {
      switchingProvider = false;
    }
  }

  function connectWebSocket() {
    var loc = window.location;
    var wsUrl = (loc.protocol === "https:" ? "wss://" : "ws://") + loc.host + "/stream";

    try {
      ws = new WebSocket(wsUrl);
      ws.onopen = () => {
        isDisconnected = false;
        pendingReset = false;
        lastResetTime = 0;
        addLog("WS", "Connected to live EventBus stream.");
        startStatsPolling();
      };

      ws.onmessage = (event) => {
        handleIncomingData(JSON.parse(event.data));
      };

      ws.onerror = () => {
        isDisconnected = true;
      };

      ws.onclose = () => {
        isDisconnected = true;
        stopStatsPolling();
        addLog("SYSTEM", "WebSocket disconnected. Reconnecting in 3s...");
        setTimeout(connectWebSocket, 3000);
      };
    } catch (e) {
      isDisconnected = true;
      setTimeout(connectWebSocket, 3000);
    }
  }

  function handleIncomingData(msg) {
    var kind = null;
    var payload = msg;

    // Check if it has the envelope structure {kind: ..., payload: ...}
    if (msg && typeof msg === 'object' && 'kind' in msg && 'payload' in msg) {
      kind = msg.kind;
      payload = msg.payload;
    }

    // Add to Matrix logs
    addLog(kind ? `WS:${kind.toUpperCase()}` : "CEREBRAS", JSON.stringify(payload));

    // P25: WS reset broadcast handling
    if (kind === "reset") {
      if (pendingReset) {
        pendingReset = false;
        addLog("SYSTEM", "All Clear reset received. Feeds and clock cleared.");
        return;
      }
      if (Date.now() - lastResetTime < 1500) {
        addLog("SYSTEM", "All Clear reset received. Feeds and clock cleared.");
        return;
      }
      loadNominalState();
      addLog("SYSTEM", "All Clear reset received. Feeds and clock cleared.");
      return;
    }

    // Route based on explicit kind if present, otherwise fallback to duck-typing
    if (kind === "provider") {
      currentProvider = payload.provider;
      addLog("SYSTEM", `Provider broadcast received: switched to ${currentProvider.toUpperCase()}`);
      return;
    }

    if (kind === "state" || (!kind && payload.sectors && payload.bridges)) {
      state = payload;
      return;
    }

    if (kind === "cop" || (!kind && payload.overallRisk && payload.prioritizedActions)) {
      cop = payload;
      if (!copHistory.some(item => item.summary === payload.summary && item.overallRisk === payload.overallRisk)) {
        copHistory = [payload, ...copHistory];
      }
      if (payload.metrics) {
        metrics.activeCells = payload.metrics.cellCount || 0;
        metrics.tokensPerSec = payload.metrics.aggregateTokensPerSec || 0;
        metrics.latencyMs = payload.metrics.fanOutLatencyMs || 0;
        metrics.tickTokens = payload.metrics.totalTokensOut || 0;
        // Remember this provider's latest decision latency for the comparison card.
        if (payload.metrics.fanOutLatencyMs > 0) {
          lastLatencyByProvider = { ...lastLatencyByProvider, [currentProvider]: payload.metrics.fanOutLatencyMs };
          // Accumulate total reasoning time for this run + snapshot it per provider.
          totalDecisionMs += payload.metrics.fanOutLatencyMs;
          totalDecisionByProvider = { ...totalDecisionByProvider, [currentProvider]: totalDecisionMs };
        }
      } else {
        metrics.activeCells = 0;
      }
      
      // Reset statuses to idle before setting woken cells to done
      for (var k in cellStatuses) {
        cellStatuses[k] = "idle";
      }

      if (payload.cellOutputs) {
        payload.cellOutputs.forEach(out => {
          if (cellStatuses[out.agent] !== undefined) {
            cellStatuses[out.agent] = "done";
          }
          if (cellHistory[out.agent] !== undefined) {
            if (!cellHistory[out.agent].some(item => item.stateVersion === out.stateVersion && item.summary === out.summary)) {
              cellHistory[out.agent] = [out, ...cellHistory[out.agent]];
            }
          }
        });
      }
      return;
    }

    if (kind === "cell_output" || (!kind && payload.agent && payload.recommendations)) {
      if (cellStatuses[payload.agent] !== undefined) {
        cellStatuses[payload.agent] = "done";
      }
      if (cellHistory[payload.agent] !== undefined) {
        if (!cellHistory[payload.agent].some(item => item.stateVersion === payload.stateVersion && item.summary === payload.summary)) {
          cellHistory[payload.agent] = [payload, ...cellHistory[payload.agent]];
        }
      }
      return;
    }

    if (kind === "event" || (!kind && payload.id && payload.type)) {
      timelineEvents = [...timelineEvents, payload];
      if (timelineEvents.length > 500) {
        timelineEvents = timelineEvents.slice(timelineEvents.length - 500);
      }
      if (payload.source !== "ambient") {
        var woken = classifyEvent(payload.type);
        for (var k in cellStatuses) {
          cellStatuses[k] = "idle";
        }
        woken.forEach(cell => {
          cellStatuses[cell] = "analyzing";
        });
        metrics.activeCells = woken.length;
      }
    }
  }

  function addLog(prefix, content) {
    var finalPrefix = prefix === "CEREBRAS" ? currentProvider.toUpperCase() : prefix;
    matrixLogs = [...matrixLogs, { prefix: finalPrefix, content }];
    if (matrixLogs.length > 500) {
      matrixLogs = matrixLogs.slice(matrixLogs.length - 500);
    }
  }

  var selectedScenario = "cerebro-cascade";

  async function initializeEOC() {
    try {
      var res = await fetch("/scenario/reset", { method: "POST" });
      if (res.ok) {
        overlayFading = true;
        addLog("SYSTEM", "Initializing Emergency Operation Center for Cerebro Earthquake Cascade...");
        setTimeout(() => {
          isInitialized = true;
          overlayFading = false;
          loadNominalState();
          addLog("SYSTEM", "Emergency Operation Center initialized.");
        }, 300);
      } else {
        addLog("SYSTEM_ERR", "Failed to initialize EOC simulation.");
      }
    } catch (e) {
      addLog("SYSTEM_ERR", "Network error during EOC initialization.");
    }
  }

  function handlePlay(event) {
    var play = event.detail;
    isPlaying = play;
  }

  function handleStep() {
    isPlaying = false;
  }

  function handleReset() {
    isPlaying = false;
    speed = 1.0;
    lastResetTime = Date.now();
    pendingReset = true;
    loadNominalState();
  }

  function handleLoad(event) {
    var name = event.detail;
    isPlaying = false;
    speed = 1.0;
    loadNominalState();
    addLog("SYSTEM", `Loading scenario: ${name}`);
  }

  function handleSpeed(event) {
    var s = event.detail;
    speed = s;
  }

  function classifyEvent(type) {
    switch (type) {
      case "MainshockOccurred":
        return ["Intelligence", "Infrastructure", "Medical", "Population", "Communications"];
      case "AftershockOccurred":
        return ["Intelligence", "Infrastructure", "Population"];
      case "DamStressElevated":
      case "DamBreached":
      case "LeveeOvertopping":
      case "LeveeBreached":
      case "FloodExtentUpdated":
        return ["Intelligence", "Infrastructure", "Population"];
      case "BridgeDamaged":
      case "BridgeClosed":
      case "BridgeCollapsed":
      case "RoadBlocked":
      case "TunnelClosed":
        return ["Infrastructure", "Population"];
      case "PowerFailure":
      case "PowerRestored":
        return ["Infrastructure", "Communications"];
      case "HospitalStrained":
      case "HospitalCritical":
      case "HospitalOverCapacity":
      case "MedicalEmergency":
        return ["Medical", "Population"];
      case "ShelterStrained":
      case "ShelterExceeded":
        return ["Population"];
      default:
        return [];
    }
  }

  function handleUploading() {
    for (var k in cellStatuses) {
      cellStatuses[k] = "analyzing";
    }
    metrics.activeCells = 5;
    metrics.tokensPerSec = 0;
    metrics.latencyMs = 0;
    metrics.tickTokens = 0;
  }

  function handlePerceptionEvents(e) {
    var evCount = e.detail ? e.detail.length : 0;
    addLog("PERCEPTION", `Tactical image ingested. Vision interpreted ${evCount} emergency events.`);
  }

  function handlePerceptionError(e) {
    addLog("PERCEPTION_ERR", `Ingest failed: ${e.detail}`);
    for (var k in cellStatuses) {
      cellStatuses[k] = "idle";
    }
    metrics.activeCells = 0;
  }
</script>

<div class="dashboard-container">
  <!-- Top HUD panel -->
  <HUD {state} {metrics} {isDisconnected} {isInitialized} {stats} {currentProvider} {switchingProvider} on:changeProvider={changeProvider} />

  <!-- Left Sidebar: Controller and Timeline -->
  <div class="controls-area">
    <PlaybackControl
      {state}
      activeEvent={timelineEvents.length > 0 ? timelineEvents[timelineEvents.length - 1] : null}
      {stats}
      {isDisconnected}
      {isInitialized}
      {isPlaying}
      {speed}
      latencyMs={metrics.latencyMs}
      {currentProvider}
      latencyByProvider={lastLatencyByProvider}
      {totalDecisionMs}
      totalByProvider={totalDecisionByProvider}
      on:play={handlePlay}
      on:step={handleStep}
      on:reset={handleReset}
      on:load={handleLoad}
      on:speed={handleSpeed}
    />
    
    <PerceptionUpload on:uploading={handleUploading} on:events={handlePerceptionEvents} on:error={handlePerceptionError} />
    
    <!-- Timeline Event log -->
    <div class="control-panel" style="flex: 1; display: flex; flex-direction: column;">
      <div class="panel-title">System Event log</div>
      <div class="timeline-list" bind:this={timelineElement}>
        {#each timelineEvents as ev}
          <div class="timeline-item" class:rejected={ev.type === 'event_rejected'}>
            <div class="timeline-item-meta">
              <span class="timeline-time">+{ev.timestamp}s</span>
              <span class="timeline-type">{ev.type}</span>
            </div>
            <div class="timeline-desc">
              Source: {ev.source} (Confidence: {(ev.confidence * 100).toFixed(0)}%)
            </div>
          </div>
        {/each}
      </div>
    </div>
  </div>

  <!-- Center Main Area: Commander Cell & Map split vertically (top), Specialists grid (bottom) -->
  <div class="main-area">
    <div class="top-row-split">
      <!-- Commander panel -->
      <div class="commander-panel" style="display: flex; flex-direction: column; height: 100%;">
        <div class="commander-header">
          <span class="commander-title">Commander Cell (COP)</span>
          {#if cop.overallRisk && cop.overallRisk !== 'Low'}
            <span class="cop-risk">{cop.overallRisk}</span>
          {/if}
        </div>
        
        <!-- Current Summary -->
        <div class="cop-summary" style="margin-bottom: 8px; border-bottom: 1px dashed rgba(139, 92, 246, 0.2); padding-bottom: 8px; max-height: 80px; overflow-y: auto;">
          {cop.summary || "EOC system nominal. Standing by for telemetry."}
        </div>

        <!-- Prior COPs History -->
        <div style="font-size: 0.75rem; font-weight: 700; color: #a78bfa; margin-top: 4px; margin-bottom: 4px;">COP History</div>
        <div class="cop-history-list">
          {#each copHistory.slice(1) as item, idx}
            <div class="cop-history-item">
              <div style="display: flex; justify-content: space-between; font-size: 0.75rem; font-weight: 700; color: #a78bfa;">
                <span>#{copHistory.length - 1 - idx} COP</span>
                {#if item.overallRisk && item.overallRisk !== 'Low'}
                  <span style="color: var(--color-critical); font-weight: 700;">{item.overallRisk}</span>
                {/if}
              </div>
              <div class="cop-summary" style="margin-top: 4px; font-size: 0.75rem; color: var(--text-secondary);">
                {item.summary}
              </div>
              {#if item.prioritizedActions && item.prioritizedActions.length > 0}
                <div class="cop-actions" style="margin-top: 4px;">
                  {#each item.prioritizedActions as action}
                    <div class="cop-action-item" style="font-size: 0.7rem;">
                      <span style="font-weight: 700; color: #d8b4fe;">#{action.priority}</span>
                      <span style="color: var(--text-primary);">{action.action}</span>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          {/each}
          {#if copHistory.length <= 1}
            <div style="font-size: 0.75rem; color: var(--text-muted); font-style: italic; text-align: center; margin-top: 10px;">
              No prior COP history.
            </div>
          {/if}
        </div>
      </div>

      <!-- SV Map -->
      <Map {state} activeEvent={timelineEvents.length > 0 ? timelineEvents[timelineEvents.length - 1] : null} />
    </div>

    <!-- Specialists Grid -->
    <div class="specialists-panel">
      {#each Object.keys(cellStatuses) as name}
        <CellPanel kind={name} history={cellHistory[name]} status={cellStatuses[name]} />
      {/each}
    </div>
  </div>

  <!-- Right Sidebar: JSON Matrix Stream -->
  <div class="right-sidebar">
    <!-- Matrix Stream Feed -->
    <MatrixFeed logs={matrixLogs} {currentProvider} />
  </div>
</div>

{#if !isInitialized}
  <div class="overlay-container" class:fade-out={overlayFading}>
    <div class="overlay-modal">
      <div class="overlay-title">Initialize Command Center</div>
      <div class="overlay-desc">
        Select a disaster scenario to initialize the real-time AI emergency response system.
      </div>
      <div class="overlay-select-wrapper">
        <select bind:value={selectedScenario} class="overlay-select">
          <option value="cerebro-cascade">Cerebro Earthquake Cascade (M6.8)</option>
        </select>
      </div>
      <button class="overlay-btn" on:click={initializeEOC} disabled={isDisconnected}>
        {#if isDisconnected}
          Connecting to EOC Backend...
        {:else}
          Initialize EOC Simulation
        {/if}
      </button>
    </div>
  </div>
{/if}

<style>
  .overlay-container {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(4, 5, 9, 0.85);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 9999;
    opacity: 1;
    transition: opacity 300ms ease;
  }
  .overlay-container.fade-out {
    opacity: 0;
    pointer-events: none;
  }
  .overlay-modal {
    background: rgba(13, 17, 30, 0.95);
    border: 1px solid var(--panel-border-active);
    border-radius: 12px;
    padding: 30px;
    max-width: 450px;
    width: 90%;
    text-align: center;
    box-shadow: 0 0 30px rgba(0, 242, 254, 0.2);
    animation: fadeIn 0.3s ease;
  }
  .overlay-title {
    font-size: 1.4rem;
    font-weight: 700;
    margin-bottom: 12px;
    background: linear-gradient(90deg, #00f2fe, #4facfe);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }
  .overlay-desc {
    font-size: 0.875rem;
    color: var(--text-secondary);
    line-height: 1.5;
    margin-bottom: 20px;
  }
  .overlay-select-wrapper {
    margin-bottom: 20px;
  }
  .overlay-select {
    width: 100%;
    background: rgba(0, 0, 0, 0.3);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: var(--text-primary);
    border-radius: 6px;
    padding: 10px;
    font-size: 0.875rem;
    outline: none;
    cursor: pointer;
  }
  .overlay-select:hover {
    border-color: var(--panel-border-active);
  }
  .overlay-btn {
    width: 100%;
    background: linear-gradient(90deg, #00f2fe, #4facfe);
    border: none;
    border-radius: 6px;
    color: #07090e;
    font-weight: 700;
    padding: 12px;
    cursor: pointer;
    font-size: 0.875rem;
    transition: all 0.2s ease;
  }
  .overlay-btn:hover:not(:disabled) {
    box-shadow: 0 0 15px rgba(0, 242, 254, 0.5);
    transform: translateY(-1px);
  }
  .overlay-btn:disabled {
    background: rgba(255, 255, 255, 0.1);
    color: var(--text-muted);
    cursor: not-allowed;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: scale(0.95); }
    to { opacity: 1; transform: scale(1); }
  }
</style>
