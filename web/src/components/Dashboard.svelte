<!-- e:\opsec-control\web\src\components\Dashboard.svelte -->
<script>
  import { onMount } from 'svelte';
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

  // Commander COP
  var cop = {
    summary: "EOC system nominal. Standing by for telemetry.",
    overallRisk: "Low",
    prioritizedActions: [],
    cellOutputs: []
  };

  // Specialist Cell statuses and data
  var cellStatuses = {
    "Intelligence": "idle",
    "Infrastructure": "idle",
    "Medical": "idle",
    "Population": "idle",
    "Communications": "idle"
  };

  var cellData = {
    "Intelligence": null,
    "Infrastructure": null,
    "Medical": null,
    "Population": null,
    "Communications": null
  };

  var timelineEvents = [];
  var matrixLogs = [];

  var currentProvider = "cerebras";
  var switchingProvider = false;
  var demoMode = true; // Default to demo mode if WS fails or offline
  var ws = null;
  var demoTimer = null;
  var demoStep = 0;

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

    for (var k in cellStatuses) {
      cellStatuses[k] = "idle";
      cellData[k] = null;
    }
    metrics = { activeCells: 0, tokensPerSec: 0, latencyMs: 0, tickTokens: 0 };
    timelineEvents = [];
    matrixLogs = [{ prefix: "SYSTEM", content: "Cerebro command center ready. Listening on /stream." }];
  }

  onMount(() => {
    loadNominalState();
    connectWebSocket();

    // Fallback to fetch initial state
    fetchInitialState();
    fetchProvider();

    return () => {
      if (ws) ws.close();
      if (demoTimer) clearInterval(demoTimer);
    };
  });

  async function fetchInitialState() {
    try {
      var res = await fetch("/state");
      if (res.ok) {
        state = await res.json();
        demoMode = false; // Real server is available
        addLog("SYSTEM", "Loaded snapshot from /state backend.");
      }
    } catch (e) {
      // Keep demo mode active
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
    
    // In dev mode pointing to localhost:8080
    if (loc.port === "5173" || loc.port === "4321") {
      wsUrl = "ws://localhost:8080/stream";
    }

    try {
      ws = new WebSocket(wsUrl);
      ws.onopen = () => {
        demoMode = false;
        if (demoTimer) {
          clearInterval(demoTimer);
          demoTimer = null;
        }
        addLog("WS", "Connected to live EventBus stream.");
      };

      ws.onmessage = (event) => {
        handleIncomingData(JSON.parse(event.data));
      };

      ws.onerror = () => {
        // Silent fail, falls back to demo mode
      };

      ws.onclose = () => {
        if (!demoMode) {
          addLog("SYSTEM", "WebSocket disconnected. Starting local simulation.");
          startDemoMode();
        }
      };
    } catch (e) {
      startDemoMode();
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
      if (payload.metrics) {
        metrics.activeCells = payload.metrics.cellCount || 0;
        metrics.tokensPerSec = payload.metrics.aggregateTokensPerSec || 0;
        metrics.latencyMs = payload.metrics.fanOutLatencyMs || 0;
        metrics.tickTokens = payload.metrics.totalTokensOut || 0;
      } else {
        metrics.activeCells = 0;
      }
      
      // Reset statuses to idle before setting woken cells to done
      for (var k in cellStatuses) {
        cellStatuses[k] = "idle";
      }

      if (payload.cellOutputs) {
        payload.cellOutputs.forEach(out => {
          cellStatuses[out.agent] = "done";
          cellData[out.agent] = out;
        });
      }
      return;
    }

    if (kind === "cell_output" || (!kind && payload.agent && payload.recommendations)) {
      cellStatuses[payload.agent] = "done";
      cellData[payload.agent] = payload;
      return;
    }

    if (kind === "event" || (!kind && payload.id && payload.type)) {
      timelineEvents = [payload, ...timelineEvents];
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
    if (matrixLogs.length > 100) {
      matrixLogs = matrixLogs.slice(matrixLogs.length - 100);
    }
  }

  // --- High-Fidelity Demo Simulation Mode ---
  function startDemoMode() {
    demoMode = true;
    demoStep = 0;
    loadNominalState();

    if (demoTimer) clearInterval(demoTimer);
    
    // Periodically advance the clock. Events trigger at specific times.
    demoTimer = setInterval(() => {
      state.time += 1;
      
      // Act 1: shock at t=6
      if (state.time === 6) {
        triggerAct1();
      }
      // Act 2: aftershock + fire at t=18
      else if (state.time === 18) {
        triggerAct2();
      }
      // Act 3: flood breach at t=34
      else if (state.time === 34) {
        triggerAct3();
      }
      // Loop replay at t=60
      else if (state.time >= 60) {
        state.time = 0;
        loadNominalState();
      }
    }, 1000);
  }

  function triggerAct1() {
    addLog("SENSOR", "SEISMIC SPIKE DETECTED. MAGNITUDE 6.8.");
    
    var event = { id: "evt-shock", timestamp: 6, source: "seismograph", type: "MainshockOccurred", confidence: 1.0 };
    timelineEvents = [event, ...timelineEvents];

    // Map shock adjustments
    state.version += 1;
    state.sectors.highgate.power = "off";
    state.sectors.central.power = "off";
    state.bridges.vora.status = "closed";
    state.bridges.iron.status = "closed";

    // Simulate parallel cell activation
    metrics.activeCells = 5;
    metrics.tokensPerSec = 1500;
    metrics.latencyMs = 480;
    metrics.tickTokens = 1200;

    for (var k in cellStatuses) {
      cellStatuses[k] = "analyzing";
    }

    addLog("ORCH", "ANOMALY: Seismic alert. Invoking specialists in parallel.");

    // Cells finish simultaneously
    setTimeout(() => {
      cellStatuses.Intelligence = "done";
      cellData.Intelligence = { agent: "Intelligence", summary: "Aftershock forecast: 82% within 24 hours. Dam telemetry shows elevated stress.", riskLevel: "Medium", confidence: 0.9, stateVersion: state.version, recommendations: ["Continuous dam telemetry monitoring"], evidence: ["Substation offline indicators"] };
      addLog("CEREBRAS", JSON.stringify(cellData.Intelligence));

      cellStatuses.Infrastructure = "done";
      cellData.Infrastructure = { agent: "Infrastructure", summary: "Vora and Iron bridges closed due to displacement. Highgate grid offline.", riskLevel: "High", confidence: 0.95, stateVersion: state.version, recommendations: ["Initiate structural scans on Vora Bridge", "Establish detours via South Span"], evidence: ["Bridge sensor displacement indicators"] };
      addLog("CEREBRAS", JSON.stringify(cellData.Infrastructure));

      cellStatuses.Medical = "done";
      cellData.Medical = { agent: "Medical", summary: "Central General hospital on backup generators. Bed occupancy 85%.", riskLevel: "Medium", confidence: 0.88, stateVersion: state.version, recommendations: ["Establish ER overflow zone"], evidence: ["Power grid drops"] };
      addLog("CEREBRAS", JSON.stringify(cellData.Medical));

      cellStatuses.Population = "done";
      cellData.Population = { agent: "Population", summary: "No casualties reported. Minor evacuation traffic on highway.", riskLevel: "Low", confidence: 0.92, stateVersion: state.version, recommendations: ["Monitor evacuation flows"], evidence: ["Highway traffic cams"] };
      addLog("CEREBRAS", JSON.stringify(cellData.Population));

      cellStatuses.Communications = "done";
      cellData.Communications = { agent: "Communications", summary: "Highgate cell towers disabled. Mesh network mode active.", riskLevel: "Medium", confidence: 0.91, stateVersion: state.version, recommendations: ["Broadcasting localized alerts via backup channel"], evidence: ["Cell tower telemetry dropouts"] };
      addLog("CEREBRAS", JSON.stringify(cellData.Communications));

      // Commander synthesizes COP
      setTimeout(() => {
        cop = {
          summary: "Cerebro earthquake cascade. Two bridges closed, Highgate heavily damaged, Central General hospital at critical capacity.",
          overallRisk: "High",
          prioritizedActions: [
            { priority: 1, action: "Inspect Vora and Iron bridges for structural integrity", owner: "Infrastructure" },
            { priority: 2, action: "Deploy backup generator fuel to Central General", owner: "Medical" },
            { priority: 3, action: "Enable localized emergency broadcasts in Highgate", owner: "Communications" }
          ],
          cellOutputs: Object.values(cellData)
        };
        metrics.activeCells = 0;
        addLog("CEREBRAS", JSON.stringify(cop));
        addLog("ORCH", "Commander synthesized COP successfully in 520ms.");
      }, 200);

    }, 450);
  }

  function triggerAct2() {
    addLog("SENSOR", "AFTERSHOCK DETECTED. M5.9. Fire reported in Ironworks.");

    var event = { id: "evt-after", timestamp: 18, source: "seismograph", type: "AftershockOccurred", confidence: 1.0 };
    var eventFire = { id: "evt-fire", timestamp: 18, source: "citizen", type: "FireIgnited", confidence: 0.9 };
    timelineEvents = [eventFire, event, ...timelineEvents];

    state.version += 1;
    state.bridges["south-span"].status = "closed"; // South Span closed
    state.fireZones["ironworks-fire"] = { id: "ironworks-fire", sector: "ironworks", status: "spreading" };
    state.dam.status = "stressed";

    metrics.activeCells = 2; // Infrastructure + Population
    metrics.tokensPerSec = 1500;
    metrics.latencyMs = 380;
    metrics.tickTokens = 600;

    cellStatuses.Infrastructure = "analyzing";
    cellStatuses.Population = "analyzing";

    addLog("ORCH", "ANOMALY: South Span closed, fire active. Invoking specialists.");

    setTimeout(() => {
      cellStatuses.Infrastructure = "done";
      cellData.Infrastructure = { agent: "Infrastructure", summary: "South Span restricted. Greenfield evacuation lanes compromised.", riskLevel: "Critical", confidence: 0.94, stateVersion: state.version, recommendations: ["Prioritize South Span structural inspection"], evidence: ["Bridge load sensors spike"] };
      addLog("CEREBRAS", JSON.stringify(cellData.Infrastructure));

      cellStatuses.Population = "done";
      cellData.Population = { agent: "Population", summary: "Evacuation route blocked. Greenfield shelter at 90% capacity.", riskLevel: "High", confidence: 0.96, stateVersion: state.version, recommendations: ["Redirect traffic to Greenfield secondary gymnasium"], evidence: ["Traffic queue at South Span"] };
      addLog("CEREBRAS", JSON.stringify(cellData.Population));

      setTimeout(() => {
        cop = {
          summary: "Cascading aftershock triggers multiple utility failures. Greenfield evacuation routes severely compromised. Fire active in Ironworks.",
          overallRisk: "Critical",
          prioritizedActions: [
            { priority: 1, action: "Clear alternative evacuation routes via Southport bypass", owner: "Population" },
            { priority: 2, action: "Deploy firefighting units to Ironworks sector", owner: "Infrastructure" },
            { priority: 3, action: "Inspect South Span bridge foundation", owner: "Infrastructure" }
          ],
          cellOutputs: Object.values(cellData)
        };
        metrics.activeCells = 0;
        addLog("CEREBRAS", JSON.stringify(cop));
        addLog("ORCH", "Commander synthesized COP successfully in 410ms.");
      }, 150);
    }, 350);
  }

  function triggerAct3() {
    addLog("SENSOR", "LEVEE BREACH IN SOUTHPORT. FLOOD VECTOR ACTIVE.");

    var event = { id: "evt-breach", timestamp: 34, source: "drone-feed", type: "LeveeBreached", confidence: 0.98 };
    timelineEvents = [event, ...timelineEvents];

    state.version += 1;
    state.levee.status = "breached";
    state.flood.polygons = [
      { sector: "southport", depthM: 1.5, points: [{ x: 300, y: 350 }, { x: 500, y: 350 }, { x: 500, y: 440 }, { x: 300, y: 440 }] }
    ];

    metrics.activeCells = 2; // Intelligence + Population
    metrics.tokensPerSec = 1500;
    metrics.latencyMs = 410;
    metrics.tickTokens = 700;

    cellStatuses.Intelligence = "analyzing";
    cellStatuses.Population = "analyzing";

    addLog("ORCH", "ANOMALY: Levee failure. Invoking specialists.");

    setTimeout(() => {
      cellStatuses.Intelligence = "done";
      cellData.Intelligence = { agent: "Intelligence", summary: "Flood vector modeling indicates Southport water depth of 1.5m, rising 10cm/hr.", riskLevel: "High", confidence: 0.93, stateVersion: state.version, recommendations: ["Issue flood warning for Southport lowest elevations"], evidence: ["Water depth telemetry"] };
      addLog("CEREBRAS", JSON.stringify(cellData.Intelligence));

      cellStatuses.Population = "done";
      cellData.Population = { agent: "Population", summary: "Evacuation of Southport sector required. 4,500 citizens stranded.", riskLevel: "Critical", confidence: 0.97, stateVersion: state.version, recommendations: ["Deploy rescue boats to Southport sector"], evidence: ["Drone frames showing water levels"] };
      addLog("CEREBRAS", JSON.stringify(cellData.Population));

      setTimeout(() => {
        cop = {
          summary: "Levee breach in Southport leads to severe flooding. 4,500 citizens stranded. Evacuations in progress.",
          overallRisk: "Critical",
          prioritizedActions: [
            { priority: 1, action: "Deploy rescue craft and high-clearance vehicles to Southport", owner: "Population" },
            { priority: 2, action: "Construct sandbag barriers along secondary canal", owner: "Infrastructure" },
            { priority: 3, action: "Set up triage and dry evacuation zone at Greenfield", owner: "Medical" }
          ],
          cellOutputs: Object.values(cellData)
        };
        metrics.activeCells = 0;
        addLog("CEREBRAS", JSON.stringify(cop));
        addLog("ORCH", "Commander synthesized COP successfully in 430ms.");
      }, 150);
    }, 380);
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
  <HUD {state} {metrics} {demoMode} {currentProvider} {switchingProvider} on:changeProvider={changeProvider} />

  <!-- Left Sidebar: Controller and Timeline -->
  <div class="controls-area">
    <PlaybackControl {state} activeEvent={timelineEvents[0]} />
    
    <PerceptionUpload on:uploading={handleUploading} on:events={handlePerceptionEvents} on:error={handlePerceptionError} />
    
    <!-- Timeline Event log -->
    <div class="control-panel" style="flex: 1; display: flex; flex-direction: column;">
      <div class="panel-title">System Event log</div>
      <div class="timeline-list">
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

  <!-- Center Main Area: Tactical Map and Specialists grid -->
  <div class="main-area">
    <!-- SV Map -->
    <Map {state} activeEvent={timelineEvents[0]} />

    <!-- Specialists Grid -->
    <div class="specialists-panel">
      {#each Object.keys(cellStatuses) as name}
        <CellPanel kind={name} data={cellData[name]} status={cellStatuses[name]} />
      {/each}
    </div>
  </div>

  <!-- Right Sidebar: Commander Synthesis & JSON Matrix Stream -->
  <div class="right-sidebar">
    <!-- Commander panel -->
    <div class="commander-panel">
      <div class="commander-header">
        <span class="commander-title">Commander Cell (COP)</span>
        {#if cop.overallRisk && cop.overallRisk !== 'Low'}
          <span class="cop-risk">{cop.overallRisk}</span>
        {/if}
      </div>
      
      <div class="cop-summary">
        {cop.summary}
      </div>

      <div style="font-size: 0.8rem; font-weight: 700; color: #a78bfa; margin-top: 6px; border-top: 1px solid rgba(139, 92, 246, 0.15); padding-top: 6px;">Prioritized Actions</div>
      <div class="cop-actions">
        {#if cop.prioritizedActions.length === 0}
          <div style="font-size: 0.75rem; color: var(--text-muted); font-style: italic;">No emergency recommendations generated.</div>
        {:else}
          {#each cop.prioritizedActions as action}
            <div class="cop-action-item">
              <span style="font-weight: 700; color: #d8b4fe;">#{action.priority}</span>
              <span style="color: var(--text-primary);">{action.action}</span>
            </div>
          {/each}
        {/if}
      </div>
    </div>

    <!-- Matrix Stream Feed -->
    <MatrixFeed logs={matrixLogs} {currentProvider} />
  </div>
</div>
