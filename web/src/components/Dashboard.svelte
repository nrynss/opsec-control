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
  var demoMode = true; // Default to demo mode if WS fails or offline
  var ws = null;
  var demoTimer = null;
  var demoStep = 0;

  // P25 Playback and Telemetry State
  var isPlaying = false;
  var speed = 1.0;
  var stats = {
    status: "running",
    currentTime: 0,
    elapsedTime: 0,
    wallElapsed: 0,
    eventsReplayed: 0,
    tokensIn: 0,
    tokensOut: 0,
    inferences: 0,
    speed: 1.0
  };
  var demoWallStart = null;
  var demoAccumulatedWallTime = 0;
  var lastResetTime = 0;

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
    timelineEvents = [];
    matrixLogs = [{ prefix: "SYSTEM", content: "Cerebro command center ready. Listening on /stream." }];

    // P25: Reset stats to nominal faked or zero
    stats = {
      status: isPlaying ? "running" : "paused",
      currentTime: 0,
      elapsedTime: 0,
      wallElapsed: 0,
      eventsReplayed: 0,
      tokensIn: 0,
      tokensOut: 0,
      inferences: 0,
      speed: speed
    };
    demoAccumulatedWallTime = 0;
    if (isPlaying && demoMode) {
      demoWallStart = Date.now();
    } else {
      demoWallStart = null;
    }
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
      if (demoTimer) clearInterval(demoTimer);
      stopStatsPolling();
    };
  });

  var statsInterval = null;

  function startStatsPolling() {
    if (statsInterval) return;
    statsInterval = setInterval(async () => {
      if (demoMode || !ws || ws.readyState !== WebSocket.OPEN) {
        return;
      }
      try {
        var res = await fetch("/scenario/stats");
        if (res.ok) {
          var data = await res.json();
          if (data && data.status !== "not_wired") {
            if (Date.now() - lastResetTime < 800) {
              // ignore potentially stale stats right after reset
              return;
            }
            stats = data;
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
        demoMode = false; // Real server is available
        addLog("SYSTEM", "Loaded snapshot from /state backend.");
        startStatsPolling();
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
        startStatsPolling();
      };

      ws.onmessage = (event) => {
        handleIncomingData(JSON.parse(event.data));
      };

      ws.onerror = () => {
        // Silent fail, falls back to demo mode
      };

      ws.onclose = () => {
        stopStatsPolling();
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

    // P25: WS reset broadcast handling
    if (kind === "reset") {
      if (Date.now() - lastResetTime < 1500) {
        // already handled optimistically in handleReset
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

  const DEMO_MAX_TIME = 60;

  function updateDemoStats(updates) {
    for (var k in updates) {
      stats[k] = updates[k];
    }
    stats = stats;
  }

  function startDemoMode() {
    demoMode = true;
    isPlaying = true;
    speed = 1.0;
    loadNominalState();
    resumeDemoMode();
  }

  function handlePlay(event) {
    var play = event.detail;
    isPlaying = play;
    if (demoMode) {
      updateDemoStats({ status: play ? "running" : "paused" });
      if (play) {
        demoWallStart = Date.now();
        resumeDemoMode();
      } else {
        if (demoWallStart) {
          demoAccumulatedWallTime += Date.now() - demoWallStart;
        }
        demoWallStart = null;
        pauseDemoMode();
      }
    }
  }

  function handleStep() {
    isPlaying = false;
    if (demoMode) {
      updateDemoStats({ status: "paused" });
      if (demoWallStart) {
        demoAccumulatedWallTime += Date.now() - demoWallStart;
      }
      demoWallStart = null;
      pauseDemoMode();
      tickDemo();
    }
  }

  function handleReset() {
    if (demoMode) {
      isPlaying = true;
      speed = 1.0;
      loadNominalState();
      resumeDemoMode();
    } else {
      // Optimistic immediate clear for live reset (snappy All Clear UX);
      // backend reset + WS broadcast will confirm/sync
      isPlaying = false;
      speed = 1.0;
      lastResetTime = Date.now();
      loadNominalState();
    }
  }

  function handleLoad(event) {
    var name = event.detail;
    isPlaying = false;
    speed = 1.0;
    if (demoMode) {
      loadNominalState();
      pauseDemoMode();
      addLog("SYSTEM", `Loaded mock scenario: ${name}`);
    } else {
      // Optimistic clear for live scenario load
      loadNominalState();
      addLog("SYSTEM", `Loading scenario: ${name}`);
    }
  }

  function handleSpeed(event) {
    var s = event.detail;
    speed = s;
    if (demoMode) {
      updateDemoStats({ speed: s });
      if (isPlaying) {
        resumeDemoMode();
      }
    }
  }

  function pauseDemoMode() {
    if (demoTimer) {
      clearInterval(demoTimer);
      demoTimer = null;
    }
  }

  function resumeDemoMode() {
    if (demoTimer) clearInterval(demoTimer);
    demoTimer = setInterval(() => {
      tickDemo();
    }, 1000 / speed);
  }

  function tickDemo() {
    state.time += 1;
    state = state;
    
    var wall = demoAccumulatedWallTime;
    if (demoWallStart) {
      wall += (Date.now() - demoWallStart);
    }

    updateDemoStats({
      currentTime: state.time,
      elapsedTime: state.time,
      wallElapsed: wall,
      eventsReplayed: timelineEvents.length
    });

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
    // Loop replay at t=DEMO_MAX_TIME
    else if (state.time >= DEMO_MAX_TIME) {
      state.time = 0;
      loadNominalState();
    }
  }

  function triggerAct1() {
    addLog("SENSOR", "SEISMIC SPIKE DETECTED. MAGNITUDE 6.8.");
    
    var event = { id: "evt-shock", timestamp: 6, source: "seismograph", type: "MainshockOccurred", confidence: 1.0 };
    timelineEvents = [...timelineEvents, event];

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
      if (!demoMode || stats.currentTime === 0) return;
      cellStatuses.Intelligence = "done";
      var outIntel = { agent: "Intelligence", summary: "Aftershock forecast: 82% within 24 hours. Dam telemetry shows elevated stress.", riskLevel: "Medium", confidence: 0.9, stateVersion: state.version, recommendations: ["Continuous dam telemetry monitoring"], evidence: ["Substation offline indicators"] };
      cellHistory.Intelligence = [outIntel, ...cellHistory.Intelligence];
      addLog("CEREBRAS", JSON.stringify(outIntel));

      cellStatuses.Infrastructure = "done";
      var outInfra = { agent: "Infrastructure", summary: "Vora and Iron bridges closed due to displacement. Highgate grid offline.", riskLevel: "High", confidence: 0.95, stateVersion: state.version, recommendations: ["Initiate structural scans on Vora Bridge", "Establish detours via South Span"], evidence: ["Bridge sensor displacement indicators"] };
      cellHistory.Infrastructure = [outInfra, ...cellHistory.Infrastructure];
      addLog("CEREBRAS", JSON.stringify(outInfra));

      cellStatuses.Medical = "done";
      var outMed = { agent: "Medical", summary: "Central General hospital on backup generators. Bed occupancy 85%.", riskLevel: "Medium", confidence: 0.88, stateVersion: state.version, recommendations: ["Establish ER overflow zone"], evidence: ["Power grid drops"] };
      cellHistory.Medical = [outMed, ...cellHistory.Medical];
      addLog("CEREBRAS", JSON.stringify(outMed));

      cellStatuses.Population = "done";
      var outPop = { agent: "Population", summary: "No casualties reported. Minor evacuation traffic on highway.", riskLevel: "Low", confidence: 0.92, stateVersion: state.version, recommendations: ["Monitor evacuation flows"], evidence: ["Highway traffic cams"] };
      cellHistory.Population = [outPop, ...cellHistory.Population];
      addLog("CEREBRAS", JSON.stringify(outPop));

      cellStatuses.Communications = "done";
      var outComm = { agent: "Communications", summary: "Highgate cell towers disabled. Mesh network mode active.", riskLevel: "Medium", confidence: 0.91, stateVersion: state.version, recommendations: ["Broadcasting localized alerts via backup channel"], evidence: ["Cell tower telemetry dropouts"] };
      cellHistory.Communications = [outComm, ...cellHistory.Communications];
      addLog("CEREBRAS", JSON.stringify(outComm));

      // P25: Update mock stats counters for cells reasoning
      updateDemoStats({
        inferences: stats.inferences + 5,
        tokensIn: stats.tokensIn + 1200,
        tokensOut: stats.tokensOut + 950,
        eventsReplayed: timelineEvents.length
      });

      // Commander synthesizes COP
      setTimeout(() => {
        if (!demoMode || stats.currentTime === 0) return;
        var latestOutputs = [outIntel, outInfra, outMed, outPop, outComm];
        cop = {
          summary: "Cerebro earthquake cascade. Two bridges closed, Highgate heavily damaged, Central General hospital at critical capacity.",
          overallRisk: "High",
          prioritizedActions: [
            { priority: 1, action: "Inspect Vora and Iron bridges for structural integrity", owner: "Infrastructure" },
            { priority: 2, action: "Deploy backup generator fuel to Central General", owner: "Medical" },
            { priority: 3, action: "Enable localized emergency broadcasts in Highgate", owner: "Communications" }
          ],
          cellOutputs: latestOutputs
        };
        copHistory = [cop, ...copHistory];
        metrics.activeCells = 0;
        addLog("CEREBRAS", JSON.stringify(cop));
        addLog("ORCH", "Commander synthesized COP successfully in 520ms.");

        // P25: Update mock stats counters for Commander reasoning
        updateDemoStats({
          inferences: stats.inferences + 1,
          tokensIn: stats.tokensIn + 2000,
          tokensOut: stats.tokensOut + 400,
          eventsReplayed: timelineEvents.length
        });
      }, 200);

    }, 450);
  }

  function triggerAct2() {
    addLog("SENSOR", "AFTERSHOCK DETECTED. M5.9. Fire reported in Ironworks.");

    var event = { id: "evt-after", timestamp: 18, source: "seismograph", type: "AftershockOccurred", confidence: 1.0 };
    var eventFire = { id: "evt-fire", timestamp: 18, source: "citizen", type: "FireIgnited", confidence: 0.9 };
    timelineEvents = [...timelineEvents, event, eventFire];

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
      if (!demoMode || stats.currentTime === 0) return;
      cellStatuses.Infrastructure = "done";
      var outInfra = { agent: "Infrastructure", summary: "South Span restricted. Greenfield evacuation lanes compromised.", riskLevel: "Critical", confidence: 0.94, stateVersion: state.version, recommendations: ["Prioritize South Span structural inspection"], evidence: ["Bridge load sensors spike"] };
      cellHistory.Infrastructure = [outInfra, ...cellHistory.Infrastructure];
      addLog("CEREBRAS", JSON.stringify(outInfra));

      cellStatuses.Population = "done";
      var outPop = { agent: "Population", summary: "Evacuation route blocked. Greenfield shelter at 90% capacity.", riskLevel: "High", confidence: 0.96, stateVersion: state.version, recommendations: ["Redirect traffic to Greenfield secondary gymnasium"], evidence: ["Traffic queue at South Span"] };
      cellHistory.Population = [outPop, ...cellHistory.Population];
      addLog("CEREBRAS", JSON.stringify(outPop));

      // P25: Update mock stats counters for cells reasoning
      updateDemoStats({
        inferences: stats.inferences + 2,
        tokensIn: stats.tokensIn + 600,
        tokensOut: stats.tokensOut + 500,
        eventsReplayed: timelineEvents.length
      });

      setTimeout(() => {
        if (!demoMode || stats.currentTime === 0) return;
        cop = {
          summary: "Cascading aftershock triggers multiple utility failures. Greenfield evacuation routes severely compromised. Fire active in Ironworks.",
          overallRisk: "Critical",
          prioritizedActions: [
            { priority: 1, action: "Clear alternative evacuation routes via Southport bypass", owner: "Population" },
            { priority: 2, action: "Deploy firefighting units to Ironworks sector", owner: "Infrastructure" },
            { priority: 3, action: "Inspect South Span bridge foundation", owner: "Infrastructure" }
          ],
          cellOutputs: [outInfra, outPop]
        };
        copHistory = [cop, ...copHistory];
        metrics.activeCells = 0;
        addLog("CEREBRAS", JSON.stringify(cop));
        addLog("ORCH", "Commander synthesized COP successfully in 410ms.");

        // P25: Update mock stats counters for Commander reasoning
        updateDemoStats({
          inferences: stats.inferences + 1,
          tokensIn: stats.tokensIn + 1100,
          tokensOut: stats.tokensOut + 300,
          eventsReplayed: timelineEvents.length
        });
      }, 150);
    }, 350);
  }

  function triggerAct3() {
    addLog("SENSOR", "LEVEE BREACH IN SOUTHPORT. FLOOD VECTOR ACTIVE.");

    var event = { id: "evt-breach", timestamp: 34, source: "drone-feed", type: "LeveeBreached", confidence: 0.98 };
    timelineEvents = [...timelineEvents, event];

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
      if (!demoMode || stats.currentTime === 0) return;
      cellStatuses.Intelligence = "done";
      var outIntel = { agent: "Intelligence", summary: "Flood vector modeling indicates Southport water depth of 1.5m, rising 10cm/hr.", riskLevel: "High", confidence: 0.93, stateVersion: state.version, recommendations: ["Issue flood warning for Southport lowest elevations"], evidence: ["Water depth telemetry"] };
      cellHistory.Intelligence = [outIntel, ...cellHistory.Intelligence];
      addLog("CEREBRAS", JSON.stringify(outIntel));

      cellStatuses.Population = "done";
      var outPop = { agent: "Population", summary: "Evacuation of Southport sector required. 4,500 citizens stranded.", riskLevel: "Critical", confidence: 0.97, stateVersion: state.version, recommendations: ["Deploy rescue boats to Southport sector"], evidence: ["Drone frames showing water levels"] };
      cellHistory.Population = [outPop, ...cellHistory.Population];
      addLog("CEREBRAS", JSON.stringify(outPop));

      // P25: Update mock stats counters for cells reasoning
      updateDemoStats({
        inferences: stats.inferences + 2,
        tokensIn: stats.tokensIn + 700,
        tokensOut: stats.tokensOut + 550,
        eventsReplayed: timelineEvents.length
      });

      setTimeout(() => {
        if (!demoMode || stats.currentTime === 0) return;
        cop = {
          summary: "Levee breach in Southport leads to severe flooding. 4,500 citizens stranded. Evacuations in progress.",
          overallRisk: "Critical",
          prioritizedActions: [
            { priority: 1, action: "Deploy rescue craft and high-clearance vehicles to Southport", owner: "Population" },
            { priority: 2, action: "Construct sandbag barriers along secondary canal", owner: "Infrastructure" },
            { priority: 3, action: "Set up triage and dry evacuation zone at Greenfield", owner: "Medical" }
          ],
          cellOutputs: [outIntel, outPop]
        };
        copHistory = [cop, ...copHistory];
        metrics.activeCells = 0;
        addLog("CEREBRAS", JSON.stringify(cop));
        addLog("ORCH", "Commander synthesized COP successfully in 430ms.");

        // P25: Update mock stats counters for Commander reasoning
        updateDemoStats({
          inferences: stats.inferences + 1,
          tokensIn: stats.tokensIn + 1250,
          tokensOut: stats.tokensOut + 320,
          eventsReplayed: timelineEvents.length
        });
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
    <PlaybackControl
      {state}
      activeEvent={timelineEvents.length > 0 ? timelineEvents[timelineEvents.length - 1] : null}
      {stats}
      {demoMode}
      {isPlaying}
      {speed}
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
