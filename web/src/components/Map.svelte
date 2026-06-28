<!-- e:\opsec-control\web\src\components\Map.svelte -->
<script>
  export var state = {};
  export var activeEvent = null;

  // Static geographical coordinates for sectors in a 800x480 coordinate space
  // designed to look like a clean grid-aligned dashboard map.
  var sectorGeom = {
    "westbank": {
      name: "Westbank",
      points: "40,40 320,40 300,240 40,240",
      textX: 180,
      textY: 140
    },
    "greenfield": {
      name: "Greenfield",
      points: "40,240 300,240 260,440 40,440",
      textX: 150,
      textY: 340
    },
    "harborside": {
      name: "Harborside",
      points: "40,440 260,440 240,460 40,460",
      textX: 140,
      textY: 450
    },
    "central": {
      name: "Central",
      points: "360,40 600,40 600,240 340,240",
      textX: 470,
      textY: 140
    },
    "highgate": {
      name: "Highgate",
      points: "600,40 760,40 760,240 600,240",
      textX: 680,
      textY: 140
    },
    "southport": {
      name: "Southport",
      points: "340,240 600,240 560,440 300,440",
      textX: 450,
      textY: 340
    },
    "ironworks": {
      name: "Ironworks",
      points: "600,240 760,240 760,440 560,440",
      textX: 670,
      textY: 340
    }
  };

  // Helper to determine risk class for a sector based on its power or general state
  function getSectorClass(sectorId) {
    if (!state.sectors || !state.sectors[sectorId]) return "nominal";
    var sec = state.sectors[sectorId];
    if (sec.power === "off" || sec.water === "down" || sec.comms === "down") {
      return "critical";
    }
    if (sec.power === "partial" || sec.water === "degraded" || sec.comms === "degraded") {
      return "strained";
    }
    return "nominal";
  }

  // Helper to determine bridge status class
  function getBridgeClass(bridgeId) {
    if (!state.bridges || !state.bridges[bridgeId]) return "open";
    return state.bridges[bridgeId].status || "open";
  }

  // Get active fire for a sector
  function getSectorFire(sectorId) {
    if (!state.fireZones) return null;
    for (var k in state.fireZones) {
      var f = state.fireZones[k];
      if (f.sector === sectorId && (f.status === "ignited" || f.status === "spreading")) {
        return f;
      }
    }
    return null;
  }

  // Get flood depth for a sector
  function getSectorFlood(sectorId) {
    if (!state.flood || !state.flood.polygons) return 0;
    var maxDepth = 0;
    for (var i = 0; i < state.flood.polygons.length; i++) {
      var p = state.flood.polygons[i];
      if (p.sector === sectorId && p.depthM > maxDepth) {
        maxDepth = p.depthM;
      }
    }
    return maxDepth;
  }
</script>

<div class="map-panel">
  <div class="panel-title" style="position: absolute; top: 16px; left: 16px; z-index: 10;">Interactive Cerebro Map</div>
  
  <svg viewBox="0 0 800 480" class="cerebro-svg">
    <!-- Grid lines for tactical aesthetic -->
    <defs>
      <pattern id="grid" width="40" height="40" patternUnits="userSpaceOnUse">
        <path d="M 40 0 L 0 0 0 40" fill="none" stroke="rgba(255, 255, 255, 0.02)" stroke-width="1"/>
      </pattern>
    </defs>
    <rect width="800" height="480" fill="url(#grid)" />

    <!-- River Cerebro flowing vertically in the middle -->
    <path d="M 340,0 L 320,100 L 300,240 L 280,380 L 260,480 L 300,480 L 320,380 L 340,240 L 360,100 L 380,0 Z" class="map-river" />

    <!-- Sectors -->
    {#each Object.entries(sectorGeom) as [id, geom]}
      <g>
        <!-- Sector Polygon -->
        <polygon 
          points={geom.points} 
          class="map-sector {getSectorClass(id)}"
        />

        <!-- Flood Overlay (if flooded) -->
        {#if getSectorFlood(id) > 0}
          <polygon 
            points={geom.points} 
            fill="rgba(0, 191, 255, {Math.min(0.1 + getSectorFlood(id) * 0.1, 0.5)})"
            stroke="rgba(0, 191, 255, 0.5)"
            stroke-width="1"
            pointer-events="none"
          />
        {/if}

        <!-- Sector Name -->
        <text x={geom.textX} y={geom.textY} class="map-text">
          {geom.name}
        </text>

        <!-- Utility Indicators (Power, Comms, Water) -->
        {#if state.sectors && state.sectors[id]}
          <g transform="translate({geom.textX - 25}, {geom.textY + 15})">
            <!-- Power Indicator -->
            <rect width="10" height="6" rx="1" fill={state.sectors[id].power === "on" ? "var(--color-nominal)" : (state.sectors[id].power === "partial" ? "var(--color-medium)" : "var(--color-critical)")} opacity="0.8" />
            <!-- Comms Indicator -->
            <rect x="15" width="10" height="6" rx="1" fill={state.sectors[id].comms === "up" ? "var(--color-nominal)" : (state.sectors[id].comms === "degraded" ? "var(--color-medium)" : "var(--color-critical)")} opacity="0.8" />
            <!-- Water Indicator -->
            <rect x="30" width="10" height="6" rx="1" fill={state.sectors[id].water === "up" ? "var(--color-nominal)" : (state.sectors[id].water === "degraded" ? "var(--color-medium)" : "var(--color-critical)")} opacity="0.8" />
          </g>
        {/if}

        <!-- Fire Indicators -->
        {#if getSectorFire(id)}
          <g transform="translate({geom.textX - 10}, {geom.textY - 30})">
            <!-- Fire animated icon -->
            <path d="M12 2c0 0-3.5 3.5-3.5 7.5S10 17 12 17s3.5-3.5 3.5-7.5S12 2 12 2z" class="map-fire" />
            <path d="M12 7c0 0-2 2-2 4.5S11 14 12 14s2-2 2-4.5S12 7 12 7z" fill="#ffcc00" />
          </g>
        {/if}
      </g>
    {/each}

    <!-- Dynamic Flood Polygons from State -->
    {#if state.flood && state.flood.polygons}
      {#each state.flood.polygons as poly}
        {#if poly.points && poly.points.length > 0}
          <polygon 
            points={poly.points.map(p => `${p.x},${p.y}`).join(' ')} 
            class="map-flood-poly" 
            title="Flood depth: {poly.depthM}m"
          />
        {/if}
      {/each}
    {/if}

    <!-- Levee (Southport side protection) -->
    <line x1="290" y1="300" x2="275" y2="400" class="map-levee" class:stressed={state.levee && state.levee.status === "overtopping"} class:breached={state.levee && state.levee.status === "breached"} />
    <text x="260" y="350" font-size="8" fill="#94a3b8" transform="rotate(-75 260 350)">LEVEE</text>

    <!-- Dam (Upstream river block) -->
    <line x1="330" y1="30" x2="370" y2="30" class="map-dam" class:stressed={state.dam && state.dam.status === "stressed"} class:releasing={state.dam && state.dam.status === "releasing"} class:breached={state.dam && state.dam.status === "breached"} />
    <text x="350" y="22" font-size="8" fill="#94a3b8">DAM</text>

    <!-- Bridges spanning the river -->
    <!-- Vora Bridge (Westbank <-> Central) -->
    <line x1="305" y1="130" x2="355" y2="130" class="map-bridge {getBridgeClass('vora')}" title="Vora Bridge" />
    <text x="330" y="120" font-size="8" fill="#94a3b8" text-anchor="middle">Vora</text>

    <!-- Iron Bridge (Westbank <-> Central/Southport boundary) -->
    <line x1="298" y1="240" x2="342" y2="240" class="map-bridge {getBridgeClass('iron')}" title="Iron Bridge" />
    <text x="320" y="255" font-size="8" fill="#94a3b8" text-anchor="middle">Iron</text>

    <!-- South Span (Greenfield <-> Southport) -->
    <line x1="278" y1="390" x2="318" y2="390" class="map-bridge {getBridgeClass('south-span') || getBridgeClass('south_span')}" title="South Span" />
    <text x="298" y="382" font-size="8" fill="#94a3b8" text-anchor="middle">South Span</text>
  </svg>
</div>
